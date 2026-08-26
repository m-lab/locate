package heartbeat

import (
	"math"
	"math/rand"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/mathx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
)

// seedRandom routes the package randomness seams through a deterministically
// seeded source for the duration of the test.
func seedRandom(t *testing.T, seed int64) {
	src := rand.New(rand.NewSource(seed))
	randFloat64 = src.Float64
	randInt = func(max int) int {
		if max <= 0 {
			return 0
		}
		return src.Intn(max)
	}
	randExpDistributedInt = func(rate float64) int {
		return int(math.Round(src.ExpFloat64() / rate))
	}
	t.Cleanup(func() {
		randFloat64 = rand.Float64
		randInt = mathx.GetRandomInt
		randExpDistributedInt = mathx.GetExpDistributedInt
	})
}

// setNearestConfig replaces NearestConfig for the duration of the test.
func setNearestConfig(t *testing.T, cfg NearestSettings) {
	prev := NearestConfig
	NearestConfig = cfg
	t.Cleanup(func() {
		NearestConfig = prev
	})
}

var (
	// Test services.
	validNDT7Services = map[string][]string{
		"ndt/ndt7": {
			"ws://ndt/v7/upload",
			"ws://ndt/v7/download",
			"wss://ndt/v7/upload",
			"wss://ndt/v7/download",
		},
	}

	// Test URLs.
	NDT7Urls = []url.URL{
		{
			Scheme: "ws",
			Host:   "ndt",
			Path:   "/v7/upload",
		},
		{
			Scheme: "ws",
			Host:   "ndt",
			Path:   "/v7/download",
		},
		{
			Scheme: "wss",
			Host:   "ndt",
			Path:   "/v7/upload",
		},
		{
			Scheme: "wss",
			Host:   "ndt",
			Path:   "/v7/download",
		},
	}

	// Test instances.
	virtualInstance1 = v2.HeartbeatMessage{
		Registration: &v2.Registration{
			City:          "New York",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-mlab1-lga00.mlab-sandbox.measurement-lab.org",
			Latitude:      40.7667,
			Longitude:     -73.8667,
			Machine:       "mlab1",
			Metro:         "lga",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "lga00",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		Health: &v2.Health{Score: 1},
	}
	virtualInstance2 = v2.HeartbeatMessage{
		Registration: &v2.Registration{
			City:          "New York",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-mlab2-lga00.mlab-sandbox.measurement-lab.org",
			Latitude:      40.7667,
			Longitude:     -73.8667,
			Machine:       "mlab2",
			Metro:         "lga",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "lga00",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		Health: &v2.Health{Score: 1},
	}
	physicalInstance = v2.HeartbeatMessage{
		Registration: &v2.Registration{
			City:          "Los Angeles",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-mlab1-lax00.mlab-sandbox.measurement-lab.org",
			Latitude:      33.9425,
			Longitude:     -118.4072,
			Machine:       "mlab1",
			Metro:         "lax",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "lax00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		Health: &v2.Health{Score: 1},
	}
	autonodeInstance = v2.HeartbeatMessage{
		Registration: &v2.Registration{
			City:          "Council Bluffs",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Hostname:      "ndt-oma396982-2248791f.foo.sandbox.measurement-lab.org",
			Latitude:      41.3032,
			Longitude:     -95.8941,
			Machine:       "2248791f",
			Metro:         "oma",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "oma396982",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		Health: &v2.Health{Score: 1},
	}
	weheInstance = v2.HeartbeatMessage{
		Registration: &v2.Registration{
			City:          "Portland",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "wehe",
			Hostname:      "wehe-mlab1-pdx00.mlab-sandbox.measurement-lab.org",
			Latitude:      45.5886,
			Longitude:     -122.5975,
			Machine:       "mlab1",
			Metro:         "pdx",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "pdx00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      map[string][]string{"wehe/replay": {"wss://4443/v0/envelope/access"}},
		},
		Health: &v2.Health{Score: 1},
	}

	// Test sites.
	virtualSite = site{
		distance: 296.04366543852825,
		registration: v2.Registration{
			City:          "New York",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Latitude:      40.7667,
			Longitude:     -73.8667,
			Metro:         "lga",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "lga00",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		machines: []machine{
			{
				name:   "mlab1-lga00.mlab-sandbox.measurement-lab.org",
				host:   "ndt-mlab1-lga00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
			{
				name:   "mlab2-lga00.mlab-sandbox.measurement-lab.org",
				host:   "ndt-mlab2-lga00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
		},
	}
	physicalSite = site{
		distance: 3838.617961615054,
		registration: v2.Registration{
			City:          "Los Angeles",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Latitude:      33.9425,
			Longitude:     -118.4072,
			Metro:         "lax",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "lax00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		machines: []machine{
			{
				name:   "mlab1-lax00.mlab-sandbox.measurement-lab.org",
				host:   "ndt-mlab1-lax00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
		},
	}
	autonodeSite = site{
		distance: 1701.749354381346,
		registration: v2.Registration{
			City:          "Council Bluffs",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "ndt",
			Latitude:      41.3032,
			Longitude:     -95.8941,
			Metro:         "oma",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "oma396982",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		machines: []machine{
			{
				name:   "ndt-oma396982-2248791f.foo.sandbox.measurement-lab.org",
				host:   "ndt-oma396982-2248791f.foo.sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
		},
	}
	weheSite = site{
		distance: 3710.7679340078703,
		registration: v2.Registration{
			City:          "Portland",
			CountryCode:   "US",
			ContinentCode: "NA",
			Experiment:    "wehe",
			Latitude:      45.5886,
			Longitude:     -122.5975,
			Metro:         "pdx",
			Project:       "mlab-sandbox",
			Probability:   1.0,
			Site:          "pdx00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      map[string][]string{"wehe/replay": {"wss://4443/v0/envelope/access"}},
		},
		machines: []machine{
			{
				name:   "mlab1-pdx00.mlab-sandbox.measurement-lab.org",
				host:   "wehe-mlab1-pdx00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
		},
	}

	// Test Targets.
	virtualTarget = v2.Target{
		Machine:  "mlab1-lga00.mlab-sandbox.measurement-lab.org",
		Hostname: "ndt-mlab1-lga00.mlab-sandbox.measurement-lab.org",
		Location: &v2.Location{
			City:    "New York",
			Country: "US",
		},
		URLs: map[string]string{},
	}
	physicalTarget = v2.Target{
		Machine:  "mlab1-lax00.mlab-sandbox.measurement-lab.org",
		Hostname: "ndt-mlab1-lax00.mlab-sandbox.measurement-lab.org",
		Location: &v2.Location{
			City:    "Los Angeles",
			Country: "US",
		},
		URLs: map[string]string{},
	}
	weheTarget = v2.Target{
		Machine:  "mlab1-pdx00.mlab-sandbox.measurement-lab.org",
		Hostname: "wehe-mlab1-pdx00.mlab-sandbox.measurement-lab.org",
		Location: &v2.Location{
			City:    "Portland",
			Country: "US",
		},
		URLs: map[string]string{},
	}
)

func TestNearest(t *testing.T) {
	instances := []v2.HeartbeatMessage{
		virtualInstance1,
		physicalInstance,
		weheInstance,
	}

	tests := []struct {
		name      string
		service   string
		lat       float64
		lon       float64
		instances []v2.HeartbeatMessage
		opts      *NearestOptions
		expected  *TargetInfo
		wantErr   bool
	}{
		{
			// Test client coordinates are in NY, virtual target in LGA, and physical target in LAX.
			name:    "NDT7-any-type",
			service: "ndt/ndt7",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "", Country: "US"},
			expected: &TargetInfo{
				Targets: []v2.Target{virtualTarget, physicalTarget},
				URLs:    NDT7Urls,
				Ranks:   map[string]int{virtualTarget.Machine: 0, physicalTarget.Machine: 1},
			},
			wantErr: false,
		},
		{
			name:    "NDT7-physical",
			service: "ndt/ndt7",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "physical", Country: "US"},
			expected: &TargetInfo{
				Targets: []v2.Target{physicalTarget},
				URLs:    NDT7Urls,
				Ranks:   map[string]int{physicalTarget.Machine: 0},
			},
			wantErr: false,
		},
		{
			name:    "NDT7-virtual",
			service: "ndt/ndt7",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "virtual", Country: "US"},
			expected: &TargetInfo{
				Targets: []v2.Target{virtualTarget},
				URLs:    NDT7Urls,
				Ranks:   map[string]int{virtualTarget.Machine: 0},
			},
			wantErr: false,
		},
		{
			name:    "wehe",
			service: "wehe/replay",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "", Country: "US"},
			expected: &TargetInfo{
				Targets: []v2.Target{weheTarget},
				URLs: []url.URL{{
					Scheme: "wss",
					Host:   "4443",
					Path:   "/v0/envelope/access",
				}},
				Ranks: map[string]int{weheTarget.Machine: 0},
			},
			wantErr: false,
		},
		{
			// Test client coordinates are in NY, virtual target in LGA, and physical target in LAX.
			name:    "NDT-sites-found",
			service: "ndt/ndt7",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "", Country: "US", Sites: []string{"lga00", "lax00"}},
			expected: &TargetInfo{
				Targets: []v2.Target{virtualTarget, physicalTarget},
				URLs:    NDT7Urls,
				Ranks:   map[string]int{virtualTarget.Machine: 0, physicalTarget.Machine: 1},
			},
			wantErr: false,
		},
		{
			name:     "NDT-sites-empty",
			service:  "ndt/ndt7",
			lat:      43.1988,
			lon:      -75.3242,
			opts:     &NearestOptions{Type: "", Country: "US", Sites: []string{"foo99", "bar99"}},
			expected: nil,
			wantErr:  true,
		},
		{
			name:    "NDT7-any-type-country",
			service: "ndt/ndt7",
			lat:     43.1988,
			lon:     -75.3242,
			opts:    &NearestOptions{Type: "", Country: "IT"},
			expected: &TargetInfo{
				Targets: []v2.Target{virtualTarget, physicalTarget},
				URLs:    NDT7Urls,
				Ranks:   map[string]int{virtualTarget.Machine: 0, physicalTarget.Machine: 1},
			},
			wantErr: false,
		},
		{
			name:     "NDT7-any-type-country-strict",
			service:  "ndt/ndt7",
			lat:      43.1988,
			lon:      -75.3242,
			opts:     &NearestOptions{Type: "", Country: "IT", Strict: true},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memorystore := heartbeattest.FakeMemorystoreClient
			tracker := NewHeartbeatStatusTracker(&memorystore)
			locator := NewServerLocator(tracker)
			locator.StopImport()
			seedRandom(t, 1658458451000000000)

			for _, i := range instances {
				locator.RegisterInstance(*i.Registration)
				locator.UpdateHealth(i.Registration.Hostname, *i.Health)
			}

			got, err := locator.Nearest(tt.service, tt.lat, tt.lon, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Nearest() error got: %t, want %t, err: %v", err != nil, tt.wantErr, err)
			}

			// Sort targets by Machine name for deterministic comparison
			// (target selection order depends on random number generation
			// which varies across Go versions).
			if got != nil {
				sort.Slice(got.Targets, func(i, j int) bool {
					return got.Targets[i].Machine < got.Targets[j].Machine
				})
			}
			if tt.expected != nil {
				sort.Slice(tt.expected.Targets, func(i, j int) bool {
					return tt.expected.Targets[i].Machine < tt.expected.Targets[j].Machine
				})
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("Nearest() targets got: %+v, want %+v", got, tt.expected)
			}
		})
	}

}

func TestFilterSites(t *testing.T) {
	instances := map[string]v2.HeartbeatMessage{
		"virtual1": virtualInstance1,
		"virtual2": virtualInstance2,
		"physical": physicalInstance,
		"autonode": autonodeInstance,
		"wehe":     weheInstance,
	}

	tests := []struct {
		name     string
		service  string
		typ      string
		country  string
		strict   bool
		org      string
		lat      float64
		lon      float64
		expected []site
	}{
		{
			name:     "NDT7-any-type",
			service:  "ndt/ndt7",
			typ:      "",
			country:  "US",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{virtualSite, autonodeSite, physicalSite},
		},
		{
			name:     "NDT7-physical",
			service:  "ndt/ndt7",
			typ:      "physical",
			country:  "US",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{physicalSite},
		},
		{
			name:     "NDT7-virtual",
			service:  "ndt/ndt7",
			typ:      "virtual",
			country:  "US",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{virtualSite, autonodeSite},
		},
		{
			name:     "wehe",
			service:  "wehe/replay",
			typ:      "",
			country:  "US",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{weheSite},
		},
		{
			name:     "too-far",
			service:  "ndt-ndt7",
			typ:      "",
			country:  "",
			lat:      1000,
			lon:      1000,
			expected: []site{},
		},
		{
			name:     "country-with-strict",
			service:  "ndt/ndt7",
			typ:      "",
			country:  "US",
			strict:   true,
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{virtualSite, autonodeSite, physicalSite},
		},
		{
			name:     "country-with-strict-no-results",
			service:  "ndt/ndt7",
			typ:      "",
			country:  "IT",
			strict:   true,
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{},
		},
		{
			name:     "org-skip-v2-names",
			service:  "ndt/ndt7",
			org:      "foo",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{autonodeSite},
		},
		{
			name:     "org-skip-v3-names-different-org",
			service:  "ndt/ndt7",
			org:      "zoom",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{},
		},
		{
			name:     "org-allow-v2-names-for-mlab-org",
			service:  "ndt/ndt7",
			org:      "mlab",
			lat:      43.1988,
			lon:      -75.3242,
			expected: []site{virtualSite, physicalSite},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &NearestOptions{Type: tt.typ, Country: tt.country, Strict: tt.strict, Org: tt.org}
			got := filterSites(tt.service, tt.lat, tt.lon, instances, opts)

			sortSites(got)
			for _, v := range got {
				sort.Slice(v.machines, func(i, j int) bool {
					return v.machines[i].name < v.machines[j].name
				})
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("filterSites()\n got: %+v\nwant: %+v", got, tt.expected)
			}
		})
	}
}

func TestFilterSitesProbabilityOverride(t *testing.T) {
	// The autonode reports Probability 1.0; derive its machine name the same
	// way filterSites does so the override key is exact.
	autonodeName, err := host.Parse(autonodeInstance.Registration.Hostname)
	if err != nil {
		t.Fatalf("failed to parse autonode hostname: %v", err)
	}

	instances := map[string]v2.HeartbeatMessage{
		"autonode": autonodeInstance,
	}
	opts := &NearestOptions{Type: "virtual", Country: "US"}

	// Without an override the autonode site is returned (Probability 1.0).
	if got := filterSites("ndt/ndt7", 43.1988, -75.3242, instances, opts); len(got) != 1 {
		t.Fatalf("filterSites() without override got %d sites, want 1", len(got))
	}

	// With an override to 0, the autonode site is always filtered out.
	ProbabilityOverrides = map[string]float64{autonodeName.String(): 0}
	defer func() { ProbabilityOverrides = map[string]float64{} }()

	for i := 0; i < 100; i++ {
		if got := filterSites("ndt/ndt7", 43.1988, -75.3242, instances, opts); len(got) != 0 {
			t.Fatalf("filterSites() with probability override 0 got %d sites, want 0", len(got))
		}
	}
}

func TestIsValidInstance(t *testing.T) {
	validHost := "ndt-mlab1-lga00.mlab-sandbox.measurement-lab.org"
	validLat := 40.7667
	validLon := -73.8667
	validType := "virtual"
	validScore := float64(1)

	tests := []struct {
		name         string
		typ          string
		host         string
		lat          float64
		lon          float64
		instanceType string
		services     map[string][]string
		score        float64
		prom         *v2.Prometheus
		expected     bool
		expectedHost host.Name
		expectedDist float64
	}{
		{
			name:         "0-health",
			typ:          "virtual",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: validType,
			score:        0,
			expected:     false,
			expectedHost: host.Name{},
			expectedDist: 0,
		},
		{
			name:         "prometheus-unhealthy",
			typ:          "",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: validType,
			score:        validScore,
			prom: &v2.Prometheus{
				Health: false,
			},
			expected:     false,
			expectedHost: host.Name{},
			expectedDist: 0,
		},
		{
			name:         "invalid-host",
			typ:          "virtual",
			host:         "invalid-host",
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: validType,
			score:        validScore,
			expected:     false,
			expectedHost: host.Name{},
			expectedDist: 0,
		},
		{
			name:         "mismatched-type",
			typ:          "virtual",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: "physical",
			score:        validScore,
			expected:     false,
			expectedHost: host.Name{},
			expectedDist: 0,
		},
		{
			name:         "invalid-service",
			typ:          "virtual",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     map[string][]string{},
			instanceType: validType,
			score:        validScore,
			expected:     false,
			expectedHost: host.Name{},
			expectedDist: 0,
		},
		{
			name:         "success-same-type",
			typ:          "virtual",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: validType,
			score:        validScore,
			expected:     true,
			expectedHost: host.Name{
				Org:     "mlab",
				Service: "ndt",
				Machine: "mlab1",
				Site:    "lga00",
				Project: "mlab-sandbox",
				Domain:  "measurement-lab.org",
				Suffix:  "",
				Version: "v2",
			},
			expectedDist: 296.043665,
		},
		{
			name:         "success-no-type",
			typ:          "",
			host:         validHost,
			lat:          validLat,
			lon:          validLon,
			services:     validNDT7Services,
			instanceType: validType,
			score:        validScore,
			expected:     true,
			expectedHost: host.Name{
				Org:     "mlab",
				Service: "ndt",
				Machine: "mlab1",
				Site:    "lga00",
				Project: "mlab-sandbox",
				Domain:  "measurement-lab.org",
				Suffix:  "",
				Version: "v2",
			},
			expectedDist: 296.043665,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := v2.HeartbeatMessage{
				Registration: &v2.Registration{
					City:          "New York",
					CountryCode:   "US",
					ContinentCode: "NA",
					Experiment:    "ndt",
					Hostname:      tt.host,
					Latitude:      tt.lat,
					Longitude:     tt.lon,
					Machine:       "mlab1",
					Metro:         "lga",
					Project:       "mlab-sandbox",
					Probability:   1.0,
					Site:          "lga00",
					Type:          tt.instanceType,
					Uplink:        "10g",
					Services:      tt.services,
				},
				Health: &v2.Health{
					Score: tt.score,
				},
				Prometheus: tt.prom,
			}
			opts := &NearestOptions{Type: tt.typ}
			got, gotHost, gotDist := isValidInstance("ndt/ndt7", 43.1988, -75.3242, v, opts)

			if got != tt.expected {
				t.Errorf("isValidInstance() got: %t, want: %t", got, tt.expected)
			}

			if gotHost != tt.expectedHost {
				t.Errorf("isValidInstance() host got: %#v, want: %#v", gotHost, tt.expectedHost)
			}

			if math.Abs(gotDist-tt.expectedDist) > 0.01 {
				t.Errorf("isValidInstance() distance got: %f, want: %f", gotDist, tt.expectedDist)
			}
		})
	}
}

func TestSortSites(t *testing.T) {
	tests := []struct {
		name     string
		sites    []site
		expected []site
	}{
		{
			name:     "empty",
			sites:    []site{},
			expected: []site{},
		},
		{
			name:     "one",
			sites:    []site{{distance: 10}},
			expected: []site{{distance: 10}},
		},
		{
			name: "many",
			sites: []site{{distance: 3838.61}, {distance: 3710.7679340078703}, {distance: -895420.92},
				{distance: 296.0436}, {distance: math.MaxFloat64}, {distance: 3838.61}},
			expected: []site{{distance: -895420.92}, {distance: 296.0436}, {distance: 3710.7679340078703},
				{distance: 3838.61}, {distance: 3838.61}, {distance: math.MaxFloat64}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortSites(tt.sites)

			if !reflect.DeepEqual(tt.sites, tt.expected) {
				t.Errorf("sortSites() got: %+v, want: %+v", tt.sites, tt.expected)
			}
		})
	}
}

func TestRankSites(t *testing.T) {
	tests := []struct {
		name     string
		sites    []site
		expected []site
	}{
		{
			name:     "empty",
			sites:    []site{},
			expected: []site{},
		},
		{
			name:     "one",
			sites:    []site{{distance: 10}},
			expected: []site{{distance: 10, rank: 0, metroRank: 0}},
		},
		{
			name: "many",
			sites: []site{
				{registration: v2.Registration{Metro: "a"}},
				{registration: v2.Registration{Metro: "b"}},
				{registration: v2.Registration{Metro: "b"}},
				{registration: v2.Registration{Metro: "c"}},
				{registration: v2.Registration{Metro: "b"}}},
			expected: []site{
				{rank: 0, metroRank: 0, registration: v2.Registration{Metro: "a"}},
				{rank: 1, metroRank: 1, registration: v2.Registration{Metro: "b"}},
				{rank: 2, metroRank: 1, registration: v2.Registration{Metro: "b"}},
				{rank: 3, metroRank: 2, registration: v2.Registration{Metro: "c"}},
				{rank: 4, metroRank: 1, registration: v2.Registration{Metro: "b"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rank(tt.sites)

			if !reflect.DeepEqual(tt.sites, tt.expected) {
				t.Errorf("rankSites() got: %+v, want: %+v", tt.sites, tt.expected)
			}
		})
	}
}

func TestPickTargets(t *testing.T) {
	// Sites numbered by distance, which makes it easier to understand expected values.
	site1 := site{
		distance: 10,
		registration: v2.Registration{
			City:        "New York",
			CountryCode: "US",
			Services:    validNDT7Services,
			Metro:       "lga",
		},
		metroRank: 0,
		machines: []machine{
			{name: "mlab1-site1-metro0", host: "ndt-mlab1-site1-metro0"},
			{name: "mlab2-site1-metro0", host: "ndt-mlab2-site1-metro0"},
			{name: "mlab3-site1-metro0", host: "ndt-mlab3-site1-metro0"},
			{name: "mlab4-site-metro10", host: "ndt-mlab4-site-metro10"},
		},
	}
	site2 := site{
		distance: 10,
		registration: v2.Registration{
			City:        "New York",
			CountryCode: "US",
			Services:    validNDT7Services,
			Metro:       "lga",
		},
		metroRank: 0,
		machines: []machine{
			{name: "mlab1-site2-metro0", host: "ndt-mlab1-site2-metro0"},
			{name: "mlab2-site2-metro0", host: "ndt-mlab2-site2-metro0"},
			{name: "mlab3-site2-metro0", host: "ndt-mlab3-site2-metro0"},
			{name: "mlab4-site2-metro0", host: "ndt-mlab4-site2-metro0"},
		},
	}
	site3 := site{
		distance: 100,
		registration: v2.Registration{
			City:        "Los Angeles",
			CountryCode: "US",
			Services:    validNDT7Services,
			Metro:       "lax",
		},
		metroRank: 1,
		machines: []machine{
			{name: "mlab1-site3-metro1", host: "ndt-mlab1-site3-metro1"},
		},
	}
	site4 := site{
		distance: 110,
		registration: v2.Registration{
			City:        "Portland",
			CountryCode: "US",
			Services:    validNDT7Services,
			Metro:       "pdx",
		},
		metroRank: 2,
		machines: []machine{
			{name: "mlab1-site4-metro2", host: "ndt-mlab1-site4-metro2"},
		},
	}

	// Build a lookup table of valid machines per site for validation.
	allSites := []site{site1, site2, site3, site4}
	machineToSite := make(map[string]*site)
	for i := range allSites {
		for _, m := range allSites[i].machines {
			machineToSite[m.name] = &allSites[i]
		}
	}

	tests := []struct {
		name          string
		sites         []site
		weighted      bool
		expectedCount int
		expectedURLs  []url.URL
	}{
		{
			name:          "4-sites",
			sites:         []site{site1, site2, site3, site4},
			expectedCount: 4,
			expectedURLs:  NDT7Urls,
		},
		{
			name:          "1-site",
			sites:         []site{site1},
			expectedCount: 1,
			expectedURLs:  NDT7Urls,
		},
		{
			name:          "4-sites-weighted",
			sites:         []site{site1, site2, site3, site4},
			weighted:      true,
			expectedCount: 4,
			expectedURLs:  NDT7Urls,
		},
		{
			name:          "1-site-weighted",
			sites:         []site{site1},
			weighted:      true,
			expectedCount: 1,
			expectedURLs:  NDT7Urls,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNearestConfig(t, NearestSettings{Weighted: tt.weighted, EQRadiusKm: 100, MaxDistanceRatio: 20})
			got := pickTargets("ndt/ndt7", tt.sites, &NearestOptions{})

			// Verify the correct number of targets.
			if len(got.Targets) != tt.expectedCount {
				t.Errorf("pickTargets() returned %d targets, want %d", len(got.Targets), tt.expectedCount)
			}

			// Verify URLs are correct.
			if !reflect.DeepEqual(got.URLs, tt.expectedURLs) {
				t.Errorf("pickTargets() URLs = %+v, want %+v", got.URLs, tt.expectedURLs)
			}

			// Verify each target is valid and from one of the input sites.
			seenMachines := make(map[string]bool)
			for _, target := range got.Targets {
				// Check for duplicates.
				if seenMachines[target.Machine] {
					t.Errorf("pickTargets() returned duplicate machine: %s", target.Machine)
				}
				seenMachines[target.Machine] = true

				// Check machine is from one of the input sites.
				sourceSite, ok := machineToSite[target.Machine]
				if !ok {
					t.Errorf("pickTargets() returned unknown machine: %s", target.Machine)
					continue
				}

				// Verify Location matches the site's registration.
				if target.Location.City != sourceSite.registration.City {
					t.Errorf("pickTargets() target %s has City=%s, want %s",
						target.Machine, target.Location.City, sourceSite.registration.City)
				}
				if target.Location.Country != sourceSite.registration.CountryCode {
					t.Errorf("pickTargets() target %s has Country=%s, want %s",
						target.Machine, target.Location.Country, sourceSite.registration.CountryCode)
				}

				// Verify Ranks map contains the machine with correct metro rank.
				rank, ok := got.Ranks[target.Machine]
				if !ok {
					t.Errorf("pickTargets() Ranks missing machine: %s", target.Machine)
				} else if rank != sourceSite.metroRank {
					t.Errorf("pickTargets() Ranks[%s] = %d, want %d",
						target.Machine, rank, sourceSite.metroRank)
				}
			}

			// Verify Ranks map has exactly the right number of entries.
			if len(got.Ranks) != len(got.Targets) {
				t.Errorf("pickTargets() Ranks has %d entries, want %d",
					len(got.Ranks), len(got.Targets))
			}
		})
	}
}

func TestAlwaysPick(t *testing.T) {
	tests := []struct {
		name string
		opts *NearestOptions
		want bool
	}{
		{
			name: "virtual-machines",
			opts: &NearestOptions{
				Type: "virtual",
			},
			want: true,
		},
		{
			name: "sites",
			opts: &NearestOptions{
				Sites: []string{"foo"},
			},
			want: true,
		},
		{
			name: "none",
			opts: &NearestOptions{
				Type: "physical",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alwaysPick(tt.opts)
			if got != tt.want {
				t.Errorf("alwaysPick() got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestPickWithProbability(t *testing.T) {
	// Test deterministic edge cases only.
	// probability=1.0 should always return true.
	for i := 0; i < 10; i++ {
		if got := pickWithProbability(1.0); !got {
			t.Errorf("pickWithProbability(1.0) = false, want true")
		}
	}

	// probability=0.0 should always return false.
	for i := 0; i < 10; i++ {
		if got := pickWithProbability(0.0); got {
			t.Errorf("pickWithProbability(0.0) = true, want false")
		}
	}
}

func TestWeight(t *testing.T) {
	setNearestConfig(t, NearestSettings{Weighted: true, EQRadiusKm: 100})
	opts := &NearestOptions{}

	// Distances below the equivalence radius weigh the same as the radius.
	near := site{distance: 10}
	eq := site{distance: 100}
	if got, want := weight(&near, opts), weight(&eq, opts); got != want {
		t.Errorf("weight(distance=10) = %v, want %v (same as distance=100)", got, want)
	}

	// Doubling the distance beyond the radius quarters the weight.
	far := site{distance: 200}
	if got, want := weight(&far, opts), weight(&eq, opts)/4; math.Abs(got-want) > 1e-15 {
		t.Errorf("weight(distance=200) = %v, want %v (1/4 of distance=100)", got, want)
	}
}

func TestPickSiteIndexWeightedDistribution(t *testing.T) {
	setNearestConfig(t, NearestSettings{Weighted: true, EQRadiusKm: 100})
	seedRandom(t, 1)

	sites := []site{
		{distance: 50},
		{distance: 100},
		{distance: 200},
		{distance: 1000},
	}
	opts := &NearestOptions{}

	// Expected share of each site is weight_i / sum(weights).
	total := 0.0
	for i := range sites {
		total += weight(&sites[i], opts)
	}

	const draws = 10000
	counts := make([]int, len(sites))
	for i := 0; i < draws; i++ {
		counts[pickSiteIndexWeighted(sites, opts)]++
	}

	for i := range sites {
		got := float64(counts[i]) / draws
		want := weight(&sites[i], opts) / total
		if math.Abs(got-want) > 0.02 {
			t.Errorf("site %d empirical share = %v, want %v +/- 0.02", i, got, want)
		}
	}
}

func TestTruncateByDistance(t *testing.T) {
	tests := []struct {
		name  string
		ratio float64
		sites []site
		want  int
	}{
		{
			name:  "cutoff-drops-far-sites",
			ratio: 20,
			// Effective distances: 100, 100, 150, 200, 1900, 2100; cutoff is
			// 20*100=2000, so the last site is dropped.
			sites: []site{{distance: 50}, {distance: 100}, {distance: 150}, {distance: 200}, {distance: 1900}, {distance: 2100}},
			want:  5,
		},
		{
			name:  "keeps-at-least-four",
			ratio: 2,
			// All but the first exceed the 200 km cutoff, but the 4 nearest
			// are always kept.
			sites: []site{{distance: 100}, {distance: 5000}, {distance: 6000}, {distance: 7000}, {distance: 8000}},
			want:  4,
		},
		{
			name:  "zero-disables",
			ratio: 0,
			sites: []site{{distance: 100}, {distance: 20000}},
			want:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setNearestConfig(t, NearestSettings{Weighted: true, EQRadiusKm: 100, MaxDistanceRatio: tt.ratio})
			got := truncateByDistance(tt.sites)
			if len(got) != tt.want {
				t.Errorf("truncateByDistance() kept %d sites, want %d", len(got), tt.want)
			}
		})
	}
}

func TestPickTargetsAlgorithmRouting(t *testing.T) {
	sites := func() []site {
		return []site{{
			distance:     10,
			registration: v2.Registration{Services: validNDT7Services},
			machines:     []machine{{name: "mlab1-abc01"}},
		}}
	}

	// The legacy path draws the site index from the exponential seam; the
	// weighted path does not use it.
	expCalled := false
	seedRandom(t, 1)
	randExpDistributedInt = func(rate float64) int {
		expCalled = true
		return 0
	}

	setNearestConfig(t, NearestSettings{Weighted: false, EQRadiusKm: 100})
	pickTargets("ndt/ndt7", sites(), &NearestOptions{})
	if !expCalled {
		t.Errorf("pickTargets() with Weighted=false did not use the exponential-rank draw")
	}

	expCalled = false
	setNearestConfig(t, NearestSettings{Weighted: true, EQRadiusKm: 100})
	pickTargets("ndt/ndt7", sites(), &NearestOptions{})
	if expCalled {
		t.Errorf("pickTargets() with Weighted=true used the exponential-rank draw")
	}
}
