package outages

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"wayseer.dev/sdk"
)

// dashboard is IODA's own site, which each entity and outage links to.
const dashboard = "https://ioda.inetintel.cc.gatech.edu/"

// ongoingSlack is how near the end of the window an outage's end may be and it still be going:
// IODA ends an ongoing outage where the asked-for window ends.
const ongoingSlack = 10 * time.Minute

// outage is one of IODA's outage events: one signal of one entity dropping, scored by how far.
type outage struct {
	Entity     entity  `json:"entity"`
	From       int64   `json:"from"`
	Until      int64   `json:"until"`
	Score      float64 `json:"score"`
	Datasource string  `json:"datasource"`
	Method     string  `json:"method"`
}

// id names the outage for good: its entity, signal, method and start.
func (o *outage) id() string {
	return key(o.Entity.Type, o.Entity.Code) + "/" + o.Datasource + "." + o.Method + "@" + strconv.FormatInt(o.From, 10)
}

func (o *outage) start() time.Time { return time.Unix(o.From, 0) }
func (o *outage) end() time.Time   { return time.Unix(o.Until, 0) }

// ongoing reports whether the outage runs to the end of a window that ended at until.
func (o *outage) ongoing(until time.Time) bool { return !o.end().Before(until.Add(-ongoingSlack)) }

// fetchOutages reads the outages of the world's entities from lookback before until.
func fetchOutages(ctx context.Context, c *client, w *world, o *options, until time.Time) ([]outage, error) {
	var out []outage
	for _, typ := range []string{"country", "region", "asn"} {
		codes := w.codes(typ)
		if len(codes) == 0 {
			continue
		}
		q := url.Values{
			"entityType": {typ}, "format": {"ioda"},
			"from": {unix(until.Add(-o.Lookback))}, "until": {unix(until)},
		}
		if typ != "country" || len(o.Countries) > 0 {
			q.Set("entityCode", strings.Join(codes, ","))
		}
		var outs []outage
		if err := c.get(ctx, "outages/events", q, &outs); err != nil {
			return nil, err
		}
		out = append(out, outs...)
	}
	slices.SortFunc(out, func(a, b outage) int { return strings.Compare(a.id(), b.id()) })
	return out, nil
}

func unix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

// withStatus is w with each entity critical while one of its outages goes on, and OK otherwise.
func withStatus(w world, outs []outage, until time.Time) world {
	since := map[sdk.EntityRef]time.Time{}
	signals := map[sdk.EntityRef][]string{}
	for i := range outs {
		o := &outs[i]
		ref, ok := w.refs[key(o.Entity.Type, o.Entity.Code)]
		if !ok || !o.ongoing(until) {
			continue
		}
		if s, seen := since[ref]; !seen || o.start().Before(s) {
			since[ref] = o.start()
		}
		if !slices.Contains(signals[ref], o.Datasource) {
			signals[ref] = append(signals[ref], o.Datasource)
		}
	}
	ents := maps.Clone(w.ents)
	for ref, e := range ents {
		e.Status = sdk.Status{Level: sdk.StatusOK}
		if s, ok := since[ref]; ok {
			e.Status = sdk.Status{Level: sdk.StatusCrit, Reason: fmt.Sprintf("%s outage since %s",
				strings.Join(signals[ref], " and "), s.UTC().Format("2006-01-02 15:04 UTC"))}
		}
		ents[ref] = e
	}
	w.ents = ents
	return w
}

// news is the events for outages not seen before, or seen going on and now ended. known holds
// whether each outage seen last time was going on; news returns the same for this time.
func news(src sdk.ModuleID, w *world, outs []outage, until time.Time, known map[string]bool) ([]sdk.Event, map[string]bool) {
	var evs []sdk.Event
	seen := make(map[string]bool, len(outs))
	for i := range outs {
		o := &outs[i]
		ref, ok := w.refs[key(o.Entity.Type, o.Entity.Code)]
		if !ok {
			continue
		}
		ongoing := o.ongoing(until)
		was, before := known[o.id()]
		seen[o.id()] = ongoing
		switch {
		case !before:
			evs = append(evs, began(src, ref, w.ents[ref].Name, o, ongoing))
		case was && !ongoing:
			evs = append(evs, ended(src, ref, w.ents[ref].Name, o))
		}
	}
	return evs, seen
}

// began is the event for an outage first seen: going on, or over already.
func began(src sdk.ModuleID, ref sdk.EntityRef, name string, o *outage, ongoing bool) sdk.Event {
	msg := fmt.Sprintf("%s: %s outage began", name, o.Datasource)
	if !ongoing {
		msg = fmt.Sprintf("%s: %s outage for %v", name, o.Datasource, o.end().Sub(o.start()))
	}
	return sdk.Event{
		ID: o.id(), Entity: ref, At: o.start(), Severity: severity(o.Score), Kind: "outage",
		Message: msg, Fields: fields(o, ongoing), Source: src,
	}
}

// ended is the event for an outage seen going on that is now over.
func ended(src sdk.ModuleID, ref sdk.EntityRef, name string, o *outage) sdk.Event {
	return sdk.Event{
		ID: o.id() + "/ended", Entity: ref, At: o.end(), Severity: sdk.SevInfo, Kind: "outage-ended",
		Message: fmt.Sprintf("%s: %s outage ended after %v", name, o.Datasource, o.end().Sub(o.start())),
		Fields:  fields(o, false), Source: src,
	}
}

// severity grades IODA's score, which grows with how deep and long the drop is.
func severity(score float64) sdk.Severity {
	switch {
	case score < 1e3:
		return sdk.SevWarn
	case score < 1e5:
		return sdk.SevError
	}
	return sdk.SevCritical
}

func fields(o *outage, ongoing bool) map[string]sdk.Value {
	f := map[string]sdk.Value{
		"datasource": sdk.String(o.Datasource), "method": sdk.String(o.Method),
		"score": sdk.Number(o.Score), "start": sdk.Time(o.start()),
		"ioda": sdk.String(fmt.Sprintf("%s?from=%d&until=%d", link(o.Entity.Type, o.Entity.Code), o.From, o.Until)),
	}
	if !ongoing {
		f["end"] = sdk.Time(o.end())
	}
	return f
}

// link is an entity's page on IODA's dashboard.
func link(typ, code string) string { return dashboard + typ + "/" + url.PathEscape(code) }
