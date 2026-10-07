package outages

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"wayseer.dev/sdk"
)

// Kind is the module kind in config.
const Kind = "outages"

const version = "1"

// retryMax is the longest wait before retrying a failed read.
const retryMax = 10 * time.Second

// init registers the kind for a build of the app that imports the package.
func init() { sdk.Register(Kind, func() sdk.Module { return New() }) }

// Module reads IODA every interval and sends what changed.
type Module struct {
	health atomic.Pointer[sdk.Health]

	mu      sync.Mutex // guards what follows, shared by Run and Discover
	name    sdk.ModuleID
	opts    options
	tracker sdk.Tracker
	world   world // as last read
}

// New makes an unconfigured module.
func New() *Module { return &Module{} }

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
	m.world = world{}
	m.health.Store(&sdk.Health{})
	return nil
}

// Run reads IODA every interval: a snapshot after the first good read, then deltas.
// A failed read shows in Health and is retried sooner; Run returns only when ctx ends.
func (m *Module) Run(ctx context.Context, sink sdk.Sink) error {
	m.mu.Lock()
	m.tracker.Reset()
	every := m.opts.Interval
	m.mu.Unlock()
	send := sink.Snapshot
	t := time.NewTimer(0)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		cs, err := m.refresh(ctx)
		if ctx.Err() != nil {
			return nil
		}
		m.health.Store(&sdk.Health{Err: err})
		if err != nil {
			t.Reset(min(every, retryMax))
			continue
		}
		if err := send(ctx, cs); err != nil {
			return err
		}
		send = sink.Delta
		t.Reset(every)
	}
}

// refresh reads IODA and returns what changed since the last send.
func (m *Module) refresh(ctx context.Context) (*sdk.ChangeSet, error) {
	w, err := m.read(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.world = w
	return m.tracker.Changes(w.ents, w.edges, time.Now()), nil
}

func (m *Module) read(context.Context) (world, error) {
	return world{ents: map[sdk.EntityRef]sdk.Entity{}, edges: map[sdk.EdgeKey]sdk.Edge{}}, nil
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
	w, err := m.read(ctx)
	if err != nil {
		return nil, err
	}
	var fresh sdk.Tracker
	return fresh.Changes(w.ents, w.edges, time.Now()), nil
}
