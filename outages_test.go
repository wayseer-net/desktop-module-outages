package outages

import (
	"context"
	"strings"
	"testing"
	"time"

	"wayseer.dev/sdk"
)

// at is a module reading the fake IODA, with its clock at t.
func at(t *testing.T, api, options string, when time.Time) *Module {
	t.Helper()
	m := configured(t, "api: "+api+"\n"+options)
	m.now = func() time.Time { return when }
	return m
}

func TestEachOutageIsAnEventOnce(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "", recorded)
	first := refreshed(t, m)
	var got []string
	for _, e := range first.Events {
		got = append(got, e.Severity.String()+" "+e.Kind+" "+e.Message)
	}
	want := []string{
		"warn outage Ethiopia: bgp outage for 15m0s",
		"warn outage Tunisia: bgp outage for 40m0s",
		"critical outage Tunisia: ping-slash24 outage began",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("events\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if again := refreshed(t, m); len(again.Events) != 0 {
		t.Errorf("events sent again: %+v", again.Events)
	}
}

func TestAnOutageThatEndsIsAnotherEvent(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "", recorded)
	refreshed(t, m)
	m.now = func() time.Time { return recorded.Add(time.Hour) }
	evs := refreshed(t, m).Events
	if len(evs) != 1 {
		t.Fatalf("events %+v", evs)
	}
	e := evs[0]
	if e.Kind != "outage-ended" || e.Severity != sdk.SevInfo || !e.At.Equal(recorded) || e.Entity.Native() != "TN" {
		t.Errorf("event %+v", e)
	}
	if e.Message != "Tunisia: ping-slash24 outage ended after 182h40m0s" {
		t.Errorf("message %q", e.Message)
	}
}

func TestAnOutageEventSaysWhenAndHowBad(t *testing.T) {
	_, api := serve(t)
	e := refreshed(t, at(t, api, "", recorded)).Events[0]
	start, end := time.Unix(1791377400, 0), time.Unix(1791378300, 0)
	if !e.At.Equal(start) || e.Entity != "out/outages%2Fcountry/ET" || e.Source != "out" {
		t.Errorf("event %+v", e)
	}
	for k, want := range map[string]sdk.Value{
		"datasource": sdk.String("bgp"), "method": sdk.String("median"), "score": sdk.Number(82.61617900172118),
		"start": sdk.Time(start), "end": sdk.Time(end),
		"ioda": sdk.String("https://ioda.inetintel.cc.gatech.edu/country/ET?from=1791377400&until=1791378300"),
	} {
		if got := e.Fields[k]; got.String() != want.String() {
			t.Errorf("field %s is %v, want %v", k, got, want)
		}
	}
}

func TestPastOutagesCanBeQueried(t *testing.T) {
	_, api := serve(t)
	m := at(t, api, "", recorded)
	refreshed(t, m)
	got, err := m.QueryEvents(context.Background(), sdk.EventQuery{
		Window: sdk.TimeWindow{From: recorded.Add(-2 * time.Hour), To: recorded}, MinSeverity: sdk.SevWarn,
	})
	if err != nil || len(got) != 1 || got[0].Entity.Native() != "ET" {
		t.Errorf("events %+v, %v", got, err)
	}
}

// refreshed reads the fake IODA once, as Run does.
func refreshed(t *testing.T, m *Module) *sdk.ChangeSet {
	t.Helper()
	cs, _, err := m.refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return cs
}
