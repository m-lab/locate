package instances

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

func setUpTest() (*redigomock.Conn, *redisDatastoreClient) {
	conn := redigomock.NewConn()
	pool := redis.Pool{
		Dial: func() (redis.Conn, error) {
			return conn, nil
		},
	}
	c := NewRedisDatastoreClient(&pool)
	return conn, c
}

func TestPut_MarshalError(t *testing.T) {
	conn, client := setUpTest()

	hset := conn.GenericCommand("HSET")

	err := client.Put(testdata.FakeHostname, "Registration", math.Inf(1))

	if conn.Stats(hset) > 0 {
		t.Fatal("Put() failure, HSET command should not be called, want: marshal error")
	}

	if err == nil {
		t.Error("Put() error: nil, want: marshal error")
	}
}

func TestPut_HSETError(t *testing.T) {
	conn, client := setUpTest()

	hset := conn.GenericCommand("HSET").ExpectError(errors.New("HSET error"))
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration)

	if conn.Stats(hset) != 1 {
		t.Fatal("Put() failure, HSET command should have been called")
	}

	if err == nil {
		t.Error("Put() error: nil, want: HSET error")
	}
}

func TestPut_EXPIREError(t *testing.T) {
	conn, client := setUpTest()

	hset := conn.GenericCommand("HSET").Expect(1)
	expire := conn.GenericCommand("EXPIRE").ExpectError(errors.New("EXPIRE error"))
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration)

	if conn.Stats(hset) != 1 || conn.Stats(expire) != 1 {
		t.Fatal("Put() failure, HSET and EXPIRE commands should have been called")
	}

	if err == nil {
		t.Error("Put() error: nil, want: EXPIRE error")
	}
}

func TestPut_Success(t *testing.T) {
	conn, client := setUpTest()

	hset := conn.GenericCommand("HSET").Expect(1)
	expire := conn.GenericCommand("EXPIRE").Expect(1)
	err := client.Put(testdata.FakeHostname, "Registration", testdata.FakeRegistration.Registration)

	if conn.Stats(hset) != 1 || conn.Stats(expire) != 1 {
		t.Fatal("Put() failure, HSET and EXPIRE commands should have been called")
	}

	if err != nil {
		t.Errorf("Put() error: %+v, want: nil", err)
	}
}

func TestUpdate_NonExistentKey(t *testing.T) {
	conn, client := setUpTest()

	// The redis.Bool helper casts its input into int64.
	exists := conn.GenericCommand("EXISTS").Expect(int64(0))
	err := client.Update(testdata.FakeHostname, "Health", testdata.FakeHealth.Health)

	if conn.Stats(exists) != 1 {
		t.Fatal("Update() failure, EXISTS should have been called")
	}

	if !errors.Is(err, errKeyNotFound) {
		t.Errorf("Update() error: %+v, want: %+v", err, errKeyNotFound)
	}
}

func TestUpdate_EXISTSError(t *testing.T) {
	conn, client := setUpTest()

	exists := conn.GenericCommand("EXISTS").ExpectError(errors.New("EXISTS error"))
	err := client.Update(testdata.FakeHostname, "Health", testdata.FakeHealth.Health)

	if conn.Stats(exists) != 1 {
		t.Fatal("Update() failure, EXISTS should have been called")
	}

	if !errors.Is(err, errKeyNotFound) {
		t.Errorf("Update() error: %+v, want: %+v", err, errKeyNotFound)
	}
}

func TestUpdate_Sucess(t *testing.T) {
	conn, client := setUpTest()

	hset := conn.GenericCommand("HSET").Expect(1)
	expire := conn.GenericCommand("EXPIRE").Expect(1)
	exists := conn.GenericCommand("EXISTS").Expect(int64(1))
	err := client.Update(testdata.FakeHostname, "Health", testdata.FakeHealth.Health)

	if conn.Stats(hset) != 1 || conn.Stats(expire) != 1 || conn.Stats(exists) != 1 {
		t.Fatal("Update() failure, HSET, EXPIRE, and EXISTS commands should have been called")
	}

	if err != nil {
		t.Errorf("Update() error: %+v, want: nil", err)
	}
}

func TestGetAllHeartbeats_SCANError(t *testing.T) {
	conn, client := setUpTest()
	scan := conn.GenericCommand("SCAN").ExpectError(errors.New("SCAN error"))

	_, err := client.GetAllHeartbeats()

	if conn.Stats(scan) != 1 {
		t.Fatal("GetAllHeartbeats() failure, SCAN should have been called")
	}

	if err == nil {
		t.Error("GetAllHeartbeats() error: nil, want: SCAN error")
	}
}

func TestGetAllHeartbeats_Success(t *testing.T) {
	conn, client := setUpTest()

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

	got, err := client.GetAllHeartbeats()

	if conn.Stats(scan) != 1 || conn.Stats(scan2) != 1 || conn.Stats(hgetall) != 1 {
		t.Fatal("GetAllHeartbeats() failure, SCAN and HGETALL should have been called")
	}

	if err != nil {
		t.Fatalf("GetAllHeartbeats() error: %+v, want: nil", err)
	}

	want := map[string]*v2.HeartbeatMessage{testdata.FakeHostname: &hbm}
	if diff := deep.Equal(got, want); diff != nil {
		t.Errorf("GetAllHeartbeats() incorrect output; got: %+v, want: %+v", got, want)
	}
}

func TestGetHeartbeat_HGETALLError(t *testing.T) {
	conn, _ := setUpTest()

	hgetall := conn.GenericCommand("HGETALL").ExpectError(errors.New("HGETALL error"))
	got, err := getHeartbeat("foo", conn)

	if conn.Stats(hgetall) != 1 {
		t.Fatal("getHeartbeat() failure, HGETALL should have been called")
	}

	if err == nil {
		t.Error("getHeartbeat() error: nil, want: HGETALL error")
	}

	if got != nil {
		t.Errorf("getHeartbeat() incorrect output; got %+v, want nil", got)
	}
}
