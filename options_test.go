package outages

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"wayseer.dev/sdk/sdktest"
)

func TestDefaultsWatchEveryCountry(t *testing.T) {
	m := configured(t, "")
	o := m.opts
	if o.API != defaultAPI || len(o.Countries) != 0 || len(o.ASNs) != 0 || o.Regions {
		t.Errorf("options %+v", o)
	}
	if !slices.Equal(o.Signals, []string{"bgp", "ping-slash24", "merit-nt"}) {
		t.Errorf("signals %v", o.Signals)
	}
	if o.Interval != 5*time.Minute || o.Lookback != 24*time.Hour || o.Timeout != time.Minute {
		t.Errorf("interval %v, lookback %v, timeout %v", o.Interval, o.Lookback, o.Timeout)
	}
}

func TestCountriesAreUpperCasedAndDeduplicated(t *testing.T) {
	o := configured(t, "countries: [nz, NZ, au]\nasns: [3307, 3307]\nsignals: [bgp, bgp]").opts
	if !slices.Equal(o.Countries, []string{"AU", "NZ"}) || !slices.Equal(o.ASNs, []uint32{3307}) || !slices.Equal(o.Signals, []string{"bgp"}) {
		t.Errorf("options %+v", o)
	}
}

func TestBadOptionsAreRejected(t *testing.T) {
	for _, c := range []struct{ opts, want string }{
		{"api: ftp://example.test/v2", "http"},
		{"api: http://user:pass@example.test/v2", "credentials"},
		{"countries: [NZL]", "two letters"},
		{"countries: ['N1']", "two letters"},
		{"regions: true", "regions need countries"},
		{"regions: true\ncountries: " + countryList(21), "at most 20"},
		{"asns: [0]", "asn 0"},
		{"asns: " + asnList(51), "at most 50"},
		{"signals: []", "signals"},
		{"signals: [mozilla]", "mozilla"},
		{"interval: 30s", "interval"},
		{"lookback: 30m", "lookback"},
		{"lookback: 200h", "lookback"},
		{"interval: 2h\nlookback: 1h", "lookback"},
		{"timeout: 0s", "timeout"},
		{"no_such_option: 1", "no_such_option"},
	} {
		cfg, err := sdktest.Config("out", c.opts)
		if err != nil {
			t.Fatal(err)
		}
		err = New().Configure(context.Background(), cfg)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q: %v, want an error naming %q", c.opts, err, c.want)
		}
	}
}

// asnList is ASNs 1 to n, as a YAML flow list.
func asnList(n int) string {
	asns := make([]string, n)
	for i := range asns {
		asns[i] = strconv.Itoa(i + 1)
	}
	return "[" + strings.Join(asns, ", ") + "]"
}

// countryList is n distinct two-letter codes, as a YAML flow list.
func countryList(n int) string {
	codes := make([]string, n)
	for i := range codes {
		codes[i] = string(rune('A'+i/26)) + string(rune('A'+i%26))
	}
	return "[" + strings.Join(codes, ", ") + "]"
}
