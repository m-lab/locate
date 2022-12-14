package heartbeat

import (
	"errors"
	"net/url"
	"sort"
	"time"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/mathx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/static"
)

var (
	// ErrNoAvailableServers is returned when there are no available servers
	ErrNoAvailableServers = errors.New("no available M-Lab servers")
	rand                  = mathx.NewRandom(time.Now().UnixNano())
)

// Locator manages requests to "locate" mlab-ns servers.
type Locator struct {
	StatusTracker
}

// NearestOptions allows clients to pass parameters modifying how results are
// filtered.
type NearestOptions struct {
	Type    string   // Limit results to only machines of this type.
	Sites   []string // Limit results to only machines at these sites.
	Country string   // Bias results to prefer machines in this country.
}

// machine associates a machine name with its v2.Health value.
type machine struct {
	name   string
	health v2.Health
}

// site groups v2.HeartbeatMessage instances based on v2.Registration.Site.
type site struct {
	distance     float64
	registration v2.Registration
	machines     []machine
}

// StatusTracker defines the interface for tracking the status of experiment instances.
type StatusTracker interface {
	RegisterInstance(rm v2.Registration) error
	UpdateHealth(hostname string, hm v2.Health) error
	UpdatePrometheus(hostnames, machines map[string]bool) error
	Instances() map[string]v2.HeartbeatMessage
	StopImport()
}

// NewServerLocator creates a new Locator instance.
func NewServerLocator(tracker StatusTracker) *Locator {
	return &Locator{
		StatusTracker: tracker,
	}
}

// Nearest discovers the nearest machines for the target service, using
// an exponentially distributed function based on distance.
func (l *Locator) Nearest(service string, lat, lon float64, opts *NearestOptions) ([]v2.Target, []url.URL, error) {
	// Filter.
	sites := filterSites(service, lat, lon, l.Instances(), opts)

	// Sort.
	sortSites(sites)

	// Pick.
	targets, urls := pickTargets(service, sites)

	if len(targets) == 0 || len(urls) == 0 {
		return nil, nil, ErrNoAvailableServers
	}

	return targets, urls, nil
}

// filterSites groups the v2.HeartbeatMessage instances into sites and returns
// only those that can serve the client request.
func filterSites(service string, lat, lon float64, instances map[string]v2.HeartbeatMessage, opts *NearestOptions) []site {
	m := make(map[string]*site)

	for _, v := range instances {
		isValid, machineName, distance := isValidInstance(service, lat, lon, v, opts)
		if !isValid {
			continue
		}

		r := v.Registration
		s, ok := m[r.Site]
		if !ok {
			s = &site{
				distance:     distance,
				registration: *r,
				machines:     make([]machine, 0),
			}
			s.registration.Hostname = ""
			s.registration.Machine = ""
			m[r.Site] = s
		}
		s.machines = append(s.machines, machine{name: machineName.String(), health: *v.Health})
	}

	sites := make([]site, 0)
	for _, v := range m {
		if pickWithProbability(v.registration.Site) {
			sites = append(sites, *v)
		}
	}

	return sites
}

// isValidInstance returns whether a v2.HeartbeatMessage signals a valid
// instance that can serve a request given its parameters.
func isValidInstance(service string, lat, lon float64, v v2.HeartbeatMessage, opts *NearestOptions) (bool, host.Name, float64) {
	if !isHealthy(v) {
		return false, host.Name{}, 0
	}

	r := v.Registration

	machineName, err := host.Parse(r.Hostname)
	if err != nil {
		return false, host.Name{}, 0
	}

	if opts.Type != "" && opts.Type != r.Type {
		return false, host.Name{}, 0
	}

	if opts.Sites != nil && !contains(opts.Sites, r.Site) {
		return false, host.Name{}, 0
	}

	if _, ok := r.Services[service]; !ok {
		return false, host.Name{}, 0
	}

	distance := mathx.GetHaversineDistance(lat, lon, r.Latitude, r.Longitude)
	if distance > static.EarthHalfCircumferenceKm {
		return false, host.Name{}, 0
	}

	return true, machineName, distance
}

func isHealthy(v v2.HeartbeatMessage) bool {
	if v.Registration == nil || v.Health == nil || v.Health.Score == 0 {
		return false
	}

	if v.Prometheus != nil && !v.Prometheus.Health {
		return false
	}

	return true
}

// contains reports whether the given string array contains the given value.
func contains(sa []string, value string) bool {
	for _, v := range sa {
		if v == value {
			return true
		}
	}
	return false
}

// sortSites sorts a []site in ascending order based on distance.
func sortSites(sites []site) {
	sort.Slice(sites, func(i, j int) bool {
		return sites[i].distance < sites[j].distance
	})
}

// pickTargets picks up to 4 sites using an exponentially distributed function based
// on distance. For each site, it picks a machine at random and returns them
// as []v2.Target.
// For any of the picked targets, it also returns the service URL templates as []url.URL.
func pickTargets(service string, sites []site) ([]v2.Target, []url.URL) {
	numTargets := mathx.Min(4, len(sites))
	targets := make([]v2.Target, numTargets)
	var urls []url.URL

	for i := 0; i < numTargets; i++ {
		index := rand.GetExpDistributedInt(1) % len(sites)
		s := sites[index]
		// TODO(cristinaleon): Once health values range between 0 and 1,
		// pick based on health. For now, pick at random.
		machineIndex := rand.GetRandomInt(len(s.machines))
		machine := s.machines[machineIndex]

		r := s.registration
		targets[i] = v2.Target{
			Machine: machine.name,
			Location: &v2.Location{
				City:    r.City,
				Country: r.CountryCode,
			},
			URLs: make(map[string]string),
		}
		// Remove the selected site from the set of candidates for the next target selection.
		sites = append(sites[:index], sites[index+1:]...)

		if urls == nil {
			urls = getURLs(service, r)
		}
	}

	return targets, urls
}

// pickWithProbability returns true if a pseudo-random number in the interval
// [0.0,1.0) is less than the given site's defined probability (or if there is
// no explicit probability defined for the site).
func pickWithProbability(site string) bool {
	if val, ok := static.SiteProbability[site]; ok {
		return rand.Src.Float64() < val
	}
	return true
}

// getURLs extracts the URL templates from v2.Registration.Services and outputs
// them as a []url.Url.
func getURLs(service string, registration v2.Registration) []url.URL {
	urls := registration.Services[service]
	result := make([]url.URL, len(urls))

	for i, u := range urls {
		url, error := url.Parse(u)
		if error != nil {
			continue
		}
		result[i] = *url
	}

	return result
}

func biasedDistance(country string, r *v2.Registration, distance float64) float64 {
	// The 'ZZ' country code is used for unknown or unspecified countries.
	if country == "" || country == "ZZ" {
		return distance
	}

	if country == r.CountryCode {
		return distance
	}

	return 2 * distance
}
