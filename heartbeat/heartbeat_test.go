package heartbeat

import (
	"errors"
	"runtime"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/heartbeat/heartbeattest"
)

var (
	fakeDC    = &heartbeattest.FakeDatastoreClient
	fakeErrDC = &heartbeattest.FakeErrorDatastoreClient
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

func TestStopImport(t *testing.T) {
	h := NewHeartbeatStatusTracker(fakeDC)
	before := runtime.NumGoroutine()

	h.StopImport()
	after := runtime.NumGoroutine()
	if after != before-1 {
		t.Errorf("StopImport() failed to stop import goroutine; got %d, want %d",
			after, before-1)
	}
}

func TestImportDatastore(t *testing.T) {
	fdc := &heartbeattest.FakeDatastoreClient
	h := NewHeartbeatStatusTracker(fdc)
	defer h.StopImport()

	fdc.FakeAdd(testdata.FakeHostname, testdata.FakeRegistration)
	h.importDatastore()

	expected := map[string]v2.HeartbeatMessage{testdata.FakeHostname: testdata.FakeRegistration}
	if diff := deep.Equal(h.instances, expected); diff != nil {
		t.Errorf("importDatastore() failed to import; got: %+v, want: %+v", h.instances,
			expected)
	}
}
