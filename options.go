package outages

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"
)

// defaultAPI is IODA's API v2, the only host the module reaches unless api names a mirror.
const defaultAPI = "https://api.ioda.inetintel.cc.gatech.edu/v2"

// Bounds that keep the working set, and the load on IODA, small.
const (
	maxCountries       = 300
	maxRegionCountries = 20
	maxASNs            = 50
	minInterval        = time.Minute
	minLookback        = time.Hour
	maxLookback        = 7 * 24 * time.Hour
)

type options struct {
	API       string        `yaml:"api"`       // IODA's API v2; another only for a mirror or tests
	Countries []string      `yaml:"countries"` // ISO 3166-1 alpha-2 codes; none for every country
	Regions   bool          `yaml:"regions"`   // also the regions of each listed country
	ASNs      []uint32      `yaml:"asns"`      // autonomous systems to watch
	Signals   []string      `yaml:"signals"`   // which of IODA's signals to chart
	Interval  time.Duration `yaml:"interval"`  // how often IODA is read
	Lookback  time.Duration `yaml:"lookback"`  // how far back outages and signals reach
	Timeout   time.Duration `yaml:"timeout"`   // longest wait for one request
}

func defaults() options {
	return options{
		API: defaultAPI, Signals: []string{"bgp", "ping-slash24", "merit-nt"},
		Interval: 5 * time.Minute, Lookback: 24 * time.Hour, Timeout: time.Minute,
	}
}

// normalise upper-cases country codes and drops repeats, keeping the signals' order.
func (o *options) normalise() {
	for i, c := range o.Countries {
		o.Countries[i] = strings.ToUpper(c)
	}
	slices.Sort(o.Countries)
	o.Countries = slices.Compact(o.Countries)
	slices.Sort(o.ASNs)
	o.ASNs = slices.Compact(o.ASNs)
	var signals []string
	for _, s := range o.Signals {
		if !slices.Contains(signals, s) {
			signals = append(signals, s)
		}
	}
	o.Signals = signals
}

func (o *options) validate() error {
	return errors.Join(checkAPI(o.API), o.checkCountries(), o.checkASNs(), o.checkSignals(), o.checkTimes())
}

func (o *options) checkCountries() error {
	for _, c := range o.Countries {
		if len(c) != 2 || strings.Trim(c, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
			return fmt.Errorf("country %q must be two letters, as ISO 3166-1 alpha-2", c)
		}
	}
	switch {
	case len(o.Countries) > maxCountries:
		return fmt.Errorf("countries: at most %d", maxCountries)
	case o.Regions && len(o.Countries) == 0:
		return errors.New("regions need countries listed: every country's regions would be too many")
	case o.Regions && len(o.Countries) > maxRegionCountries:
		return fmt.Errorf("regions: at most %d countries", maxRegionCountries)
	}
	return nil
}

func (o *options) checkASNs() error {
	if len(o.ASNs) > maxASNs {
		return fmt.Errorf("asns: at most %d", maxASNs)
	}
	if slices.Contains(o.ASNs, 0) {
		return errors.New("asn 0 is not an autonomous system")
	}
	return nil
}

func (o *options) checkSignals() error {
	if len(o.Signals) == 0 {
		return errors.New("signals: name at least one")
	}
	for _, s := range o.Signals {
		if _, ok := metricOf(s); !ok {
			return fmt.Errorf("signal %q is not one of %s", s, strings.Join(signalNames(), ", "))
		}
	}
	return nil
}

func (o *options) checkTimes() error {
	switch {
	case o.Interval < minInterval:
		return fmt.Errorf("interval %v must be at least %v, to spare IODA", o.Interval, minInterval)
	case o.Lookback < minLookback || o.Lookback > maxLookback:
		return fmt.Errorf("lookback %v must be from %v to %v", o.Lookback, minLookback, maxLookback)
	case o.Lookback < o.Interval:
		return fmt.Errorf("lookback %v must be at least the interval, %v", o.Lookback, o.Interval)
	case o.Timeout <= 0 || o.Timeout > 5*time.Minute:
		return fmt.Errorf("timeout %v must be above 0 and at most 5m", o.Timeout)
	}
	return nil
}

func checkAPI(raw string) error {
	u, err := url.Parse(raw)
	switch {
	case err != nil:
		return fmt.Errorf("api: %w", err)
	case u.Scheme != "http" && u.Scheme != "https" || u.Host == "":
		return fmt.Errorf("api %q must be http:// or https:// with a host", raw)
	case u.User != nil:
		return errors.New("api must not hold credentials")
	}
	return nil
}
