package outages

import "wayseer.dev/sdk"

// The kinds the module sends, one per IODA entity type.
const (
	kindCountry sdk.Kind = "outages/country"
	kindRegion  sdk.Kind = "outages/region"
	kindASN     sdk.Kind = "outages/asn"
)

// world is what the module sends, keyed as sdk.Tracker wants it.
type world struct {
	ents  map[sdk.EntityRef]sdk.Entity
	edges map[sdk.EdgeKey]sdk.Edge
}
