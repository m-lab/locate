package heartbeat

import (
	"errors"
	"math"
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

	// ProbabilityOverrides maps a machine name (e.g.
	// mlab1-lga01.mlab-oti.measurement-lab.org) to a probability that
	// overrides whatever the machine reports in its registration. It is a
	// locate-side kill-switch for individual machines (e.g. an autojoined
	// node whose registration we don't control) without disabling its whole
	// organization. It is populated once at startup and read-only thereafter.
	ProbabilityOverrides = map[string]float64{}
)

// NearestSettings holds the startup-time configuration for the nearest
// selection algorithm.
type NearestSettings struct {
	// Weighted enables distance-weighted site sampling instead of the
	// exponential draw over the distance-ranked list.
	Weighted bool
	// EQRadiusKm is the distance, in km, below which sites are considered
	// equivalent: distances are clamped to this value before weighting.
	EQRadiusKm float64
	// MaxDistanceRatio drops candidate sites whose effective distance exceeds
	// this ratio times the nearest site's, always keeping at least the 4
	// nearest. Zero disables the cutoff.
	MaxDistanceRatio float64
}

// NearestConfig configures the nearest selection algorithm. Like
// ProbabilityOverrides, it is populated once at startup and read-only
// thereafter.
var NearestConfig = NearestSettings{
	EQRadiusKm:       100,
	MaxDistanceRatio: 20,
}

// Randomness seams. Production uses the goroutine-safe default math/rand
// source; tests substitute functions backed by a seeded *rand.Rand for
// deterministic draws.
var (
	randFloat64           = rand.Float64
	randInt               = mathx.GetRandomInt
	randExpDistributedInt = mathx.GetExpDistributedInt
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
	Org     string   // Limit results to only machines from this organization.
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
	host   string
	health v2.Health
}

// site groups v2.HeartbeatMessage instances based on v2.Registration.Site.
type site struct {
	distance     float64
	rank         int
	metroRank    int
	registration v2.Registration
	machines     []machine
	// overridden is true when a ProbabilityOverrides entry set this site's
	// probability. Overridden sites are always subject to probability
	// filtering, even for queries that otherwise skip it (alwaysPick), so the
	// override acts as a reliable kill-switch.
	overridden bool
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

// Nearest discovers the nearest machines for the target service. Sites are
// sampled either through an exponential draw over the distance-ranked list
// (the default) or, when NearestConfig.Weighted is set, proportionally to
// 1/max(distance, EQRadiusKm)^2, so that nearby sites share the load instead
// of the closest one capturing nearly all of it.
func (l *Locator) Nearest(service string, lat, lon float64, opts *NearestOptions) (*TargetInfo, error) {
	// Filter.
	sites := filterSites(service, lat, lon, l.Instances(), opts)

	// Sort.
	sortSites(sites)

	// Rank.
	rank(sites)

	// Pick.
	result := pickTargets(service, sites, opts)

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
		// Apply a per-machine probability override, if configured. Because
		// the pick happens at the site level, this overrides the effective
		// probability for the site this machine belongs to (an autojoined
		// node is its own single-machine site).
		if p, ok := ProbabilityOverrides[machineName.String()]; ok {
			s.registration.Probability = p
			s.overridden = true
		}
		s.machines = append(s.machines, machine{
			name:   machineName.String(),
			host:   machineName.StringWithService(),
			health: *v.Health})
	}

	sites := make([]site, 0)
	for _, v := range m {
		// An explicit per-machine override always applies probability
		// filtering, even when the query would otherwise skip it; this keeps
		// the override effective as a kill-switch for org- or virtual-targeted
		// queries.
		if (alwaysPick(opts) && !v.overridden) || pickWithProbability(v.registration.Probability) {
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

	if opts.Country != "" && opts.Strict && r.CountryCode != opts.Country {
		return false, host.Name{}, 0
	}

	if opts.Org != "" {
		// We are filtering on user-specified organization.
		if opts.Org != "mlab" && machineName.Version == "v2" {
			// All v2 names are "mlab" managed.
			return false, host.Name{}, 0
		}
		if machineName.Version == "v3" && opts.Org != machineName.Org {
			return false, host.Name{}, 0
		}
		// NOTE: Org == "mlab" will allow all v2 names.
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

// pickTargets picks up to 4 sites from the distance-sorted candidates, using
// either the exponential-rank draw or, when NearestConfig.Weighted is set,
// sampling proportional to each site's distance weight. For each site, it
// picks a machine at random and returns them as []v2.Target.
// For any of the picked targets, it also returns the service URL templates as []url.URL.
func pickTargets(service string, sites []site, opts *NearestOptions) *TargetInfo {
	weighted := NearestConfig.Weighted
	if weighted {
		sites = truncateByDistance(sites)
		metrics.NearestSelectionTotal.WithLabelValues("weighted").Inc()
	} else {
		metrics.NearestSelectionTotal.WithLabelValues("legacy").Inc()
	}

	numTargets := mathx.Min(4, len(sites))
	targets := make([]v2.Target, numTargets)
	ranks := make(map[string]int)
	var urls []url.URL

	for i := 0; i < numTargets; i++ {
		var index int
		if weighted {
			index = pickSiteIndexWeighted(sites, opts)
		} else {
			index = pickSiteIndex(sites)
		}
		s := sites[index]
		metrics.ServerDistanceRanking.WithLabelValues(strconv.Itoa(i)).Observe(float64(s.rank))
		metrics.MetroDistanceRanking.WithLabelValues(strconv.Itoa(i)).Observe(float64(s.metroRank))
		// TODO(cristinaleon): Once health values range between 0 and 1,
		// pick based on health. For now, pick at random.
		machineIndex := randInt(len(s.machines))
		machine := s.machines[machineIndex]

		r := s.registration
		targets[i] = v2.Target{
			Machine:  machine.name,
			Hostname: machine.host,
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

// pickSiteIndex returns the index of the next site to select from the
// distance-sorted candidate list.
func pickSiteIndex(sites []site) int {
	// A rate of 6 yields index 0 around 95% of the time, index 1 a little less
	// than 5% of the time, and higher indices infrequently.
	return randExpDistributedInt(6) % len(sites)
}

// pickSiteIndexWeighted samples a site index proportionally to each site's
// weight.
func pickSiteIndexWeighted(sites []site, opts *NearestOptions) int {
	weights := make([]float64, len(sites))
	total := 0.0
	for i := range sites {
		weights[i] = weight(&sites[i], opts)
		total += weights[i]
	}
	r := randFloat64() * total
	sum := 0.0
	for i, w := range weights {
		sum += w
		if r < sum {
			return i
		}
	}
	// Only reachable through floating-point rounding.
	return len(sites) - 1
}

// effDist returns the effective distance used for weighting: distances below
// the equivalence radius are clamped so that all nearby sites weigh the same.
// TODO(m-lab/locate): EQRadiusKm may need to be regional (e.g. a function of
// the site's country or metro); the site's registration is available for that.
func effDist(s *site) float64 {
	return math.Max(s.distance, NearestConfig.EQRadiusKm)
}

// weight returns the sampling weight for a site:
// bias / max(distance, EQRadiusKm)^2.
func weight(s *site, opts *NearestOptions) float64 {
	d := effDist(s)
	return countryBias(&s.registration, opts) * continentBias(&s.registration, opts) / (d * d)
}

// countryBias is a seam for a future in-country preference: a multiplier
// applied to the site's weight. No bias is applied for now.
func countryBias(r *v2.Registration, opts *NearestOptions) float64 {
	return 1.0
}

// continentBias is a seam for a future in-continent preference: a multiplier
// applied to the site's weight. No bias is applied for now.
func continentBias(r *v2.Registration, opts *NearestOptions) float64 {
	return 1.0
}

// truncateByDistance drops sites whose effective distance exceeds
// NearestConfig.MaxDistanceRatio times the nearest site's, always keeping at
// least the 4 nearest sites. The input must be sorted by distance. A ratio of
// zero disables the cutoff.
func truncateByDistance(sites []site) []site {
	ratio := NearestConfig.MaxDistanceRatio
	if ratio == 0 || len(sites) <= 4 {
		return sites
	}
	maxDist := ratio * effDist(&sites[0])
	for i := 4; i < len(sites); i++ {
		if effDist(&sites[i]) > maxDist {
			return sites[:i]
		}
	}
	return sites
}

func alwaysPick(opts *NearestOptions) bool {
	// Sites do not need further filtering if the query is already requesting
	// only virtual machines or a specific set of sites or a specific org.
	return opts.Type == "virtual" || len(opts.Sites) > 0 || opts.Org != ""
}

// pickWithProbability returns true if a pseudo-random number in the interval
// [0.0,1.0) is less than the given site's defined probability.
func pickWithProbability(probability float64) bool {
	return randFloat64() < probability
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
