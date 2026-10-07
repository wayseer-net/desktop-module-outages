package outages

import (
	"context"
	"testing"

	"wayseer.dev/sdk"
	"wayseer.dev/sdk/sdktest"
)

// configured is a module configured with options.
func configured(t *testing.T, options string) *Module {
	t.Helper()
	cfg, err := sdktest.Config("out", options)
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	if err := m.Configure(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestConformance(t *testing.T) {
	sdktest.Conform(t, sdktest.Case{
		New:      func() sdk.Module { return New() },
		Name:     "out",
		Manifest: "manifest.yaml",
	})
}
