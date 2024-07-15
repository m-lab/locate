package memorystore

import (
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/go-test/deep"
	"github.com/gomodule/redigo/redis"
	"github.com/m-lab/go/testingx"
	v2 "github.com/m-lab/locate/api/v2"
	"github.com/m-lab/locate/connection/testdata"
	"github.com/rafaeljusto/redigomock"
)

func setUpTest[V any]() (*redigomock.Conn, *client[V]) {
	conn := redigomock.NewConn()
	pool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return conn, nil
		},
	}
	c := NewClient[V](&pool)
	return conn, c
}

func TestPut_MarshalError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("HSET")
	r := *testdata.FakeRegistration.Registration
	r.Latitude = math.Inf(1)
	opts := &PutOptions{FieldMustExist: "", WithExpire: true}
	err := client.Put(testdata.FakeHostname, "Registration", &r, opts)

	if conn.Stats(hset) > 0 {
		t.Fatal("Put() failure, HSET command should not be called, want: marshal error")
	}

	if err == nil {
		t.Error("Put() error: nil, want: marshal error")
	}
}

func TestPut_HSETError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("HSET").ExpectError(errors.New("HSET error"))
	opts := &PutOptions{FieldMustExist: "", WithExpire: true}
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration, opts)

	if conn.Stats(hset) != 1 {
		t.Fatal("Put() failure, HSET command should have been called")
	}

	if err == nil {
		t.Error("Put() error: nil, want: HSET error")
	}
}

func TestPut_EVALError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("EVAL").ExpectError(errors.New("EVAL error"))
	opts := &PutOptions{FieldMustExist: "Registration", WithExpire: true}
	err := client.Put(testdata.FakeHostname, "Health", testdata.FakeHealth.Health, opts)

	if conn.Stats(hset) != 1 {
		t.Fatal("Put() failure, EVAL command should have been called")
	}

	if err == nil {
		t.Error("Put() error: nil, want: EVAL error")
	}
}

func TestPut_EXPIREError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("HSET").Expect(1)
	expire := conn.GenericCommand("EXPIRE").ExpectError(errors.New("EXPIRE error"))
	opts := &PutOptions{FieldMustExist: "", WithExpire: true}
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration, opts)

	if conn.Stats(hset) != 1 || conn.Stats(expire) != 1 {
		t.Fatal("Put() failure, HSET and EXPIRE commands should have been called")
	}

	if err == nil {
		t.Error("Put() error: nil, want: EXPIRE error")
	}
}

func TestPut_Success(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("HSET").Expect(1)
	opts := &PutOptions{FieldMustExist: "", WithExpire: false}
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration, opts)

	if conn.Stats(hset) != 1 {
		t.Fatal("Put() failure, HSET command should have been called")
	}

	if err != nil {
		t.Errorf("Put() error: %+v, want: nil", err)
	}
}

func TestPut_SuccessWithEXISTS(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("EVAL").Expect(1)
	opts := &PutOptions{FieldMustExist: "Registration", WithExpire: false}
	err := client.Put(testdata.FakeHostname, "Health", testdata.FakeHealth.Health, opts)

	if conn.Stats(hset) != 1 {
		t.Fatal("Put() failure, EVAL command should have been called")
	}

	if err != nil {
		t.Errorf("Put() error: %+v, want: nil", err)
	}
}

func TestPut_SuccessWithEXPIRE(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hset := conn.GenericCommand("HSET").Expect(1)
	expire := conn.GenericCommand("EXPIRE").Expect(1)
	opts := &PutOptions{FieldMustExist: "", WithExpire: true}
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration, opts)

	if conn.Stats(hset) != 1 || conn.Stats(expire) != 1 {
		t.Fatal("Put() failure, HSET and EXPIRE commands should have been called")
	}

	if err != nil {
		t.Errorf("Put() error: %+v, want: nil", err)
	}
}

func TestGetAll_SCANError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()
	scan := conn.GenericCommand("SCAN").ExpectError(errors.New("SCAN error"))

	_, err := client.GetAll()

	if conn.Stats(scan) != 1 {
		t.Fatal("GetAll() failure, SCAN should have been called")
	}

	if err == nil {
		t.Error("GetAll() error: nil, want: SCAN error")
	}
}

func TestGetAll_ScanLibraryError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	// Only returning one argument will cause the `redis.Scan()`
	// call to fail with a "redigo.Scan: array short" error.
	scan := conn.Command("SCAN", 0).Expect([]interface{}{
		int64(10),
	})

	_, err := client.GetAll()

	if conn.Stats(scan) != 1 {
		t.Fatal("GetAll() failure, SCAN should have been called")
	}

	if err == nil {
		t.Error("GetAll() error: nil, want: redigo.Scan error")
	}
}

func TestGetAll_GetError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	scan := conn.Command("SCAN", 0).Expect([]interface{}{
		int64(10), []interface{}{testdata.FakeHostname},
	})

	// This will return an error in the inner get() call.
	hgetall := conn.GenericCommand("HGETALL").ExpectError(errors.New("HGETALL error"))

	_, err := client.GetAll()

	if conn.Stats(scan) != 1 || conn.Stats(hgetall) != 1 {
		t.Fatal("GetAll() failure, SCAN and HGETALL should have been called")
	}

	if err == nil {
		t.Error("GetAll() error: nil, want: get error")
	}
}

func TestGetAll_Success(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	scan := conn.Command("SCAN", 0).Expect([]interface{}{
		int64(10), []interface{}{testdata.FakeHostname},
	})
	scan2 := conn.Command("SCAN", 10).Expect([]interface{}{
		int64(0), nil,
	})

	hbm := v2.HeartbeatMessage{}
	hbm.Registration = testdata.FakeRegistration.Registration
	hbm.Health = testdata.FakeHealth.Health
	rBytes, err := json.Marshal(hbm.Registration)
	testingx.Must(t, err, "failed to marshal registration")
	hBytes, err := json.Marshal(hbm.Health)
	testingx.Must(t, err, "failed to marshal health")
	hgetall := conn.Command("HGETALL", testdata.FakeHostname).Expect([]interface{}{
		[]byte("Registration"), rBytes, []byte("Health"), hBytes,
	})

	got, err := client.GetAll()

	if conn.Stats(scan) != 1 || conn.Stats(scan2) != 1 || conn.Stats(hgetall) != 1 {
		t.Fatal("GetAll() failure, SCAN and HGETALL should have been called")
	}

	if err != nil {
		t.Fatalf("GetAll() error: %+v, want: nil", err)
	}

	want := map[string]v2.HeartbeatMessage{testdata.FakeHostname: hbm}
	if diff := deep.Equal(got, want); diff != nil {
		t.Errorf("GetAll() incorrect output; got: %+v, want: %+v", got, want)
	}
}

func TestGet_HGETALLError(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	hgetall := conn.GenericCommand("HGETALL").ExpectError(errors.New("HGETALL error"))
	_, err := client.get("", conn)

	if conn.Stats(hgetall) != 1 {
		t.Fatal("get() failure, HGETALL should have been called")
	}

	if err == nil {
		t.Error("get() error: nil, want: HGETALL error")
	}
}

func TestGet_ScanStructError(t *testing.T) {
	// ScanStruct fails when it does not know how to scan a field
	// that doesn't implement the Scanner interface.
	conn, client := setUpTest[v2.MonitoringResult]()

	hgetall := conn.GenericCommand("HGETALL").Expect([]interface{}{
		[]byte("Error"), &v2.Error{},
	})

	_, err := client.get("foo", conn)

	if conn.Stats(hgetall) != 1 {
		t.Fatal("get() failure, HGETALL should have been called")
	}

	if err == nil {
		t.Error("get() error: nil, want: ScanStruct error")
	}
}

func TestDel_Success(t *testing.T) {
	conn, client := setUpTest[v2.HeartbeatMessage]()

	delCmd := conn.Command("DEL", testdata.FakeHostname).Expect(1)
	err := client.Del(testdata.FakeHostname)

	if conn.Stats(delCmd) != 1 {
		t.Fatal("Del() failure, DEL should have been called")
	}

	if err != nil {
		t.Errorf("Del() error:  %+v, want: nil", err)
	}
}
