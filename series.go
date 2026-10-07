package outages

import (
	"slices"

	"wayseer.dev/sdk"
)

// catalogue is the IODA signals the module charts: those whose points are single numbers.
// Each metric is named ioda.<datasource>, IODA's own name for the signal.
var catalogue = []sdk.Metric{
	{
		Name: "ioda.bgp", Unit: sdk.UnitCount, Kinds: []sdk.Kind{kindCountry, kindRegion, kindASN},
		Description: "/24 blocks visible in BGP to most full-feed peers", Native: "signals/raw datasource=bgp",
	},
	{
		Name: "ioda.ping-slash24", Unit: sdk.UnitCount, Kinds: []sdk.Kind{kindCountry, kindRegion, kindASN},
		Description: "/24 blocks that answer active probing", Native: "signals/raw datasource=ping-slash24",
	},
	{
		Name: "ioda.merit-nt", Unit: sdk.UnitCount, Kinds: []sdk.Kind{kindCountry, kindRegion, kindASN},
		Description: "unique source IPs a minute seen by the Merit network telescope", Native: "signals/raw datasource=merit-nt",
	},
	{
		Name: "ioda.gtr-norm", Unit: sdk.UnitRatio, Kinds: []sdk.Kind{kindCountry},
		Description: "Google Transparency Report traffic, normalised", Native: "signals/raw datasource=gtr-norm",
	},
}

// metricPrefix joins a datasource's name to make its metric's.
const metricPrefix = "ioda."

// metricOf is the catalogue's metric for an IODA datasource.
func metricOf(datasource string) (sdk.Metric, bool) {
	i := slices.IndexFunc(catalogue, func(m sdk.Metric) bool { return m.Name == metricPrefix+datasource })
	if i < 0 {
		return sdk.Metric{}, false
	}
	return catalogue[i], true
}

// signalNames is every datasource the catalogue charts.
func signalNames() []string {
	out := make([]string, len(catalogue))
	for i, m := range catalogue {
		out[i] = m.Name[len(metricPrefix):]
	}
	return out
}
