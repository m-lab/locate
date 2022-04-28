// Package connection provides a Websocket that will automatically
// reconnect if the connection is dropped.
package connection

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/static"
)

// ErrNotConnected is returned when the application tries to
// read/write a message and the connection is closed.
var ErrNotConnected = errors.New("websocket not connected")

// Conn contains state needed to connect, reconnect, and send
// messages.
type Conn struct {
	InitialInterval      time.Duration
	RandomizationFactor  float64
	Multiplier           float64
	MaxInterval          time.Duration
	MaxElapsedTime       time.Duration
	MaxReconnectionsTime time.Duration
	Factor               float64
	Dialer               websocket.Dialer
	ws                   *websocket.Conn
	url                  string
	header               http.Header
	ticker               time.Ticker
	mu                   sync.Mutex
	reconnections        int
	isConnected          bool
}

// NewConn creates a new Conn with default values and starts
// a new goroutine to reset the number of disconnects followed
// by reconnects per hour.
func NewConn() *Conn {
	c := &Conn{
		InitialInterval:      static.BackoffInitialInterval,
		RandomizationFactor:  static.BackoffRandomizationFactor,
		Multiplier:           static.BackoffMultiplier,
		MaxInterval:          static.BackoffMaxInterval,
		MaxElapsedTime:       static.BackoffMaxElapsedTime,
		MaxReconnectionsTime: static.MaxReconnectionsTime,
		Dialer:               websocket.Dialer{},
	}
	return c
}

// Dial creates a new persistent client connection and sets
// the necessary state for future reconnections. It also
// starts a goroutine to reset the number of reconnections.
func (c *Conn) Dial(url string, header http.Header) error {
	c.url = url
	c.header = header

	c.ticker = *time.NewTicker(c.MaxReconnectionsTime)
	go func(c *Conn) {
		defer c.ticker.Stop()
		for {
			<-c.ticker.C
			c.mu.Lock()
			c.resetReconnections()
			c.mu.Unlock()
		}
	}(c)

	return c.connect()
}

// WriteMessage is a helper method for getting a writer using
// NextWriter, writing the message and closing the writer.
// If the write fails or a disconnect has been detected, it will
// close and reconnect.
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	if c.IsConnected() {
		err := c.ws.WriteMessage(messageType, data)
		if err != nil {
			c.closeAndReconnect()
		}
		return err
	}
	if c.canConnect() {
		c.closeAndReconnect()
	}
	return ErrNotConnected
}

// Close closes the underlying network connection without
// sending or waiting for a close frame.
func (c *Conn) Close() {
	c.isConnected = false
	if c.ws != nil {
		c.ws.Close()
	}
}

// IsConnected returns the WebSocket connection state.
func (c *Conn) IsConnected() bool {
	return c.isConnected
}

// resetReconnections sets the number of disconnects followed
// by reconnects.
func (c *Conn) resetReconnections() {
	c.reconnections = 0
}

// canConnect checks whether it is possible to reconnect
// given the recent number of attempts.
func (c *Conn) canConnect() bool {
	return c.reconnections < static.MaxReconnectionsTotal
}

// closeAndReconnect calls Close and reconnect.
func (c *Conn) closeAndReconnect() {
	c.Close()
	c.reconnect()
}

// reconnect updates the number of reconnections and
// re-establishes the connection.
func (c *Conn) reconnect() {
	if c.canConnect() {
		c.mu.Lock()
		c.reconnections++
		c.mu.Unlock()
		c.connect()
	}
}

// connect creates a new client connection. In case of failure,
// it uses an exponential backoff to increase the duration of
// retry attempts.
func (c *Conn) connect() error {
	b := c.getBackoff()
	ticker := backoff.NewTicker(b)

	var err error
	var ws *websocket.Conn
	for range ticker.C {
		ws, _, err = c.Dialer.Dial(c.url, c.header)
		if err != nil {
			log.Printf("could not establish a connection with %s (will retry), err: %v",
				c.url, err)
		} else {
			c.ws = ws
			c.isConnected = true
			ticker.Stop()
			log.Printf("successfully established a connection with %s", c.url)
			return nil
		}
	}
	return err
}

// getBackoff returns a backoff implementation that increases the
// backoff period for each retry attempt using a randomization function
// that grows exponentially.
func (c *Conn) getBackoff() *backoff.ExponentialBackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = c.InitialInterval
	b.RandomizationFactor = c.RandomizationFactor
	b.Multiplier = c.Multiplier
	b.MaxInterval = c.MaxInterval
	b.MaxElapsedTime = c.MaxElapsedTime
	return b
}
