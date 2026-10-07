//go:build ignore

// Writes places/countries.csv and places/regions.csv from Natural Earth's GeoJSON (public domain):
//
//	go run scripts/places.go <10m admin_0 countries> <10m admin_0 map units> <10m admin_1 states provinces>
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
)

type props map[string]any

func main() {
	if len(os.Args) != 4 {
		log.Fatal("usage: go run scripts/places.go <countries> <map units> <states provinces>")
	}
	countries := map[string]string{}
	for _, f := range os.Args[1:3] {
		for _, p := range features(f) {
			code := str(p, "ISO_A2_EH")
			if code == "-99" || code == "" {
				code = str(p, "ISO_A2")
			}
			if _, ok := countries[code]; !ok && len(code) == 2 {
				countries[code] = point(p["LABEL_Y"], p["LABEL_X"])
			}
		}
	}
	regions := map[string]string{}
	for _, p := range features(os.Args[3]) {
		regions[strconv.Itoa(int(p["diss_me"].(float64)))] = point(p["latitude"], p["longitude"])
	}
	write("places/countries.csv", countries)
	write("places/regions.csv", regions)
}

func features(path string) []props {
	b, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var fc struct {
		Features []struct {
			Properties props `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(b, &fc); err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	out := make([]props, 0, len(fc.Features))
	for _, f := range fc.Features {
		out = append(out, f.Properties)
	}
	return out
}

func str(p props, k string) string { s, _ := p[k].(string); return s }

func point(lat, lon any) string {
	return fmt.Sprintf("%.2f,%.2f", lat.(float64), lon.(float64))
}

func write(path string, rows map[string]string) {
	keys := make([]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	for _, k := range keys {
		fmt.Fprintf(f, "%s,%s\n", k, rows[k])
	}
}
