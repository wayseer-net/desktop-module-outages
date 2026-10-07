package outages

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"wayseer.dev/sdk"
	"wayseer.dev/sdk/sdktest"
)

// discovered is what a module with options finds in the fake IODA at the time it was recorded.
func discovered(t *testing.T, options string) *sdk.ChangeSet {
	t.Helper()
	_, api := serve(t)
	m := configured(t, "api: "+api+"\n"+options)
	m.now = func() time.Time { return recorded }
	cs, err := m.Discover(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func TestEveryCountryByDefault(t *testing.T) {
	sdktest.Golden(t, "testdata/countries.txt", render(discovered(t, "")))
}

func TestCountriesRegionsAndNetworksAreLinked(t *testing.T) {
	cs := discovered(t, "countries: [NZ]\nregions: true\nasns: [9500]\nsignals: [bgp, gtr-norm]")
	sdktest.Golden(t, "testdata/nz.txt", render(cs))
}

// render lists a change set's entities, with their places, and its edges, one per line.
func render(cs *sdk.ChangeSet) string {
	var b strings.Builder
	for _, e := range cs.Upserts {
		keys := make([]string, 0, len(e.Attrs))
		for k, v := range e.Attrs {
			keys = append(keys, k+"="+v.String())
		}
		slices.Sort(keys)
		place := "unplaced"
		if e.Place.Known {
			place = fmt.Sprintf("at %.2f,%.2f", e.Place.Lat, e.Place.Lon)
		}
		fmt.Fprintf(&b, "%s %q %s %s %s\n", e.Ref, e.Name, e.Status.Level, place, strings.Join(keys, " "))
		if e.Status.Reason != "" {
			fmt.Fprintf(&b, "  because %s\n", e.Status.Reason)
		}
	}
	for _, e := range cs.Edges {
		fmt.Fprintf(&b, "%s %s %s\n", e.From, e.Rel, e.To)
	}
	return b.String()
}
