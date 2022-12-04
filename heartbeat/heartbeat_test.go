package heartbeat

import (
	"errors"
	"reflect"
	"runtime"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
)

var (
	fakeDC       = &heartbeattest.FakeMemorystoreClient
	fakeErrDC    = &heartbeattest.FakeErrorMemorystoreClient
	testMachine  = "mlab1-lga00.mlab-sandbox.measurement-lab.org"
	testHostname = "ndt-" + testMachine
)

func TestRegisterInstance_PutError(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeErrDC)
	defer h.StopImport()

	err := h.RegisterInstance(*testdata.FakeRegistration.Registration)

	if !errors.Is(err, heartbeattest.FakeError) {
		t.Errorf("RegisterInstance() error: %+v, want: %+v", err, heartbeattest.FakeError)
	}
}

func TestRegisterInstance_Success(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	defer h.StopImport()

	hbm := testdata.FakeRegistration
	err := h.RegisterInstance(*hbm.Registration)

	if err != nil {
		t.Errorf("RegisterInstance() error: %+v, want: nil", err)
	}

	if diff := deep.Equal(h.instances[hbm.Registration.Hostname], hbm); diff != nil {
		t.Errorf("RegisterInstance() failed to register; got: %+v. want: %+v",
			h.instances[hbm.Registration.Hostname], &hbm)
	}
}

func TestUpdateHealth_UpdateError(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeErrDC)
	defer h.StopImport()

	hm := testdata.FakeHealth.Health
	err := h.UpdateHealth(testdata.FakeHostname, *hm)

	if !errors.Is(err, heartbeattest.FakeError) {
		t.Errorf("UpdateHealth() error: %+v, want: %+v", err, heartbeattest.FakeError)
	}
}

func TestUpdateHealth_LocalError(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	defer h.StopImport()

	hm := testdata.FakeHealth.Health
	err := h.UpdateHealth(testdata.FakeHostname, *hm)

	if err == nil {
		t.Error("UpdateHealth() error: nil, want: !nil")
	}
}

func TestUpdateHealth_Success(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	defer h.StopImport()

	err := h.RegisterInstance(*testdata.FakeRegistration.Registration)
	testingx.Must(t, err, "failed to register instance")

	hm := testdata.FakeHealth.Health
	err = h.UpdateHealth(testdata.FakeHostname, *hm)

	if err != nil {
		t.Errorf("UpdateHealth() error: %+v, want: !nil", err)
	}

	if diff := deep.Equal(h.instances[testdata.FakeHostname].Health, hm); diff != nil {
		t.Errorf("UpdateHealth() failed to update health; got: %+v, want: %+v",
			h.instances[testdata.FakeHostname].Health, hm)
	}
}

func TestUpdatePrometheus_PutError(t *testing.T) {
	h := heartbeatStatusTracker{
		MemorystoreClient: fakeErrDC,
		instances: map[string]v2.HeartbeatMessage{
			testHostname: {
				Registration: &v2.Registration{
					Hostname: testHostname,
				},
			},
		},
	}
	hostnames := map[string]bool{testHostname: true}
	machines := map[string]bool{testMachine: true}

	err := h.UpdatePrometheus(hostnames, machines)

	if !errors.Is(err, errPrometheus) {
		t.Errorf("UpdatePrometheus() err: %v, want: %v", err, errPrometheus)
	}
}

func TestUpdatePrometheus_Success(t *testing.T) {
	h := heartbeatStatusTracker{
		MemorystoreClient: fakeDC,
		instances: map[string]v2.HeartbeatMessage{
			testHostname: {
				Registration: &v2.Registration{
					Hostname: testHostname,
				},
			},
		},
	}
	hostnames := map[string]bool{testHostname: true}
	machines := map[string]bool{testMachine: true}

	err := h.UpdatePrometheus(hostnames, machines)

	if err != nil {
		t.Errorf("UpdatePrometheus() err: %v, want: nil", err)
	}
}

func TestInstances(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	h.StopImport()

	hbm := testdata.FakeRegistration
	h.RegisterInstance(*hbm.Registration)

	instances := h.Instances()
	expected := map[string]v2.HeartbeatMessage{testdata.FakeHostname: testdata.FakeRegistration}
	if diff := deep.Equal(instances, expected); diff != nil {
		t.Errorf("Instances() got: %+v, want: %+v", instances, expected)
	}

}

func TestInstancesCopy(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	h.StopImport()

	// Add a new instance with nil v2.Health.
	hbm := testdata.FakeRegistration
	h.RegisterInstance(*hbm.Registration)

	// Get copy of instances and verify that v2.Health field is nil.
	instances := h.Instances()
	if instances[testdata.FakeHostname].Health != nil {
		t.Errorf("Instances() got: %+v, want: nil", instances[testdata.FakeHostname].Health)
	}

	// Update v2.Health for the instance in the tracker.
	h.UpdateHealth(testdata.FakeHostname, *testdata.FakeHealth.Health)
	instancesWithUpdate := h.Instances()
	if instancesWithUpdate[testdata.FakeHostname].Health == nil {
		t.Errorf("Instances() got: nil, want: %+v", instancesWithUpdate[testdata.FakeHostname].Health)
	}

	// Verify original copy of instances did not get updated.
	if instances[testdata.FakeHostname].Health != nil {
		t.Errorf("Instances() got: %+v, want: nil", instances[testdata.FakeHostname].Health)
	}
}

func TestStopImport(t *testing.T) {
	before := runtime.NumGoroutine()
	h := NewHeartbeatStatusTracker(fakeDC)

	h.StopImport()
	after := runtime.NumGoroutine()
	if after != before {
		t.Errorf("StopImport() failed to stop import goroutine; got %d, want %d",
			after, before)
	}
}

func TestImportMemorystore(t *testing.T) {
	fdc := &heartbeattest.FakeMemorystoreClient
	h := NewHeartbeatStatusTracker(fdc)
	defer h.StopImport()

	fdc.FakeAdd(testdata.FakeHostname, testdata.FakeRegistration)
	h.importMemorystore()

	expected := map[string]v2.HeartbeatMessage{testdata.FakeHostname: testdata.FakeRegistration}
	if diff := deep.Equal(h.instances, expected); diff != nil {
		t.Errorf("importMemorystore() failed to import; got: %+v, want: %+v", h.instances,
			expected)
	}
}

func TestGetPrometheusMessage(t *testing.T) {
	tests := []struct {
		name      string
		hostnames map[string]bool
		machines  map[string]bool
		reg       *v2.Registration
		want      *v2.Prometheus
	}{
		{
			name:      "nil-registration",
			hostnames: map[string]bool{testHostname: true},
			machines:  map[string]bool{testMachine: true},
			reg:       nil,
			want:      nil,
		},
		{
			name:      "both-empty",
			hostnames: map[string]bool{},
			machines:  map[string]bool{},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: nil,
		},
		{
			name:      "only-hostnames",
			hostnames: map[string]bool{testHostname: true},
			machines:  map[string]bool{},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: true},
		},
		{
			name:      "only-machines",
			hostnames: map[string]bool{},
			machines:  map[string]bool{testMachine: true},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: true},
		},
		{
			name:      "both-unhealthy",
			hostnames: map[string]bool{testHostname: false},
			machines:  map[string]bool{testMachine: false},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: false},
		},
		{
			name:      "only-hostname-unhealthy",
			hostnames: map[string]bool{testHostname: false},
			machines:  map[string]bool{testMachine: true},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: false},
		},
		{
			name:      "only-machine-unhealthy",
			hostnames: map[string]bool{testHostname: true},
			machines:  map[string]bool{testMachine: false},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: false},
		},
		{
			name:      "both-healthy",
			hostnames: map[string]bool{testHostname: true},
			machines:  map[string]bool{testMachine: true},
			reg: &v2.Registration{
				Hostname: testHostname,
			},
			want: &v2.Prometheus{Health: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := v2.HeartbeatMessage{Registration: tt.reg}
			pm := constructPrometheusMessage(i, tt.hostnames, tt.machines)

			if !reflect.DeepEqual(pm, tt.want) {
				t.Errorf("getPrometheusMessage() got: %v, want: %v", pm, tt.want)
			}
		})
	}
}

func TestPromNumericStatus(t *testing.T) {
	tests := []struct {
		name string
		pm   *v2.Prometheus
		want float64
	}{
		{
			name: "true",
			pm: &v2.Prometheus{
				Health: true,
			},
			want: 1,
		},
		{
			name: "false",
			pm: &v2.Prometheus{
				Health: false,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promNumericStatus(tt.pm)

			if got != tt.want {
				t.Errorf("promNumericStatus() got: %v, want: %v", got, tt.want)
			}
		})
	}
}
