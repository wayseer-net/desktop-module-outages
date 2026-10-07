package outages

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// maxBody is the largest response read; a week of signals for every country fits.
const maxBody = 64 << 20

// userAgent names the module to IODA, so its operators can reach whoever runs it.
const userAgent = "wayseer-outages (+https://github.com/wayseer-net/desktop-module-outages)"

// client reads IODA's API v2, one request at a time.
type client struct {
	base string
	http *http.Client
}

func newClient(base string, timeout time.Duration) *client {
	return &client{base: strings.TrimSuffix(base, "/"), http: &http.Client{Timeout: timeout}}
}

// envelope is what every IODA response wraps its data in.
type envelope struct {
	Error *string         `json:"error"`
	Data  json.RawMessage `json:"data"`
}

// busyError is IODA asking the module to wait before it asks again.
type busyError struct {
	path string
	wait time.Duration
}

func (e *busyError) Error() string {
	return fmt.Sprintf("%s: IODA is busy (429 Too Many Requests); waiting %v", e.path, e.wait)
}

// retryAfter is how long err says to wait, if it is IODA asking.
func retryAfter(err error) (time.Duration, bool) {
	var b *busyError
	if errors.As(err, &b) {
		return b.wait, true
	}
	return 0, false
}

// get reads path with query q and decodes the envelope's data into data; errors name the path.
func (c *client) get(ctx context.Context, path string, q url.Values, data any) error {
	body, err := c.fetch(ctx, path, q)
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if env.Error != nil {
		return fmt.Errorf("%s: %s", path, *env.Error)
	}
	if err := json.Unmarshal(env.Data, data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// fetch returns the body of a 200 response to GET path?q.
func (c *client) fetch(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.base + "/" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &busyError{path: path, wait: waitFor(resp.Header.Get("Retry-After"))}
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%s: %s", path, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
}

// waitFor reads a Retry-After of seconds; IODA states no limit, so a missing one means a minute.
func waitFor(header string) time.Duration {
	if s, err := strconv.Atoi(header); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	return time.Minute
}
