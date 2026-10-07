package outages

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"wayseer.dev/sdk"
)

// The kinds the module sends, one per IODA entity type.
const (
	kindCountry sdk.Kind = "outages/country"
	kindRegion  sdk.Kind = "outages/region"
	kindASN     sdk.Kind = "outages/asn"
)

// kinds maps IODA's entity types to the module's kinds.
var kinds = map[string]sdk.Kind{"country": kindCountry, "region": kindRegion, "asn": kindASN}

// entity is one of IODA's metadata entities, as entities/query and outages/events give it.
type entity struct {
	Code  string         `json:"code"`
	Name  string         `json:"name"`
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs"`
}

func (e *entity) attr(k string) string { s, _ := e.Attrs[k].(string); return s }

// key is how the module finds an IODA entity's ref: its type and code.
func key(typ, code string) string { return typ + "/" + code }

// world is what the module sends, keyed as sdk.Tracker wants it, and the IODA codes behind it.
type world struct {
	ents  map[sdk.EntityRef]sdk.Entity
	edges map[sdk.EdgeKey]sdk.Edge
	refs  map[string]sdk.EntityRef // by key(type, code)
	notes []string                 // codes asked for that IODA doesn't know
}

func newWorld() world {
	return world{ents: map[sdk.EntityRef]sdk.Entity{}, edges: map[sdk.EdgeKey]sdk.Edge{}, refs: map[string]sdk.EntityRef{}}
}

// codes is the IODA codes of one type in the world, sorted.
func (w *world) codes(typ string) []string {
	var out []string
	for k := range w.refs {
		if t, code, _ := strings.Cut(k, "/"); t == typ {
			out = append(out, code)
		}
	}
	slices.Sort(out)
	return out
}

// lister lists the entities the options name, one IODA request at a time.
type lister struct {
	c   *client
	src sdk.ModuleID
	o   *options
	w   world
}

// list reads the countries, regions and networks the options watch.
func list(ctx context.Context, c *client, src sdk.ModuleID, o *options) (world, error) {
	l := lister{c: c, src: src, o: o, w: newWorld()}
	for _, step := range []func(context.Context) error{l.countries, l.regions, l.asns} {
		if err := step(ctx); err != nil {
			return world{}, err
		}
	}
	return l.w, nil
}

func (l *lister) query(ctx context.Context, q url.Values) ([]entity, error) {
	var ents []entity
	err := l.c.get(ctx, "entities/query", q, &ents)
	return ents, err
}

// countries lists those the options name, or every country IODA knows.
func (l *lister) countries(ctx context.Context) error {
	q := url.Values{"entityType": {"country"}}
	if len(l.o.Countries) > 0 {
		q.Set("entityCode", strings.Join(l.o.Countries, ","))
	}
	ents, err := l.query(ctx, q)
	if err != nil {
		return err
	}
	for i := range ents {
		l.add(&ents[i], placeOf(countryAt(ents[i].Code)), nil)
	}
	l.missing("country", l.o.Countries)
	return nil
}

// regions lists each named country's regions, if the options ask for them.
func (l *lister) regions(ctx context.Context) error {
	if !l.o.Regions {
		return nil
	}
	for _, country := range l.w.codes("country") {
		ents, err := l.query(ctx, url.Values{"entityType": {"region"}, "relatedTo": {"country/" + country}})
		if err != nil {
			return err
		}
		for i := range ents {
			l.add(&ents[i], placeOf(regionAt(ents[i].attr("ne_region_id"))), []string{country})
		}
	}
	return nil
}

// asns lists the networks the options name, linked to the countries they serve.
func (l *lister) asns(ctx context.Context) error {
	if len(l.o.ASNs) == 0 {
		return nil
	}
	codes := make([]string, len(l.o.ASNs))
	for i, n := range l.o.ASNs {
		codes[i] = strconv.FormatUint(uint64(n), 10)
	}
	ents, err := l.query(ctx, url.Values{"entityType": {"asn"}, "entityCode": {strings.Join(codes, ",")}})
	if err != nil {
		return err
	}
	for i := range ents {
		served, err := l.query(ctx, url.Values{"entityType": {"country"}, "relatedTo": {"asn/" + ents[i].Code}})
		if err != nil {
			return err
		}
		l.add(&ents[i], sdk.Place{}, countryCodes(served))
	}
	l.missing("asn", codes)
	return nil
}

func countryCodes(ents []entity) []string {
	out := make([]string, len(ents))
	for i, e := range ents {
		out[i] = e.Code
	}
	slices.Sort(out)
	return out
}

// add makes an entity of e, a member of whichever of its countries the world holds.
func (l *lister) add(e *entity, place sdk.Place, countries []string) {
	kind, ok := kinds[e.Type]
	if !ok {
		return
	}
	ref, err := sdk.NewEntityRef(string(l.src), kind, e.Code)
	if err != nil {
		l.w.notes = append(l.w.notes, fmt.Sprintf("IODA's %s %q can't be shown: %v", e.Type, e.Code, err))
		return
	}
	l.w.refs[key(e.Type, e.Code)] = ref
	l.w.ents[ref] = sdk.Entity{Ref: ref, Kind: kind, Name: e.Name, Place: place, Attrs: attrsOf(e, countries), Source: l.src}
	for _, c := range countries {
		if to, ok := l.w.refs[key("country", c)]; ok {
			edge := sdk.Edge{From: ref, To: to, Rel: sdk.RelMemberOf, Weight: 1, Source: l.src}
			l.w.edges[sdk.EdgeKey{From: ref, To: to, Rel: edge.Rel}] = edge
		}
	}
}

// missing notes each code of typ the options name that IODA didn't list.
func (l *lister) missing(typ string, codes []string) {
	for _, c := range codes {
		if _, ok := l.w.refs[key(typ, c)]; !ok {
			l.w.notes = append(l.w.notes, fmt.Sprintf("IODA knows no %s %s", typ, c))
		}
	}
}

// attrsOf is what Detail shows of e: its code and IODA page, and what IODA says of a region or network.
func attrsOf(e *entity, countries []string) map[string]sdk.Value {
	a := map[string]sdk.Value{"code": sdk.String(e.Code), "ioda": sdk.String(link(e.Type, e.Code))}
	switch e.Type {
	case "region":
		a["country"] = sdk.String(e.attr("country_code"))
	case "asn":
		a["org"] = sdk.String(e.attr("org"))
		if n, err := strconv.ParseFloat(e.attr("ip_count"), 64); err == nil {
			a["addresses"] = sdk.Number(n)
		}
		a["countries"] = sdk.List(stringValues(countries)...)
	}
	maps.DeleteFunc(a, func(_ string, v sdk.Value) bool { return v.Type() == sdk.TypeString && v.Str() == "" })
	return a
}

func stringValues(ss []string) []sdk.Value {
	out := make([]sdk.Value, len(ss))
	for i, s := range ss {
		out[i] = sdk.String(s)
	}
	return out
}

func placeOf(lat, lon float32, ok bool) sdk.Place {
	if !ok {
		return sdk.Place{}
	}
	return sdk.At(lat, lon)
}
