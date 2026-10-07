package outages

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// answer serves one fixed response and records the request.
func answer(t *testing.T, status int, header http.Header, body string) (*client, *http.Request) {
	t.Helper()
	got := new(http.Request)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*got = *r.Clone(context.Background())
		for k, v := range header {
			w.Header()[k] = v
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return newClient(srv.URL+"/v2", time.Second), got
}

func TestGetDecodesTheEnvelopesData(t *testing.T) {
	c, req := answer(t, 200, nil, `{"type":"entities.lookup","error":null,"data":[{"code":"NZ"}],"copyright":"(c)"}`)
	var data []struct{ Code string }
	if err := c.get(context.Background(), "entities/query", url.Values{"entityType": {"country"}}, &data); err != nil {
		t.Fatal(err)
	}
	if len(data) != 1 || data[0].Code != "NZ" {
		t.Errorf("data %+v", data)
	}
	if req.URL.Path != "/v2/entities/query" || req.URL.RawQuery != "entityType=country" {
		t.Errorf("asked for %s", req.URL)
	}
	if ua := req.Header.Get("User-Agent"); !strings.Contains(ua, "desktop-module-outages") {
		t.Errorf("User-Agent %q", ua)
	}
}

func TestGetFailsOnIODAsErrors(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   string
	}{
		{200, `{"error":"cannot find corresponding metadata entity 0","data":null}`, "signals/raw/country/ZZ: cannot find"},
		{200, `not json`, "signals/raw/country/ZZ"},
		{500, `oops`, "500 Internal Server Error"},
	} {
		cl, _ := answer(t, c.status, nil, c.body)
		var data any
		err := cl.get(context.Background(), "signals/raw/country/ZZ", nil, &data)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%d %s: %v, want %q", c.status, c.body, err, c.want)
		}
	}
}

func TestTooManyRequestsSaysHowLongToWait(t *testing.T) {
	c, _ := answer(t, 429, http.Header{"Retry-After": {"120"}}, "")
	var data any
	err := c.get(context.Background(), "outages/events", nil, &data)
	if wait, ok := retryAfter(err); !ok || wait != 2*time.Minute {
		t.Errorf("%v: wait %v, %v", err, wait, ok)
	}
	if _, ok := retryAfter(errors.New("other")); ok {
		t.Error("another error asks to wait")
	}
}

func TestIODAsHostIsCappedAtTLS12(t *testing.T) {
	if got := maxTLS(newClient(defaultAPI, time.Second)); got != tls.VersionTLS12 {
		t.Errorf("IODA's client allows TLS %x, want at most 1.2", got)
	}
}

func TestOtherHostsKeepTheDefaultTLS(t *testing.T) {
	if got := maxTLS(newClient("https://ioda.example.org/v2", time.Second)); got != 0 {
		t.Errorf("a mirror's client caps TLS at %x, want Go's default", got)
	}
}

// maxTLS is the highest TLS version c's transport allows; 0 is Go's default.
func maxTLS(c *client) uint16 {
	tr, ok := c.http.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil {
		return 0
	}
	return tr.TLSClientConfig.MaxVersion
}
