package gmailsync

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// batchWindow is how long the first queued mutation waits for company.
	// Concurrent archives of one account arrive within a few milliseconds.
	batchWindow = 20 * time.Millisecond
	// Each threads.modify costs 10 of the user's 250 quota units/second.
	maxBatchCalls = 25
	gmailBatchAPI = "https://www.googleapis.com/batch/gmail/v1"
)

type modifyResult struct {
	retry   bool
	err     error
	history string
}

type modifyCall struct {
	ctx     context.Context
	thread  string
	payload []byte
	done    chan modifyResult
}

// ModifyBatched is Modify for concurrent callers: mutations queued within
// batchWindow (or until maxBatchCalls) travel as ONE Gmail HTTP batch request,
// each call keeping its own success or failure. A lone mutation is sent as a
// plain threads.modify. Quota consequence: a batch does not reduce quota, each
// threads.modify inside it still costs 10 units against Gmail's 250 units/s per
// user, so sustained throughput stays near 25 threads/s and 429/rateLimit
// answers are retried per call with backoff; the batch removes the per-request
// round trips, not the quota.
func (c *Client) ModifyBatched(ctx context.Context, thread string, add, remove []string) error {
	_, err := c.modifyBatchedHistory(ctx, thread, add, remove)
	return err
}
func (c *Client) modifyBatchedHistory(ctx context.Context, thread string, add, remove []string) (string, error) {
	var history string
	payload, err := json.Marshal(map[string][]string{"addLabelIds": add, "removeLabelIds": remove})
	if err != nil {
		return "", err
	}
	err = c.retrying(ctx, func() (bool, error) {
		call := &modifyCall{ctx: ctx, thread: thread, payload: payload, done: make(chan modifyResult, 1)}
		c.enqueue(call)
		select {
		case r := <-call.done:
			history = r.history
			return r.retry, r.err
		case <-ctx.Done():
			return false, ctx.Err()
		}
	})
	var api *APIError
	if errors.As(err, &api) && (api.StatusCode == http.StatusBadRequest || api.StatusCode == http.StatusNotFound) {
		c.forgetLabels() // a deleted or recreated label would otherwise stay cached
	}
	return history, err
}

func (c *Client) enqueue(call *modifyCall) {
	var full []*modifyCall
	c.bmu.Lock()
	c.pending = append(c.pending, call)
	if len(c.pending) >= maxBatchCalls {
		full, c.pending = c.pending, nil
	} else if len(c.pending) == 1 {
		time.AfterFunc(batchWindow, c.flushPending)
	}
	c.bmu.Unlock()
	if full != nil {
		go c.send(full)
	}
}

func (c *Client) flushPending() {
	c.bmu.Lock()
	calls := c.pending
	c.pending = nil
	c.bmu.Unlock()
	if len(calls) > 0 {
		c.send(calls)
	}
}

func (c *Client) batchURL() string {
	if c.BatchURL != "" {
		return c.BatchURL
	}
	if c.APIBase == "" {
		return gmailBatchAPI
	}
	u, err := url.Parse(c.APIBase)
	if err != nil {
		return gmailBatchAPI
	}
	return u.Scheme + "://" + u.Host + "/batch/gmail/v1"
}

func threadModifyPath(thread string) string { return "/threads/" + url.PathEscape(thread) + "/modify" }

func (c *Client) send(calls []*modifyCall) {
	// One sender per account enforces quota across simultaneous batch flushes.
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.quotaUsed+len(calls) > maxBatchCalls {
		if delay := time.Until(c.quotaUntil); delay > 0 {
			time.Sleep(delay)
		}
	}
	if !time.Now().Before(c.quotaUntil) {
		c.quotaUsed = 0
	}
	active := calls[:0]
	for _, call := range calls {
		if err := call.ctx.Err(); err != nil {
			call.done <- modifyResult{err: err}
		} else {
			active = append(active, call)
		}
	}
	if len(active) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	// Warm the token before the final cancellation check: OAuth can take WAN time.
	_, err := c.token(ctx)
	calls = active
	active = calls[:0]
	for _, call := range calls {
		if e := call.ctx.Err(); e != nil {
			call.done <- modifyResult{err: e}
		} else if err != nil {
			call.done <- modifyResult{err: err}
		} else {
			active = append(active, call)
		}
	}
	if len(active) == 0 {
		return
	}
	c.quotaUsed += len(active)
	if len(active) == 1 {
		call := active[0]
		var result Thread
		retry, err := c.try(call.ctx, "POST", threadModifyPath(call.thread), call.payload, &result)
		if errors.Is(err, io.EOF) {
			err = nil
		}
		call.done <- modifyResult{retry, err, result.HistoryID}
	} else {
		results := c.sendBatch(ctx, active)
		for i, call := range active {
			call.done <- results[i]
		}
	}
	// Start the rolling window at completion, conservatively allowing for WAN
	// jitter and provider processing rather than assuming request arrival times.
	c.quotaUntil = time.Now().Add(time.Second)
}

func (c *Client) sendBatch(ctx context.Context, calls []*modifyCall) []modifyResult {
	results := make([]modifyResult, len(calls))
	fail := func(retry bool, err error) []modifyResult {
		for i := range results {
			results[i] = modifyResult{retry, err, ""}
		}
		return results
	}
	tok, err := c.token(ctx)
	if err != nil {
		return fail(false, err)
	}
	apiPath := "/gmail/v1/users/me"
	if c.APIBase != "" {
		if u, e := url.Parse(c.APIBase); e == nil {
			apiPath = strings.TrimRight(u.Path, "/")
		}
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for i, call := range calls {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", "application/http")
		h.Set("Content-ID", fmt.Sprintf("<item%d>", i))
		part, _ := mw.CreatePart(h)
		fmt.Fprintf(part, "POST %s%s\r\nContent-Type: application/json\r\n\r\n%s", apiPath, threadModifyPath(call.thread), call.payload)
	}
	mw.Close()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.batchURL(), &body)
	if err != nil {
		return fail(false, err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())
	h := c.HTTP
	if h == nil {
		h = defaultGmailHTTPClient
	}
	res, err := h.Do(req)
	if err != nil {
		return fail(ctx.Err() == nil, err)
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		if res.StatusCode == http.StatusUnauthorized {
			c.invalidateToken(tok)
		}
		return fail(retryableStatus(res.StatusCode, b), &APIError{StatusCode: res.StatusCode, Method: "POST", Path: "/batch", Status: res.Status, Body: string(b)})
	}
	_, params, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil || params["boundary"] == "" {
		return fail(false, fmt.Errorf("gmail batch: bad response content type %q", res.Header.Get("Content-Type")))
	}
	seen := make([]bool, len(calls))
	mr := multipart.NewReader(res.Body, params["boundary"])
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			for i := range results {
				if !seen[i] {
					results[i] = modifyResult{true, fmt.Errorf("gmail batch: %w", err), ""}
					seen[i] = true
				}
			}
			return results
		}
		id := strings.TrimSuffix(strings.TrimPrefix(part.Header.Get("Content-ID"), "<"), ">")
		idx, convErr := strconv.Atoi(strings.TrimPrefix(strings.TrimPrefix(id, "response-"), "item"))
		if convErr != nil || idx < 0 || idx >= len(calls) || seen[idx] {
			continue
		}
		seen[idx] = true
		inner, err := http.ReadResponse(bufio.NewReader(part), nil)
		if err != nil {
			results[idx] = modifyResult{true, fmt.Errorf("gmail batch part: %w", err), ""}
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(inner.Body, 10<<20))
		inner.Body.Close()
		if inner.StatusCode/100 == 2 {
			var result Thread
			_ = json.Unmarshal(b, &result)
			results[idx].history = result.HistoryID
			continue
		}
		if inner.StatusCode == http.StatusUnauthorized {
			c.invalidateToken(tok)
		}
		results[idx] = modifyResult{retryableStatus(inner.StatusCode, b), &APIError{StatusCode: inner.StatusCode, Method: "POST", Path: threadModifyPath(calls[idx].thread), Status: inner.Status, Body: string(b)}, ""}
	}
	for i := range results {
		if !seen[i] {
			results[i] = modifyResult{false, fmt.Errorf("gmail batch: no response for call %d", i), ""}
		}
	}
	return results
}

// resolveLabel returns the Gmail label ID for a name, creating the label when
// absent. Results are cached on the client so a burst of label operations
// costs one lookup, not one per operation.
func (c *Client) resolveLabel(ctx context.Context, name string) (string, error) {
	c.lmu.Lock()
	defer c.lmu.Unlock()
	if id, ok := c.labels[name]; ok {
		return id, nil
	}
	id, err := c.labelID(ctx, name)
	if err != nil {
		return "", err
	}
	if id == "" {
		if id, err = c.CreateLabel(ctx, name); err != nil {
			return "", err
		}
	}
	if c.labels == nil {
		c.labels = map[string]string{}
	}
	c.labels[name] = id
	return id, nil
}

func (c *Client) forgetLabels() {
	c.lmu.Lock()
	c.labels = nil
	c.lmu.Unlock()
}
