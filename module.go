package outages

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wayseer.dev/sdk"
)

// Kind is the module kind in config.
const Kind = "outages"

const version = "1"

// Waits between reads: retries after a failure back off from retryMin, and entities, which
// rarely change, are listed again only every relist.
const (
	retryMin  = 10 * time.Second
	relist    = 6 * time.Hour
	keepEvent = 1000 // events kept for QueryEvents
)

// init registers the kind for a build of the app that imports the package.
func init() { sdk.Register(Kind, func() sdk.Module { return New() }) }

// Module reads IODA every interval and sends what changed.
type Module struct {
	health atomic.Pointer[sdk.Health]
	now    func() time.Time // the clock; tests set it

	mu       sync.Mutex // guards what follows, shared by Run, Discover and the queries
	name     sdk.ModuleID
	opts     options
	api      *client
	tracker  sdk.Tracker
	listing  world           // the entities as last listed, without status
	listedAt time.Time       // when they were
	world    world           // as last sent
	known    map[string]bool // outages seen last read, and whether each was going on
	events   *sdk.EventLog   // sent, for QueryEvents
}

// New makes an unconfigured module.
func New() *Module { return &Module{now: time.Now} }

// Info describes the module.
func (m *Module) Info() sdk.Info {
	return sdk.Info{Kind: Kind, Version: version, Description: "Internet outages from IODA (Georgia Tech)"}
}

// Configure checks the options; nothing is fetched until Run or Discover.
func (m *Module) Configure(_ context.Context, cfg sdk.Config) error {
	o := defaults()
	if err := cfg.Decode(&o); err != nil {
		return err
	}
	o.normalise()
	if err := o.validate(); err != nil {
		return fmt.Errorf("line %d: %w", cfg.Line, err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.name, m.opts = cfg.Name, o
	m.api = newClient(o.API, o.Timeout)
	m.listing, m.listedAt, m.world = world{}, time.Time{}, newWorld()
	m.known, m.events = map[string]bool{}, sdk.NewEventLog(keepEvent)
	m.health.Store(&sdk.Health{})
	return nil
}

// Run reads IODA every interval: a snapshot after the first good read, then deltas.
// A failed read shows in Health and is retried with back-off; Run returns only when ctx ends.
func (m *Module) Run(ctx context.Context, sink sdk.Sink) error {
	m.mu.Lock()
	m.tracker.Reset()
	every := m.opts.Interval
	m.mu.Unlock()
	send := sink.Snapshot
	t := time.NewTimer(0)
	defer t.Stop()
	wait := retryMin
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		cs, note, err := m.refresh(ctx)
		if ctx.Err() != nil {
			return nil
		}
		m.health.Store(&sdk.Health{Err: err, Note: note})
		if err != nil {
			t.Reset(retryIn(err, wait, every))
			wait = min(2*wait, every)
			continue
		}
		if err := send(ctx, cs); err != nil {
			return err
		}
		send, wait = sink.Delta, retryMin
		t.Reset(every)
	}
}

// retryIn is how long to wait after err: as long as IODA asks, or else the back-off.
func retryIn(err error, backoff, every time.Duration) time.Duration {
	if wait, ok := retryAfter(err); ok {
		return max(wait, backoff)
	}
	return min(backoff, every)
}

// refresh reads IODA and returns what changed since the last send, with an event for each
// outage that began or ended, and a note of what IODA doesn't know.
func (m *Module) refresh(ctx context.Context) (*sdk.ChangeSet, string, error) {
	st, err := m.read(ctx, true)
	if err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	evs, known := news(m.name, &st.w, st.outages, st.until, m.known)
	m.world, m.known = st.w, known
	m.events.Add(evs...)
	cs := m.tracker.Changes(st.w.ents, st.w.edges, st.until)
	cs.Events = evs
	return cs, strings.Join(st.w.notes, "; "), nil
}

// state is one read of IODA: the entities with their status, and the outages behind it.
type state struct {
	w       world
	outages []outage
	until   time.Time
}

// read lists the entities, or reuses the last listing while it is fresh, and reads their outages.
func (m *Module) read(ctx context.Context, reuse bool) (state, error) {
	now := m.now()
	m.mu.Lock()
	c, name, o, listing, listedAt := m.api, m.name, m.opts, m.listing, m.listedAt
	m.mu.Unlock()
	if !reuse || listing.ents == nil || now.Sub(listedAt) >= relist {
		var err error
		if listing, err = list(ctx, c, name, &o); err != nil {
			return state{}, err
		}
		m.keepListing(reuse, listing, now)
	}
	outs, err := fetchOutages(ctx, c, &listing, &o, now)
	if err != nil {
		return state{}, err
	}
	return state{w: withStatus(listing, outs, now), outages: outs, until: now}, nil
}

// keepListing keeps a listing Run made, for its next reads.
func (m *Module) keepListing(reuse bool, w world, at time.Time) {
	if !reuse {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listing, m.listedAt = w, at
}

// Health reports whether the last read worked.
func (m *Module) Health() sdk.Health {
	if h := m.health.Load(); h != nil {
		return *h
	}
	return sdk.Health{}
}

// Discover reads IODA now and returns all of it.
func (m *Module) Discover(ctx context.Context) (*sdk.ChangeSet, error) {
	st, err := m.read(ctx, false)
	if err != nil {
		return nil, err
	}
	var fresh sdk.Tracker
	return fresh.Changes(st.w.ents, st.w.edges, st.until), nil
}

// QueryEvents answers from the outage events sent since the module was configured.
func (m *Module) QueryEvents(ctx context.Context, q sdk.EventQuery) ([]sdk.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events.Query(q), nil
}
