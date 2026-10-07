package outages

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestEntitiesAreListedAgainOnlyEverySixHours(t *testing.T) {
	f, api := serve(t)
	m := at(t, api, "", recorded)
	refreshed(t, m)
	refreshed(t, m)
	const entities, events = "entities/query?entityType=country", "outages/events?entityType=country"
	if f.count(entities) != 1 || f.count(events) != 2 {
		t.Errorf("asked for entities %d times and outages %d", f.count(entities), f.count(events))
	}
	m.now = func() time.Time { return recorded.Add(relist) }
	refreshed(t, m)
	if f.count(entities) != 2 {
		t.Errorf("asked for entities %d times after %v", f.count(entities), relist)
	}
}

func TestCodesIODADoesntKnowAreANote(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "countries: [nz, zz]\nsignals: [bgp]", recorded)
	sink := sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sink.WaitFor(t, 1)
	if h := m.Health(); h.Err != nil || h.Note != "IODA knows no country ZZ" {
		t.Errorf("health %+v", h)
	}
}

func TestAFailedReadShowsInHealth(t *testing.T) {
	f, api := serve(t)
	f.fail(http.StatusServiceUnavailable)
	m := at(t, api, "", recorded)
	sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sdktest.Eventually(t, func() bool {
		err := m.Health().Err
		return err != nil && strings.Contains(err.Error(), "entities/query: 503 Service Unavailable")
	})
}

func TestRetriesBackOffAndWaitAsLongAsIODAAsks(t *testing.T) {
	busy := &busyError{path: "outages/events", wait: 2 * time.Minute}
	for _, c := range []struct {
		err            error
		backoff, every time.Duration
		want           time.Duration
	}{
		{errors.New("refused"), firstRetry, 5 * time.Minute, firstRetry},
		{errors.New("refused"), 10 * time.Minute, 5 * time.Minute, 5 * time.Minute},
		{busy, firstRetry, 5 * time.Minute, 2 * time.Minute},
		{busy, 4 * time.Minute, 5 * time.Minute, 4 * time.Minute},
	} {
		if got := retryIn(c.err, c.backoff, c.every); got != c.want {
			t.Errorf("retryIn(%v, %v, %v) = %v, want %v", c.err, c.backoff, c.every, got, c.want)
		}
	}
}

func TestEntitiesAreSentBeforeTheSignalsAreRead(t *testing.T) {
	f, api := serve(t)
	f.answer("signals/raw/country/ET,NZ,TN?datasource=bgp", `{"error": "the signal store is down", "data": null}`)
	m := at(t, api, "", recorded)
	sink := sdktest.Run(t, func(ctx context.Context, s *sdktest.Sink) error { return m.Run(ctx, s) })
	sink.WaitFor(t, 1)
	if n := len(sink.Sets()[0].Upserts); n != 3 {
		t.Errorf("snapshot of %d entities", n)
	}
	sdktest.Eventually(t, func() bool {
		err := m.Health().Err
		return err != nil && strings.Contains(err.Error(), "the signal store is down")
	})
}
