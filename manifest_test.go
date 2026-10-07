package outages

import (
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"wayseer.dev/sdk/manifest"
)

// TestManifestDeclaresWhatTheModuleDoes keeps manifest.yaml in step with the module: its kinds,
// no actions, no keyring, and IODA's host as its only endpoint.
func TestManifestDeclaresWhatTheModuleDoes(t *testing.T) {
	m := readManifest(t)
	var kinds []string
	for _, k := range m.Kinds {
		kinds = append(kinds, k.Kind)
	}
	if want := []string{string(kindCountry), string(kindRegion), string(kindASN)}; !slices.Equal(kinds, want) {
		t.Errorf("manifest.yaml declares kinds %v, want %v", kinds, want)
	}
	api, err := url.Parse(defaultAPI)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{api.Host + ":443"}; !slices.Equal(m.Network, want) {
		t.Errorf("manifest.yaml declares network %v, want %v", m.Network, want)
	}
	if len(m.Actions) != 0 || m.Secrets {
		t.Errorf("manifest.yaml declares actions %v and secrets %v", m.Actions, m.Secrets)
	}
}

// readManifest reads manifest.yaml as dev sign does, filling the fields it writes.
func readManifest(t *testing.T) manifest.Manifest {
	t.Helper()
	data, err := os.ReadFile("manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, m, err := manifest.Fill(data, manifest.Platform{OS: "linux", Arch: "amd64", SHA256: strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
