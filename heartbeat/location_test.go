package heartbeat

import (
	"math"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"github.com/m-lab/go/host"
	"github.com/m-lab/go/mathx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
)

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
			Site:          "lax00",
			Type:          "physical",
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
			Site:          "lga00",
			Type:          "virtual",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		machines: []machine{
			{
				name:   "mlab1-lga00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
			{
				name:   "mlab2-lga00.mlab-sandbox.measurement-lab.org",
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
			Site:          "lax00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      validNDT7Services,
		},
		machines: []machine{
			{
				name:   "mlab1-lax00.mlab-sandbox.measurement-lab.org",
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
			Site:          "pdx00",
			Type:          "physical",
			Uplink:        "10g",
			Services:      map[string][]string{"wehe/replay": {"wss://4443/v0/envelope/access"}},
		},
		machines: []machine{
			{
				name:   "mlab1-pdx00.mlab-sandbox.measurement-lab.org",
				health: v2.Health{Score: 1},
			},
		},
	}

	// Test Targets.
	virtualTarget = v2.Target{
		Machine: "mlab1-lga00.mlab-sandbox.measurement-lab.org",
		Location: &v2.Location{
			City:    "New York",
			Country: "US",
		},
		URLs: map[string]string{},
	}
	physicalTarget = v2.Target{
		Machine: "mlab1-lax00.mlab-sandbox.measurement-lab.org",
		Location: &v2.Location{
			City:    "Los Angeles",
			Country: "US",
		},
		URLs: map[string]string{},
	}
	weheTarget = v2.Target{
		Machine: "mlab1-pdx00.mlab-sandbox.measurement-lab.org",
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
		name            string
		service         string
		typ             string
		country         string
		lat             float64
		lon             float64
		instances       []v2.HeartbeatMessage
		expectedTargets []v2.Target
		expectedURLs    []url.URL
		wantErr         bool
	}{
		{
			name:            "NDT7-any-type",
			service:         "ndt/ndt7",
			typ:             "",
			country:         "US",
			lat:             43.1988,
			lon:             -75.3242,
			expectedTargets: []v2.Target{virtualTarget, physicalTarget},
			expectedURLs:    NDT7Urls,
			wantErr:         false,
		},
		{
			name:            "NDT7-physical",
			service:         "ndt/ndt7",
			typ:             "physical",
			country:         "US",
			lat:             43.1988,
			lon:             -75.3242,
			expectedTargets: []v2.Target{physicalTarget},
			expectedURLs:    NDT7Urls,
			wantErr:         false,
		},
		{
			name:            "NDT7-virtual",
			service:         "ndt/ndt7",
			typ:             "virtual",
			country:         "US",
			lat:             43.1988,
			lon:             -75.3242,
			expectedTargets: []v2.Target{virtualTarget},
			expectedURLs:    NDT7Urls,
			wantErr:         false,
		},
		{
			name:            "wehe",
			service:         "wehe/replay",
			typ:             "",
			country:         "US",
			lat:             43.1988,
			lon:             -75.3242,
			expectedTargets: []v2.Target{weheTarget},
			expectedURLs: []url.URL{{
				Scheme: "wss",
				Host:   "4443",
				Path:   "/v0/envelope/access",
			}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memorystore := heartbeattest.FakeMemorystoreClient
			tracker := NewHeartbeatStatusTracker(&memorystore)
			locator := NewServerLocator(tracker)
			locator.StopImport()
			rand = mathx.NewRandom(1658458451000000000)

			for _, i := range instances {
				locator.RegisterInstance(*i.Registration)
				locator.UpdateHealth(i.Registration.Hostname, *i.Health)
			}

			gotTargets, gotURLs, err := locator.Nearest(tt.service, tt.typ, "", tt.lat, tt.lon)

			if !reflect.DeepEqual(gotTargets, tt.expectedTargets) {
				t.Errorf("Nearest() targets got: %+v, want %+v", gotTargets, tt.expectedTargets)
			}

			if !reflect.DeepEqual(gotURLs, tt.expectedURLs) {
				t.Errorf("Nearest() URLs got: %+v, want %+v", gotURLs, tt.expectedURLs)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("Nearest() error got: %t, want %t", err != nil, tt.wantErr)
			}
		})
	}

}

func TestFilterSites(t *testing.T) {
	instances := map[string]v2.HeartbeatMessage{
		"virtual1": virtualInstance1,
		"virtual2": virtualInstance2,
		"physical": physicalInstance,
		"wehe":     weheInstance,
	}

	tests := []struct {
		name     string
		service  string
		typ      string
		country  string
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
			expected: []site{virtualSite, physicalSite},
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
			expected: []site{virtualSite},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSites(tt.service, tt.typ, tt.country, tt.lat, tt.lon, instances)

			sortSites(got)
			for _, v := range got {
				sort.Slice(v.machines, func(i, j int) bool {
					return v.machines[i].name < v.machines[j].name
				})
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("filterSites() got: %+v, want: %+v", got, tt.expected)
			}
		})
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
				Machine: "mlab1",
				Site:    "lga00",
				Project: "mlab-sandbox",
				Domain:  "measurement-lab.org",
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
				Machine: "mlab1",
				Site:    "lga00",
				Project: "mlab-sandbox",
				Domain:  "measurement-lab.org",
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
			got, gotHost, gotDist := isValidInstance("ndt/ndt7", tt.typ, 43.1988, -75.3242, v)

			if got != tt.expected {
				t.Errorf("isValidInstance() got: %t, want: %t", got, tt.expected)
			}

			if gotHost != tt.expectedHost {
				t.Errorf("isValidInstance() host got: %+v, want: %+v", gotHost, tt.expectedHost)
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

func TestPickTargets(t *testing.T) {
	// Sites numbered by distance, which makes it easier to understand expected values.
	site1 := site{
		distance: 10,
		registration: v2.Registration{
			City:        "New York",
			CountryCode: "US",
			Services:    validNDT7Services,
		},
		machines: []machine{{name: "mlab1-site1"}, {name: "mlab2-site1"}, {name: "mlab3-site1"}, {name: "mlab4-site1"}},
	}
	site2 := site{
		distance: 10,
		registration: v2.Registration{
			City:        "New York",
			CountryCode: "US",
			Services:    validNDT7Services,
		},
		machines: []machine{{name: "mlab1-site2"}, {name: "mlab2-site2"}, {name: "mlab3-site2"}, {name: "mlab4-site2"}},
	}
	site3 := site{
		distance: 100,
		registration: v2.Registration{
			City:        "Los Angeles",
			CountryCode: "US",
			Services:    validNDT7Services,
		},
		machines: []machine{{name: "mlab1-site3"}},
	}
	site4 := site{
		distance: 110,
		registration: v2.Registration{
			City:        "Portland",
			CountryCode: "US",
			Services:    validNDT7Services,
		},
		machines: []machine{{name: "mlab1-site4"}},
	}

	tests := []struct {
		name         string
		sites        []site
		expected     []v2.Target
		expectedURLs []url.URL
	}{
		{
			name: "4-sites",
			sites: []site{
				site1, site2, site3, site4,
			},
			expected: []v2.Target{
				{
					Machine: "mlab2-site1",
					Location: &v2.Location{
						City:    site1.registration.City,
						Country: site1.registration.CountryCode,
					},
					URLs: make(map[string]string),
				},
				{
					Machine: "mlab1-site3",
					Location: &v2.Location{
						City:    site3.registration.City,
						Country: site3.registration.CountryCode,
					},
					URLs: make(map[string]string),
				},
				{
					Machine: "mlab4-site2",
					Location: &v2.Location{
						City:    site2.registration.City,
						Country: site2.registration.CountryCode,
					},
					URLs: make(map[string]string),
				},
				{
					Machine: "mlab1-site4",
					Location: &v2.Location{
						City:    site4.registration.City,
						Country: site4.registration.CountryCode,
					},
					URLs: make(map[string]string),
				},
			},
			expectedURLs: NDT7Urls,
		},
		{
			name: "1-site",
			sites: []site{
				site1,
			},
			expected: []v2.Target{
				{
					Machine: "mlab2-site1",
					Location: &v2.Location{
						City:    site1.registration.City,
						Country: site1.registration.CountryCode,
					},
					URLs: make(map[string]string),
				},
			},
			expectedURLs: NDT7Urls,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a fixed seed so the pattern is only pseudorandom and can
			// be verififed against expectations.
			rand = mathx.NewRandom(1658340109320624212)
			got, gotURLs := pickTargets("ndt/ndt7", tt.sites)

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("pickTargets() got: %+v, want: %+v", got, tt.expected)
			}

			if !reflect.DeepEqual(gotURLs, tt.expectedURLs) {
				t.Errorf("pickTargets() urls got: %+v, want: %+v", gotURLs, tt.expectedURLs)
			}
		})
	}
}

func TestPickWithProbability(t *testing.T) {
	tests := []struct {
		name string
		site string
		seed int64
		want bool
	}{
		{
			name: "no-probability",
			site: "lga00",
			want: true,
		},
		// The current probability for yyc02 is 0.5.
		// If we use 2 as a seed, the pseudo-random number generated will be < 0.5.
		{
			name: "pick-with-probability",
			site: "yyc02",
			seed: 2,
			want: true,
		},
		// If we use 1 as a seed, the pseudo-random number generated will be > 0.5.
		{
			name: "do-not-pick-with-probability",
			site: "yyc02",
			seed: 1,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rand = mathx.NewRandom(tt.seed)
			got := pickWithProbability(tt.site)

			if got != tt.want {
				t.Errorf("pickWithProbability() got: %v, want: %v", got, tt.want)
			}
		})
	}
}

func TestBiasedDistance(t *testing.T) {
	tests := []struct {
		name     string
		country  string
		r        *v2.Registration
		distance float64
		want     float64
	}{
		{
			name:    "empty-country",
			country: "",
			r: &v2.Registration{
				CountryCode: "foo",
			},
			distance: 100,
			want:     100,
		},
		{
			name:    "unknown-country",
			country: "ZZ",
			r: &v2.Registration{
				CountryCode: "foo",
			},
			distance: 100,
			want:     100,
		},
		{
			name:    "same-country",
			country: "foo",
			r: &v2.Registration{
				CountryCode: "foo",
			},
			distance: 100,
			want:     100,
		},
		{
			name:    "different-country",
			country: "bar",
			r: &v2.Registration{
				CountryCode: "foo",
			},
			distance: 100,
			want:     200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := biasedDistance(tt.country, tt.r, tt.distance)

			if got != tt.want {
				t.Errorf("biasedDistance() got: %f, want: %f", got, tt.want)
			}
		})
	}
}
