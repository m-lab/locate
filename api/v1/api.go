package v1

// Result is the default structure for mlab-ns requests.
//
// Result is provided for legacy mlab-ns requests. No new services should be
// built using this structure.
type Result struct {
	City    string   `json:"city"`
	Country string   `json:"country"`
	FQDN    string   `json:"fqdn"`
	IP      []string `json:"ip"` // obsolete
	Site    string   `json:"site"`
	URL     string   `json:"url"` // obsolete
}

// Results consist of multiple Result objects, for policy=geo_options requests.
//
// Results is provided for legacy mlab-ns requests. No new services should be
// built using this structure.
type Results []Result
