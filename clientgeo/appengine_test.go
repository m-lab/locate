// Package clientgeo supports interfaces to different data sources to help
// identify client geo location for server selection.
package clientgeo

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestAppEngineLocator_Locate(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name       string
		useHeaders map[string]string
		want       *Location
		wantErr    bool
	}{
		{
			name: "success-using-latlong",
			useHeaders: map[string]string{
				"X-AppEngine-CityLatLong": "40.3,-70.4",
			},
			want: &Location{
				Latitude:  "40.3",
				Longitude: "-70.4",
				Headers: http.Header{
					"X-Locate-Clientlatlon":        []string{"40.3,-70.4"},
					"X-Locate-Clientlatlon-Method": []string{"appengine-latlong"},
				},
			},
		},
		{
			name:       "error-missing-country",
			useHeaders: map[string]string{}, // none.
			want: &Location{
				Headers: http.Header{
					"X-Locate-Clientlatlon-Method": []string{"appengine-none"},
				},
			},
			wantErr: true,
		},
		{
			name: "success-using-region",
			useHeaders: map[string]string{
				"X-AppEngine-Country": "US",
				"X-AppEngine-Region":  "NY",
			},
			want: &Location{
				Latitude:  "43.19880000",
				Longitude: "-75.3242000",
				Headers: http.Header{
					"X-Locate-Clientlatlon-Method": []string{"appengine-region"},
					"X-Locate-Clientlatlon":        []string{"43.19880000,-75.3242000"},
				},
			},
		},
		{
			name: "success-ignore-latlong-use-region",
			useHeaders: map[string]string{
				"X-AppEngine-CityLatLong": "0.000000,0.000000", // some IPs receive a "null" latlon, when region and country are valid.
				"X-AppEngine-Country":     "US",
				"X-AppEngine-Region":      "NY",
			},
			want: &Location{
				Latitude:  "43.19880000",
				Longitude: "-75.3242000",
				Headers: http.Header{
					"X-Locate-Clientlatlon-Method": []string{"appengine-region"},
					"X-Locate-Clientlatlon":        []string{"43.19880000,-75.3242000"},
				},
			},
		},
		{
			name: "success-using-country",
			useHeaders: map[string]string{
				"X-AppEngine-Country": "US",
			},
			want: &Location{
				Latitude:  "37.09024",
				Longitude: "-95.712891",
				Headers: http.Header{
					"X-Locate-Clientlatlon-Method": []string{"appengine-country"},
					"X-Locate-Clientlatlon":        []string{"37.09024,-95.712891"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl := NewAppEngineLocator()
			req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
			for key, value := range tt.useHeaders {
				req.Header.Set(key, value)
			}
			got, err := sl.Locate(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("AppEngineLocator.Locate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AppEngineLocator.Locate() = %v, want %v", got, tt.want)
			}
		})
	}
}
