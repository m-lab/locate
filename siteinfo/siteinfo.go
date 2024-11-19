package siteinfo

import (
	"fmt"
	"net/url"

	"github.com/m-lab/go/host"
	v2 "github.com/m-lab/locate/api/v2"
)

func Machines(msgs map[string]v2.HeartbeatMessage, v url.Values) (map[string]v2.HeartbeatMessage, error) {
	machines := make(map[string]v2.HeartbeatMessage)

	org := v.Get("org")
	exp := v.Get("exp")

	if org != "" || exp != "" {
		for k, v := range msgs {
			parts, err := host.Parse(k)
			if err != nil {
				returnError := fmt.Errorf("failed to parse hostname: %s", k)
				return nil, returnError
			}
			if org != "" && exp == "" {
				if org == parts.Org {
					machines[k] = v
				}
			} else if org == "" && exp != "" {
				if exp == parts.Service {
					machines[k] = v
				}
			} else {
				if org == parts.Org && exp == parts.Service {
					machines[k] = v
				}
			}
		}
	} else {
		machines = msgs
	}

	return machines, nil

}
