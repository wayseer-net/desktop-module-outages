package outages

import (
	"cmp"
	"context"
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

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

// signalLag is how far back each read asks again, since IODA fills recent points late: Google's
// lag two to three hours.
const signalLag = 3 * time.Hour

// signalStep is the finest step IODA keeps, which bounds the points asked for.
const signalStep = 5 * time.Minute

// signal is one of IODA's time series: values every step from from, null where none is known.
type signal struct {
	EntityType string            `json:"entityType"`
	EntityCode string            `json:"entityCode"`
	Datasource string            `json:"datasource"`
	From       int64             `json:"from"`
	Step       int64             `json:"step"`
	Values     []json.RawMessage `json:"values"`
}

// points is the signal's known values; IODA gives some signals as objects, which it skips.
func (s *signal) points() []sdk.Point {
	var out []sdk.Point
	for i, raw := range s.Values {
		var v float64
		if string(raw) == "null" || json.Unmarshal(raw, &v) != nil {
			continue
		}
		out = append(out, sdk.Point{T: time.Unix(s.From+int64(i)*s.Step, 0).UnixNano(), V: v})
	}
	return out
}

// signalsSince is where a read of signals starts: the lookback the first time, then the last
// read less the lag.
func signalsSince(last, now time.Time, lookback time.Duration) time.Time {
	oldest := now.Add(-lookback)
	if last.IsZero() {
		return oldest
	}
	return maxTime(last.Add(-signalLag), oldest)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

// fetchSignals reads each charted signal of the world's entities from since to until.
func fetchSignals(ctx context.Context, c *client, w *world, o *options, since, until time.Time) (map[sdk.SeriesRef][]sdk.Point, error) {
	out := map[sdk.SeriesRef][]sdk.Point{}
	for _, typ := range []string{"country", "region", "asn"} {
		codes := w.codes(typ)
		for _, ds := range o.Signals {
			if mt, _ := metricOf(ds); len(codes) == 0 || !slices.Contains(mt.Kinds, kinds[typ]) {
				continue
			}
			if err := fetchSignal(ctx, c, w, typ, codes, ds, since, until, out); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// fetchSignal reads one signal of the world's entities of one type into out.
func fetchSignal(ctx context.Context, c *client, w *world, typ string, codes []string, ds string, since, until time.Time, out map[sdk.SeriesRef][]sdk.Point) error {
	q := url.Values{
		"datasource": {ds}, "from": {unix(since)}, "until": {unix(until)},
		"maxPoints": {strconv.Itoa(int(until.Sub(since)/signalStep) + 1)},
	}
	var groups [][]signal
	if err := c.get(ctx, "signals/raw/"+typ+"/"+escapeAll(codes), q, &groups); err != nil {
		return err
	}
	for _, g := range groups {
		for i := range g {
			s := &g[i]
			if ref, ok := w.refs[key(s.EntityType, s.EntityCode)]; ok && s.Datasource == ds {
				out[sdk.SeriesRef{Entity: ref, Metric: metricPrefix + ds}] = s.points()
			}
		}
	}
	return nil
}

// escapeAll is codes for a path, each escaped, joined by commas as IODA wants them.
func escapeAll(codes []string) string {
	esc := make([]string, len(codes))
	for i, c := range codes {
		esc[i] = url.PathEscape(c)
	}
	return strings.Join(esc, ",")
}

// series is one signal's points, oldest first, none older than the lookback.
type series struct{ points []sdk.Point }

// merge adds points, a new one replacing an old at the same time, and drops those before oldest.
func (s *series) merge(pts []sdk.Point, oldest int64) {
	byT := make(map[int64]float64, len(s.points)+len(pts))
	for _, p := range slices.Concat(s.points, pts) {
		if p.T >= oldest {
			byT[p.T] = p.V
		}
	}
	s.points = s.points[:0]
	for _, t := range slices.Sorted(maps.Keys(byT)) {
		s.points = append(s.points, sdk.Point{T: t, V: byT[t]})
	}
}

// in is the points within w; a nil series has none.
func (s *series) in(w sdk.TimeWindow) []sdk.Point {
	if s == nil {
		return nil
	}
	at := func(p sdk.Point, t int64) int { return cmp.Compare(p.T, t) }
	lo, _ := slices.BinarySearchFunc(s.points, w.From.UnixNano(), at)
	hi, _ := slices.BinarySearchFunc(s.points, w.To.UnixNano(), at)
	return slices.Clone(s.points[lo:max(lo, hi)])
}

// Metrics lists what QuerySeries can answer: every signal the module can chart.
func (m *Module) Metrics() []sdk.Metric { return slices.Clone(catalogue) }

// record merges a read's points into the series, and forgets series whose entity has gone or
// that the options no longer chart. Callers hold m.mu.
func (m *Module) record(pts map[sdk.SeriesRef][]sdk.Point, oldest time.Time) {
	for ref, p := range pts {
		s := m.series[ref]
		if s == nil {
			s = &series{}
			m.series[ref] = s
		}
		s.merge(p, oldest.UnixNano())
	}
	maps.DeleteFunc(m.series, func(ref sdk.SeriesRef, _ *series) bool {
		_, ok := m.world.ents[ref.Entity]
		return !ok || !m.charts(ref.Metric)
	})
}

// charts reports whether the options chart a metric. Callers hold m.mu.
func (m *Module) charts(metric string) bool {
	return slices.Contains(m.opts.Signals, strings.TrimPrefix(metric, metricPrefix))
}

// QuerySeries answers from the points read since Run started: a series, empty or not, for
// each queried entity that has the metric.
func (m *Module) QuerySeries(ctx context.Context, q sdk.SeriesQuery) ([]sdk.Series, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []sdk.Series
	for _, ref := range m.queried(q) {
		for _, name := range q.Metrics {
			mt, ok := inCatalogue(name)
			if ok && m.charts(name) && slices.Contains(mt.Kinds, ref.Kind()) {
				key := sdk.SeriesRef{Entity: ref, Metric: name}
				out = append(out, sdk.Series{Ref: key, Unit: mt.Unit, Points: m.series[key].in(q.Window)})
			}
		}
	}
	return out, nil
}

func inCatalogue(name string) (sdk.Metric, bool) {
	return metricOf(strings.TrimPrefix(name, metricPrefix))
}

// queried is the world's entities q names, or else those its filter matches, in ref order.
func (m *Module) queried(q sdk.SeriesQuery) []sdk.EntityRef {
	var out []sdk.EntityRef
	for _, ref := range slices.Sorted(maps.Keys(m.world.ents)) {
		e := m.world.ents[ref]
		if len(q.Entities) > 0 && slices.Contains(q.Entities, ref) || len(q.Entities) == 0 && q.Filter.Match(&e) {
			out = append(out, ref)
		}
	}
	return out
}
