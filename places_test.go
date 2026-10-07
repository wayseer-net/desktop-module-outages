package outages

import "testing"

func TestCountriesAndRegionsHavePlaces(t *testing.T) {
	for _, c := range []struct {
		find     func(string) (float32, float32, bool)
		code     string
		lat, lon float32
	}{
		{countryAt, "NZ", -39.76, 172.79},
		{countryAt, "GP", 16.30, -61.43},
		{regionAt, "3334", -39.85, 175.63},
	} {
		lat, lon, ok := c.find(c.code)
		if !ok || lat != c.lat || lon != c.lon {
			t.Errorf("%s at %v,%v (%v), want %v,%v", c.code, lat, lon, ok, c.lat, c.lon)
		}
	}
	for _, unplaced := range []string{"EU", "AP", "", "nz"} {
		if _, _, ok := countryAt(unplaced); ok {
			t.Errorf("country %q has a place", unplaced)
		}
	}
	if _, _, ok := regionAt("0"); ok {
		t.Error("region 0 has a place")
	}
}
