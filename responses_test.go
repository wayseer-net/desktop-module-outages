package outages

import (
	"context"
	"strings"
	"testing"
)

// The requests a module with the default options makes first, by kind.
const (
	askCountries = "entities/query?entityType=country"
	askOutages   = "outages/events?entityType=country"
	askBGP       = "signals/raw/country/ET,NZ,TN?datasource=bgp"
)

func TestBadResponsesAreErrors(t *testing.T) {
	for _, c := range []struct{ ask, body, want string }{
		{askCountries, `{"error": null, "data": {"code": "NZ"}}`, "entities/query: json"},
		{askCountries, `{"error": "IODA is down", "data": null}`, "entities/query: IODA is down"},
		{askOutages, `{"error": null, "data": [{"from": "yesterday"}]}`, "outages/events: json"},
		{askBGP, `{"error": null, "data": [{"values": []}]}`, "signals/raw/country/ET,NZ,TN: json"},
		{askBGP, `{"error": null, "data": [[{"from": 1, "step": 60, "values": "x"}]]}`, "signals/raw/country/ET,NZ,TN: json"},
	} {
		f, api := serve(t)
		f.answer(c.ask, c.body)
		m := at(t, api, "", recorded)
		_, _, err := m.refresh(context.Background())
		if err == nil {
			err = m.readSignals(context.Background())
		}
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s answering %s: %v, want %q", c.ask, c.body, err, c.want)
		}
	}
}

func TestNoCountriesIsAnEmptyWorld(t *testing.T) {
	f, api := serve(t)
	f.answer(askCountries, `{"error": null, "data": []}`)
	cs, note, err := at(t, api, "", recorded).refresh(context.Background())
	if err != nil || len(cs.Upserts) != 0 || note != "" {
		t.Errorf("%+v, %q, %v", cs, note, err)
	}
}

func TestEntitiesThatCantBeShownAreLeftOutWithANote(t *testing.T) {
	f, api := serve(t)
	f.answer("entities/query?entityCode=NZ&entityType=country",
		`{"error": null, "data": [{"code": "NZ", "type": "country", "name": "New Zealand"}, {"code": "", "type": "country"}, {"code": "OC", "type": "continent"}]}`)
	cs, note, err := at(t, api, "countries: [NZ]\nsignals: [bgp]", recorded).refresh(context.Background())
	if err != nil || len(cs.Upserts) != 1 || !strings.Contains(note, `IODA's country "" can't be shown`) {
		t.Errorf("%d entities, %q, %v", len(cs.Upserts), note, err)
	}
}
