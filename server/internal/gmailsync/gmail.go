package gmailsync

import (
	"bytes"
	"context"
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

type Client struct {
	HTTP                                                    *http.Client
	APIBase, TokenURL, ClientID, ClientSecret, RefreshToken string
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
		h = http.DefaultClient
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
	tok, e := c.token(ctx)
	if e != nil {
		return e
	}
	base := c.APIBase
	if base == "" {
		base = gmailAPI
	}
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimRight(base, "/")+path, r)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	h := c.HTTP
	if h == nil {
		h = http.DefaultClient
	}
	res, e := h.Do(req)
	if e != nil {
		return e
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("gmail API %s %s: %s: %s", method, path, res.Status, b)
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
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
