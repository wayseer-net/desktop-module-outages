package outages

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorded is when the fixtures were recorded: the end of every window they answer.
var recorded = time.Unix(1791381600, 0)

// routes maps each recorded request, less its window, to its fixture in testdata.
var routes = map[string]string{
	"entities/query?entityType=country":                       "ioda/countries.json",
	"outages/events?entityType=country":                       "ioda/country-events.json",
	"signals/raw/country/ET,NZ,TN?datasource=bgp":             "ioda/country-bgp.json",
	"signals/raw/country/ET,NZ,TN?datasource=ping-slash24":    "ioda/country-ping-slash24.json",
	"signals/raw/country/ET,NZ,TN?datasource=merit-nt":        "ioda/country-merit-nt.json",
	"entities/query?entityType=region&relatedTo=country%2FNZ": "ioda/nz-regions.json",
	"outages/events?entityCode=3046%2C3047&entityType=region": "ioda/region-events.json",
	"signals/raw/region/3046,3047?datasource=bgp":             "ioda/region-bgp.json",
	"entities/query?entityCode=9500&entityType=asn":           "ioda/asn-9500.json",
	"entities/query?entityType=country&relatedTo=asn%2F9500":  "ioda/asn-9500-countries.json",
	"outages/events?entityCode=9500&entityType=asn":           "ioda/asn-events.json",
	"signals/raw/asn/9500?datasource=bgp":                     "ioda/asn-bgp.json",
	"signals/raw/country/NZ?datasource=gtr-norm":              "ioda/nz-gtr-norm.json",
	"entities/query?entityCode=NZ&entityType=country":         "source/v2/entities/query",
	"entities/query?entityCode=NZ%2CZZ&entityType=country":    "source/v2/entities/query",
	"outages/events?entityCode=NZ&entityType=country":         "source/v2/outages/events",
	"signals/raw/country/NZ?datasource=bgp":                   "source/v2/signals/raw/country/NZ",
}

// fakeIODA answers recorded requests from testdata, and counts what it was asked.
type fakeIODA struct {
	t      *testing.T
	mu     sync.Mutex
	asked  map[string]int
	status int // when set, every answer is this status
}

func (f *fakeIODA) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := requestKey(r.URL)
	f.mu.Lock()
	f.asked[key]++
	status := f.status
	f.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		return
	}
	name, ok := routes[key]
	if !ok {
		f.t.Errorf("unrecorded request %s", key)
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join("testdata", name))
}

// requestKey is a request's path under /v2/ and its query, less the window and point count.
func requestKey(u *url.URL) string {
	q := u.Query()
	for _, k := range []string{"from", "until", "maxPoints", "format"} {
		q.Del(k)
	}
	return strings.TrimPrefix(u.Path, "/v2/") + "?" + q.Encode()
}

func (f *fakeIODA) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[key]
}

func (f *fakeIODA) fail(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status = status
}

// serve starts a fake IODA and returns it with its API's URL.
func serve(t *testing.T) (*fakeIODA, string) {
	t.Helper()
	f := &fakeIODA{t: t, asked: map[string]int{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv.URL + "/v2"
}

// serveSource serves testdata/source as plain files, as the marketplace's sandbox does.
func serveSource(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.FileServerFS(os.DirFS("testdata/source")))
	t.Cleanup(srv.Close)
	return srv.URL + "/v2"
}
