package heartbeat

import (
	"errors"
	"math/rand"
	"net/url"
	"sort"
	"strconv"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/mathx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/metrics"
	"github.com/m-lab/locate/static"
)

var (
	// ErrNoAvailableServers is returned when there are no available servers
	ErrNoAvailableServers = errors.New("no available M-Lab servers")
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
	Strict  bool     // When used with Country, limit results to only machines in this country.
}

// TargetInfo returns the set of `v2.Target` to run the measurement on with the
// necessary information to create their URLs.
type TargetInfo struct {
	Targets []v2.Target    // Targets to run a measurement on.
	URLs    []url.URL      // Service URL templates.
	Ranks   map[string]int // Map of machines to metro rankings.
}

// machine associates a machine name with its v2.Health value.
type machine struct {
	name   string
	health v2.Health
}

// site groups v2.HeartbeatMessage instances based on v2.Registration.Site.
type site struct {
	distance     float64
	rank         int
	metroRank    int
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
	Ready() bool
}

// NewServerLocator creates a new Locator instance.
func NewServerLocator(tracker StatusTracker) *Locator {
	return &Locator{
		StatusTracker: tracker,
	}
}

// Nearest discovers the nearest machines for the target service, using
// an exponentially distributed function based on distance.
func (l *Locator) Nearest(service string, lat, lon float64, opts *NearestOptions) (*TargetInfo, error) {
	// Filter.
	sites := filterSites(service, lat, lon, l.Instances(), opts)

	// Sort.
	sortSites(sites)

	// Rank.
	rank(sites)

	// Pick.
	result := pickTargets(service, sites)

	if len(result.Targets) == 0 {
		return nil, ErrNoAvailableServers
	}

	return result, nil
}

// filterSites groups the v2.HeartbeatMessage instances into sites and returns
// only those that can serve the client request.
func filterSites(service string, lat, lon float64, instances map[string]v2.HeartbeatMessage, opts *NearestOptions) []site {
	m := make(map[string]*site)

	for _, v := range instances {
		isValid, hostname, distance := isValidInstance(service, lat, lon, v, opts)
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
		s.machines = append(s.machines, machine{name: hostname, health: *v.Health})
	}

	sites := make([]site, 0)
	for _, v := range m {
		if alwaysPick(opts) || pickWithProbability(v.registration.Probability) {
			sites = append(sites, *v)
		}
	}

	return sites
}

// isValidInstance returns whether a v2.HeartbeatMessage signals a valid
// instance that can serve a request given its parameters.
func isValidInstance(service string, lat, lon float64, v v2.HeartbeatMessage, opts *NearestOptions) (bool, string, float64) {
	if !isHealthy(v) {
		return false, "", 0
	}

	r := v.Registration

	_, err := host.Parse(r.Hostname)
	if err != nil {
		return false, "", 0
	}

	if opts.Type != "" && opts.Type != r.Type {
		return false, "", 0
	}

	if opts.Sites != nil && !contains(opts.Sites, r.Site) {
		return false, "", 0
	}

	if opts.Country != "" && opts.Strict && r.CountryCode != opts.Country {
		return false, "", 0
	}

	if _, ok := r.Services[service]; !ok {
		return false, "", 0
	}

	distance := mathx.GetHaversineDistance(lat, lon, r.Latitude, r.Longitude)
	if distance > static.EarthHalfCircumferenceKm {
		return false, "", 0
	}

	return true, r.Hostname, distance
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

// rank ranks sites and metros.
func rank(sites []site) {
	metroRank := 0
	metros := make(map[string]int)
	for i, site := range sites {
		// Update site rank.
		sites[i].rank = i

		// Update metro rank.
		metro := site.registration.Metro
		_, ok := metros[metro]
		if !ok {
			metros[metro] = metroRank
			metroRank++
		}
		sites[i].metroRank = metros[metro]
	}
}

// pickTargets picks up to 4 sites using an exponentially distributed function based
// on distance. For each site, it picks a machine at random and returns them
// as []v2.Target.
// For any of the picked targets, it also returns the service URL templates as []url.URL.
func pickTargets(service string, sites []site) *TargetInfo {
	numTargets := mathx.Min(4, len(sites))
	targets := make([]v2.Target, numTargets)
	ranks := make(map[string]int)
	var urls []url.URL

	for i := 0; i < numTargets; i++ {
		// A rate of 6 yields index 0 around 95% of the time, index 1 a little less
		// than 5% of the time, and higher indices infrequently.
		index := mathx.GetExpDistributedInt(6) % len(sites)
		s := sites[index]
		metrics.ServerDistanceRanking.WithLabelValues(strconv.Itoa(i)).Observe(float64(s.rank))
		metrics.MetroDistanceRanking.WithLabelValues(strconv.Itoa(i)).Observe(float64(s.metroRank))
		// TODO(cristinaleon): Once health values range between 0 and 1,
		// pick based on health. For now, pick at random.
		machineIndex := mathx.GetRandomInt(len(s.machines))
		machine := s.machines[machineIndex]

		r := s.registration
		m, _ := host.Parse(machine.name)
		m.Service = ""
		targets[i] = v2.Target{
			Machine:  m.String(),
			Hostname: machine.name,
			Location: &v2.Location{
				City:    r.City,
				Country: r.CountryCode,
			},
			URLs: make(map[string]string),
		}
		ranks[machine.name] = s.metroRank

		// Remove the selected site from the set of candidates for the next target selection.
		sites = append(sites[:index], sites[index+1:]...)

		if urls == nil {
			urls = getURLs(service, r)
		}
	}

	return &TargetInfo{
		Targets: targets,
		URLs:    urls,
		Ranks:   ranks,
	}
}

func alwaysPick(opts *NearestOptions) bool {
	// Sites do not need further filtering if the query is already requesting
	// only virtual machines or a specific set of sites.
	return opts.Type == "virtual" || len(opts.Sites) > 0
}

// pickWithProbability returns true if a pseudo-random number in the interval
// [0.0,1.0) is less than the given site's defined probability.
func pickWithProbability(probability float64) bool {
	return rand.Float64() < probability
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
