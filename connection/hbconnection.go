package connection

import (
	"encoding/json"
	"net/http"

	v2 "github.com/m-lab/locate/api/v2"
)

// HeartbeatConn contains a base connection through which
// it sends heartbeat messages.
type HeartbeatConn struct {
	BaseConn *Conn
}

// NewHeartbeatConn creates a new HeartbeatConn with default
// values for its base connection.
func NewHeartbeatConn() *HeartbeatConn {
	return &HeartbeatConn{
		BaseConn: NewConn(),
	}
}

// Dial encodes the registration mesage into JSON and creates
// a new client connection through which to send the message.
func (hbc *HeartbeatConn) Dial(address string, header http.Header, dialMsg v2.Registration) error {
	b, err := json.Marshal(dialMsg)
	if err != nil {
		return err
	}
	return hbc.BaseConn.Dial(address, header, b)
}

// WriteMessage encodes the heartbeat message into JSON and
// forwards it to the base connection to write it.
func (hbc *HeartbeatConn) WriteMessage(messageType int, msg v2.Health) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return hbc.BaseConn.WriteMessage(messageType, b)
}

// Close closes the network connection.
func (hbc *HeartbeatConn) Close() error {
	return hbc.BaseConn.Close()
}
