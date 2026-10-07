package outages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"wayseer.dev/sdk"
	"wayseer.dev/sdk/sdktest"
)

// configured is a module configured with options.
func configured(t *testing.T, options string) *Module {
	t.Helper()
	cfg, err := sdktest.Config("out", options)
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Configure(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	return m
}

// TestConformance runs the suite as the marketplace does: against testdata/source served as files.
func TestConformance(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	sdktest.Conform(t, sdktest.Case{
		New:      func() sdk.Module { return New() },
		Name:     "out",
		Options:  "api: " + serveSource(t) + "\ncountries: [NZ]\nsignals: [bgp]",
		Failing:  "api: " + closed.URL + "/v2",
		Manifest: "manifest.yaml",
	})
}
