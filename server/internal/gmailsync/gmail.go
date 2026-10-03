package gmailsync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const gmailAPI = "https://gmail.googleapis.com/gmail/v1/users/me"
var defaultGmailHTTPClient = &http.Client{Timeout: 90 * time.Second}

type Client struct {
	HTTP                                                    *http.Client
	APIBase, TokenURL, ClientID, ClientSecret, RefreshToken string
	RetryDelay                                              time.Duration // backoff base for throttled requests; 0 means 5s
	mu                                                      sync.Mutex
	accessToken                                             string
	tokenExpiry                                             time.Time
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Until(c.tokenExpiry) > time.Minute {
		return c.accessToken, nil
	}
	endpoint := c.TokenURL
	if endpoint == "" {
		endpoint = "https://oauth2.googleapis.com/token"
	}
	form := url.Values{"client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "refresh_token": {c.RefreshToken}, "grant_type": {"refresh_token"}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return "", e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h := c.HTTP
	if h == nil {
		h = defaultGmailHTTPClient
	}
	res, e := h.Do(req)
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return "", fmt.Errorf("gmail token refresh: %s: %s", res.Status, b)
	}
	var t tokenResponse
	if e = json.NewDecoder(res.Body).Decode(&t); e != nil {
		return "", e
	}
	if t.AccessToken == "" {
		return "", fmt.Errorf("gmail token refresh: missing access token")
	}
	c.accessToken = t.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(t.ExpiresIn) * time.Second)
	return t.AccessToken, nil
}
func (c *Client) request(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	delay := c.RetryDelay
	if delay <= 0 {
		delay = 5 * time.Second
	}
	for attempt := 0; ; attempt++ {
		retry, err := c.try(ctx, method, path, payload, out)
		if !retry || attempt >= 5 {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay < time.Minute {
			delay *= 2
		}
	}
}

// try performs one API call. It reports retry when Gmail asked us to slow
// down (per-minute quota, transient overload) so request can back off
// instead of failing the whole sync pass over a throttled burst.
func (c *Client) try(ctx context.Context, method, path string, payload []byte, out any) (retry bool, err error) {
	tok, e := c.token(ctx)
	if e != nil {
		return false, e
	}
	base := c.APIBase
	if base == "" {
		base = gmailAPI
	}
	var r io.Reader
	if payload != nil {
		r = bytes.NewReader(payload)
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, r)
	if e != nil {
		return false, e
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	h := c.HTTP
	if h == nil {
		h = defaultGmailHTTPClient
	}
	res, e := h.Do(req)
	if e != nil {
		// Transient transport failures (connection resets, read timeouts)
		// are worth retrying; a done context is not.
		return ctx.Err() == nil, e
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		err := fmt.Errorf("gmail API %s %s: %s: %s", method, path, res.Status, b)
		switch {
		case res.StatusCode >= 500:
			// Gmail serves transient 5xx (observed: 500 Internal error on a
			// message raw fetch that succeeds seconds later); retry them.
			return true, err
		case res.StatusCode == http.StatusTooManyRequests:
			return true, err
		case res.StatusCode == http.StatusForbidden && bytes.Contains(b, []byte("rateLimitExceeded")):
			return true, err
		}
		return false, err
	}
	if out != nil {
		return false, json.NewDecoder(res.Body).Decode(out)
	}
	return false, nil
}

type ThreadRef struct {
	ID        string `json:"id"`
	HistoryID string `json:"historyId"`
}
type ThreadList struct {
	Threads       []ThreadRef `json:"threads"`
	NextPageToken string      `json:"nextPageToken"`
}
type Message struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	HistoryID    string   `json:"historyId"`
	InternalDate string   `json:"internalDate"`
	LabelIDs     []string `json:"labelIds"`
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}
type Thread struct {
	ID        string    `json:"id"`
	HistoryID string    `json:"historyId"`
	Messages  []Message `json:"messages"`
}
type HistoryMessage struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}
type HistoryPage struct {
	History []struct {
		ID            string `json:"id"`
		MessagesAdded []struct {
			Message HistoryMessage `json:"message"`
		} `json:"messagesAdded"`
		LabelsAdded []struct {
			Message  HistoryMessage `json:"message"`
			LabelIDs []string       `json:"labelIds"`
		} `json:"labelsAdded"`
		LabelsRemoved []struct {
			Message  HistoryMessage `json:"message"`
			LabelIDs []string       `json:"labelIds"`
		} `json:"labelsRemoved"`
	} `json:"history"`
	HistoryID     string `json:"historyId"`
	NextPageToken string `json:"nextPageToken"`
}

func (c *Client) ListInbox(ctx context.Context, page string) (ThreadList, error) {
	var x ThreadList
	p := "/threads?labelIds=INBOX&maxResults=100"
	if page != "" {
		p += "&pageToken=" + url.QueryEscape(page)
	}
	e := c.request(ctx, "GET", p, nil, &x)
	return x, e
}
func (c *Client) GetThread(ctx context.Context, id string) (Thread, error) {
	var x Thread
	e := c.request(ctx, "GET", "/threads/"+url.PathEscape(id)+"?format=full", nil, &x)
	return x, e
}

func (c *Client) GetThreadMetadata(ctx context.Context, id string) (Thread, error) {
	var x Thread
	p := "/threads/" + url.PathEscape(id) + "?format=metadata&metadataHeaders=From"
	err := c.request(ctx, "GET", p, nil, &x)
	return x, err
}

func (c *Client) GetMessageRaw(ctx context.Context, id string) ([]byte, error) {
	var message struct {
		Raw string `json:"raw"`
	}
	if err := c.request(ctx, "GET", "/messages/"+url.PathEscape(id)+"?format=raw", nil, &message); err != nil {
		return nil, err
	}
	// Gmail pads its base64url raw payloads with '='; RawURLEncoding
	// rejects padding, so normalize before decoding.
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(message.Raw, "="))
	if err != nil {
		return nil, fmt.Errorf("gmail raw message %s: %w", id, err)
	}
	return raw, nil
}
func (c *Client) History(ctx context.Context, id, page string) (HistoryPage, error) {
	var x HistoryPage
	p := "/history?startHistoryId=" + url.QueryEscape(id) + "&historyTypes=messageAdded&historyTypes=labelAdded&historyTypes=labelRemoved"
	if page != "" {
		p += "&pageToken=" + url.QueryEscape(page)
	}
	e := c.request(ctx, "GET", p, nil, &x)
	return x, e
}
func (c *Client) Modify(ctx context.Context, thread string, add, remove []string) error {
	return c.request(ctx, "POST", "/threads/"+url.PathEscape(thread)+"/modify", map[string][]string{"addLabelIds": add, "removeLabelIds": remove}, nil)
}
func (c *Client) CreateLabel(ctx context.Context, name string) (string, error) {
	var x struct {
		ID string `json:"id"`
	}
	e := c.request(ctx, "POST", "/labels", map[string]string{"name": name, "labelListVisibility": "labelShow", "messageListVisibility": "show"}, &x)
	return x.ID, e
}
