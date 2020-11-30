package clientgeo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestUserLocator_Locate(t *testing.T) {
	tests := []struct {
		name    string
		vals    url.Values
		want    *Location
		wantErr bool
	}{
		{
			name: "success-user-latlon",
			want: &Location{
				Latitude:  "12",
				Longitude: "34",
				Headers: http.Header{
					"X-Locate-Clientlatlon":        []string{"12,34"},
					"X-Locate-Clientlatlon-Method": []string{"user-latlon"},
				},
			},
			vals: url.Values{
				"lat": []string{"12"},
				"lon": []string{"34"},
			},
		},
		{
			name: "success-user-region",
			want: &Location{
				Latitude:  "43.19880000",
				Longitude: "-75.3242000",
				Headers: http.Header{
					"X-Locate-Clientlatlon":        []string{"43.19880000,-75.3242000"},
					"X-Locate-Clientlatlon-Method": []string{"user-region"},
				},
			},
			vals: url.Values{
				"region": []string{"US-NY"},
			},
		},
		{
			name: "success-user-country",
			want: &Location{
				Latitude:  "37.09024",
				Longitude: "-95.712891",
				Headers: http.Header{
					"X-Locate-Clientlatlon":        []string{"37.09024,-95.712891"},
					"X-Locate-Clientlatlon-Method": []string{"user-country"},
				},
			},
			vals: url.Values{
				"country": []string{"US"},
			},
		},
		{
			name: "error-unusable-latitude-parameters",
			vals: url.Values{
				"lat": []string{"xyz.000"},
				"lon": []string{"34"},
			},
			wantErr: true,
		},
		{
			name: "error-unusable-longitude-parameters",
			vals: url.Values{
				"lat": []string{"12"},
				"lon": []string{"xyz.000"},
			},
			wantErr: true,
		},
		{
			name: "error-unusable-nan",
			vals: url.Values{
				"lat": []string{"NaN"},
				"lon": []string{"NaN"},
			},
			wantErr: true,
		},
		{
			name: "error-unusable-inf",
			vals: url.Values{
				"lat": []string{"Inf"},
				"lon": []string{"Inf"},
			},
			wantErr: true,
		},
		{
			name: "error-unusable-lat-too-big",
			vals: url.Values{
				"lat": []string{"91"},
				"lon": []string{"0"},
			},
			wantErr: true,
		},
		{
			name: "error-unusable-lon-too-big",
			vals: url.Values{
				"lat": []string{"0"},
				"lon": []string{"181"},
			},
			wantErr: true,
		},
		{
			name:    "error-no-parameters",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewUserLocator()
			req := httptest.NewRequest(http.MethodGet, "/v2/nearest", nil)
			req.URL.RawQuery = tt.vals.Encode()
			got, err := u.Locate(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserLocator.Locate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UserLocator.Locate() = %v, want %v", got, tt.want)
			}
			u.Reload(context.Background())
		})
	}
}
