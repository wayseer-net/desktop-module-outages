package outages

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"wayseer.dev/sdk"
)

// queried asks m for one metric of the given entities, or of all its entities, over the last 3h.
func queried(t *testing.T, m *Module, metric string, ents ...sdk.EntityRef) []sdk.Series {
	t.Helper()
	got, err := m.QuerySeries(context.Background(), sdk.SeriesQuery{
		Entities: ents, Metrics: []string{metric},
		Window: sdk.TimeWindow{From: recorded.Add(-3 * time.Hour), To: recorded.Add(time.Second)},
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestEachSignalIsASeries(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "", recorded)
	refreshed(t, m)
	got := queried(t, m, "ioda.bgp", "out/outages%2Fcountry/ET")
	if len(got) != 1 || got[0].Unit != sdk.UnitCount {
		t.Fatalf("series %+v", got)
	}
	want := []sdk.Point{
		{T: time.Unix(1791374400, 0).UnixNano(), V: 4648},
		{T: time.Unix(1791376200, 0).UnixNano(), V: 4562.666666666667},
		{T: time.Unix(1791378000, 0).UnixNano(), V: 4605.333333333333},
		{T: time.Unix(1791379800, 0).UnixNano(), V: 4648},
	}
	if !slices.Equal(got[0].Points, want) {
		t.Errorf("points %+v", got[0].Points)
	}
}

func TestEveryEntityWithTheMetricHasASeries(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "countries: [NZ]\nregions: true\nasns: [9500]\nsignals: [bgp, gtr-norm]", recorded)
	refreshed(t, m)
	if got := queried(t, m, "ioda.bgp"); len(got) != 4 {
		t.Errorf("bgp series %+v", got)
	}
	gtr := queried(t, m, "ioda.gtr-norm")
	if len(gtr) != 1 || gtr[0].Ref.Entity != "out/outages%2Fcountry/NZ" || len(gtr[0].Points) != 2 {
		t.Errorf("gtr-norm series %+v", gtr)
	}
	if got := queried(t, m, "ioda.merit-nt"); len(got) != 0 {
		t.Errorf("a signal not asked for has series %+v", got)
	}
}

func TestPointsAreNumbersAtTheirStep(t *testing.T) {
	var s signal
	if err := json.Unmarshal([]byte(`{"from": 100, "step": 60, "values": [1, null, 2.5, [{"agg_values": {}}], "x", 4]}`), &s); err != nil {
		t.Fatal(err)
	}
	got := s.points()
	want := []sdk.Point{{T: 100e9, V: 1}, {T: 220e9, V: 2.5}, {T: 400e9, V: 4}}
	if !slices.Equal(got, want) {
		t.Errorf("points %+v", got)
	}
}

func TestSeriesKeepTheNewestPointsWithinTheLookback(t *testing.T) {
	var s series
	s.merge([]sdk.Point{{T: 10, V: 1}, {T: 20, V: 2}, {T: 30, V: 3}}, 0)
	s.merge([]sdk.Point{{T: 30, V: 33}, {T: 40, V: 4}}, 20)
	want := []sdk.Point{{T: 20, V: 2}, {T: 30, V: 33}, {T: 40, V: 4}}
	if !slices.Equal(s.points, want) {
		t.Errorf("points %+v", s.points)
	}
	if got := s.in(sdk.TimeWindow{From: time.Unix(0, 25), To: time.Unix(0, 41)}); !slices.Equal(got, want[1:]) {
		t.Errorf("in window %+v", got)
	}
}

func TestSignalsAreAskedForSinceTheLastReadLessTheirLag(t *testing.T) {
	lookback := 24 * time.Hour
	if got := signalsSince(time.Time{}, recorded, lookback); !got.Equal(recorded.Add(-lookback)) {
		t.Errorf("first read from %v", got)
	}
	last := recorded.Add(-5 * time.Minute)
	if got := signalsSince(last, recorded, lookback); !got.Equal(last.Add(-signalLag)) {
		t.Errorf("next read from %v", got)
	}
}
