package outages

import (
	"bufio"
	"embed"
	"strconv"
	"strings"
	"sync"
)

// Label points from Natural Earth 1:10m (public domain), keyed by ISO 3166-1 alpha-2 code and by
// the admin-1 diss_me that IODA gives a region as ne_region_id; scripts/places.go writes them.
//
//go:embed places/countries.csv places/regions.csv
var placeFiles embed.FS

type latLon struct{ lat, lon float32 }

var (
	countries = sync.OnceValue(func() map[string]latLon { return readPlaces("places/countries.csv") })
	regions   = sync.OnceValue(func() map[string]latLon { return readPlaces("places/regions.csv") })
)

// countryAt is where a country's label sits, by its alpha-2 code.
func countryAt(code string) (lat, lon float32, ok bool) { return lookup(countries(), code) }

// regionAt is where a region's label sits, by its Natural Earth id.
func regionAt(neID string) (lat, lon float32, ok bool) { return lookup(regions(), neID) }

func lookup(m map[string]latLon, key string) (lat, lon float32, ok bool) {
	p, ok := m[key]
	return p.lat, p.lon, ok
}

// readPlaces reads key,lat,lon rows; the files are ours, so a bad row is a build mistake and panics.
func readPlaces(name string) map[string]latLon {
	f, err := placeFiles.Open(name)
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]latLon{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, lat, lon := split3(sc.Text())
		out[key] = latLon{parse32(lat), parse32(lon)}
	}
	return out
}

func split3(row string) (a, b, c string) {
	a, rest, _ := strings.Cut(row, ",")
	b, c, _ = strings.Cut(rest, ",")
	return a, b, c
}

func parse32(s string) float32 {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		panic("places: " + err.Error())
	}
	return float32(f)
}
