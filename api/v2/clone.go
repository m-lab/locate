package v2

func (hbm *HeartbeatMessage) Clone() *HeartbeatMessage {
	result := &HeartbeatMessage{}
	if hbm.Registration != nil {
		result.Registration = hbm.Registration.Clone()
	}
	if hbm.Health != nil {
		result.Health = hbm.Health.Clone()
	}
	return result
}

func (rm *Registration) Clone() *Registration {
	services := make(map[string][]string)
	for k, v := range rm.Services {
		service := make([]string, len(v))
		copy(service, v)
		services[k] = service
	}

	return &Registration{
		City:          rm.City,
		CountryCode:   rm.CountryCode,
		ContinentCode: rm.ContinentCode,
		Experiment:    rm.Experiment,
		Hostname:      rm.Hostname,
		Latitude:      rm.Latitude,
		Longitude:     rm.Longitude,
		Machine:       rm.Machine,
		Metro:         rm.Metro,
		Project:       rm.Project,
		Site:          rm.Site,
		Type:          rm.Type,
		Uplink:        rm.Uplink,
		Services:      services,
	}
}

func (rm *Health) Clone() *Health {
	return &Health{
		Score: rm.Score,
	}
}
