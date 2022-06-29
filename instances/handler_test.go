package instances

import (
	"errors"
	"testing"

	"github.com/go-test/deep"
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/m-lab/locate/instances/instancestest"
)

var (
	fakeDC    = &instancestest.FakeDatastoreClient{}
	fakeErrDC = &instancestest.FakeErrorDatastoreClient{}
)

func TestRegisterInstance_InvalidArgument(t *testing.T) {
	h := NewCachingInstanceHandler(fakeDC)
	defer h.StopImport()

	err := h.RegisterInstance(v2.HeartbeatMessage{})

	if !errors.Is(err, errInvalidArgument) {
		t.Errorf("RegisterInstance() error: %v, want: %v", err, errInvalidArgument)
	}
}

func TestRegisterInstance_PutError(t *testing.T) {
	h := NewCachingInstanceHandler(fakeErrDC)
	defer h.StopImport()

	hbm := testdata.FakeRegistration
	err := h.RegisterInstance(hbm)

	if !errors.Is(err, instancestest.FakeError) {
		t.Errorf("RegisterInstance() error: %v, want: %v", err, instancestest.FakeError)
	}
}

func TestRegisterInstance_Success(t *testing.T) {
	h := NewCachingInstanceHandler(fakeDC)
	defer h.StopImport()

	hbm := testdata.FakeRegistration
	err := h.RegisterInstance(hbm)

	if err != nil {
		t.Errorf("RegisterInstance() error: %v, want: nil", err)
	}

	if diff := deep.Equal(h.instances[hbm.Registration.Hostname], &hbm); diff != nil {
		t.Errorf("RegisterInstance() failed to register; got: %+v. want: %+v",
			h.instances[hbm.Registration.Hostname], &hbm)
	}
}

func TestUpdateHealth_UpdateError(t *testing.T) {
	h := NewCachingInstanceHandler(fakeErrDC)
	defer h.StopImport()

	hm := testdata.FakeHealth.Health
	err := h.UpdateHealth(testdata.FakeHostname, *hm)

	if !errors.Is(err, instancestest.FakeError) {
		t.Errorf("UpdateHealth() error: %v, want: %v", err, instancestest.FakeError)
	}
}

func TestUpdateHealth_LocalError(t *testing.T) {
	h := NewCachingInstanceHandler(fakeDC)
	defer h.StopImport()

	hm := testdata.FakeHealth.Health
	err := h.UpdateHealth(testdata.FakeHostname, *hm)

	if err == nil {
		t.Error("UpdateHealth() error: nil, want: !nil")
	}
}

func TestUpdateHealth_Success(t *testing.T) {
	h := NewCachingInstanceHandler(fakeDC)
	defer h.StopImport()

	err := h.RegisterInstance(testdata.FakeRegistration)
	testingx.Must(t, err, "failed to register instance")

	hm := testdata.FakeHealth.Health
	err = h.UpdateHealth(testdata.FakeHostname, *hm)

	if err != nil {
		t.Errorf("UpdateHealth() error: %v, want: !nil", err)
	}

	if diff := deep.Equal(h.instances[testdata.FakeHostname].Health, hm); diff != nil {
		t.Errorf("UpdateHealth() failed to update health; got: %+v, want: %+v",
			h.instances[testdata.FakeHostname].Health, hm)
	}
}

func TestImportDatastore(t *testing.T) {
	fdc := &instancestest.FakeDatastoreClient{}
	h := NewCachingInstanceHandler(fdc)
	defer h.StopImport()

	fdc.FakeAdd(testdata.FakeHostname, &testdata.FakeRegistration)
	h.importDatastore()

	expected := map[string]*v2.HeartbeatMessage{testdata.FakeHostname: &testdata.FakeRegistration}
	if diff := deep.Equal(h.instances, expected); diff != nil {
		t.Errorf("importDatastore() failed to import; got: %v, want: %+v", h.instances,
			expected)
	}
}
