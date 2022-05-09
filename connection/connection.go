// Package connection provides a Websocket that will automatically
// reconnect if the connection is dropped.
package connection

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/gorilla/websocket"
	"github.com/m-lab/locate/static"
)

var (
	// ErrNotConnected is returned when the application tries to
	// read/write a message and the connection is closed.
	ErrNotConnected = errors.New("websocket not connected")
	// ErrCannotReconnect is returned when the number of reconnections
	// has reached MaxReconnectionsTotal within MaxElapsedTime.
	ErrCannotReconnect = errors.New("websocket cannot reconnect right now (too many attemps)")
	// ErrNotConnected is returned when WriteMessage is called, but
	// the websocket has not been created yet.
	ErrNotDailed = errors.New("websocket not created yet, please call Dial()")
	// retryClientErrors contains the list of client (4XX) errors that may
	// become successful if the request is retried.
	retryClientErrors = map[int]bool{404: true, 408: true, 425: true}
)

// Conn contains the state needed to connect, reconnect, and send
// messages.
type Conn struct {
	// InitialInterval is the first interval at which the backoff starts
	// running.
	InitialInterval time.Duration
	// RandomizationFactor is used to create the range of values:
	// [currentInterval - randomizationFactor * currentInterval,
	// currentInterval + randomizationFactor * currentInterval] and picking
	// a random value from the range.
	RandomizationFactor float64
	// Multiplier is used to increment the backoff interval by multiplying it.
	Multiplier float64
	// MaxInterval is an interval such that, once reached, the backoff will
	// retry with a constant delay of MaxInterval.
	MaxInterval time.Duration
	// MaxElapsedTime is the amount of time after which the ExponentialBackOff
	// returns Stop. It never stops if MaxElapsedTime == 0.
	MaxElapsedTime time.Duration
	// MaxReconnectionsTotal is the maximum number of reconnections that can
	// happen within MaxReconnectionsTime.
	MaxReconnectionsTotal int
	// MaxReconnectionsTime is the time period during which the number of
	// reconnections must be less than MaxReconnectionsTotal to allow a
	// reconnection atttempt.
	MaxReconnectionsTime time.Duration
	dialer               websocket.Dialer
	ws                   *websocket.Conn
	url                  url.URL
	header               http.Header
	ticker               time.Ticker
	mu                   sync.Mutex
	reconnections        int
	dialed               bool
	isConnected          bool
}

// NewConn creates a new Conn with default values and starts
// a new goroutine to reset the number of disconnects followed
// by reconnects per hour.
func NewConn() *Conn {
	c := &Conn{
		InitialInterval:       static.BackoffInitialInterval,
		RandomizationFactor:   static.BackoffRandomizationFactor,
		Multiplier:            static.BackoffMultiplier,
		MaxInterval:           static.BackoffMaxInterval,
		MaxElapsedTime:        static.BackoffMaxElapsedTime,
		MaxReconnectionsTotal: static.MaxReconnectionsTotal,
		MaxReconnectionsTime:  static.MaxReconnectionsTime,
		dialer:                websocket.Dialer{},
	}
	return c
}

// Dial creates a new persistent client connection and sets
// the necessary state for future reconnections. It also
// starts a goroutine to reset the number of reconnections.
// Dail only needs to be called once at start to create the connection.
// Alternatively, if Close is called, Dial will have to be called
// again if the connection needs to be recreated.
// It returns an error if the url is invalid.
func (c *Conn) Dial(address string, header http.Header) error {
	u, err := url.ParseRequestURI(address)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return errors.New("malformed ws or wss URL")
	}
	c.url = *u

	c.header = header
	c.dialed = true
	c.ticker = *time.NewTicker(c.MaxReconnectionsTime)
	go func(c *Conn) {
		for {
			<-c.ticker.C
			c.resetReconnections()
		}
	}(c)

	return c.connect()
}

// WriteMessage is a helper method for getting a writer using
// NextWriter, writing the message and closing the writer.
// If the write fails or a disconnect has been detected, it will
// close and reconnect.
func (c *Conn) WriteMessage(messageType int, data []byte) error {
	if !c.dialed {
		return ErrNotDailed
	}
	if !c.IsConnected() {
		if !c.canConnect() {
			return ErrCannotReconnect
		}
		// If not connected, try to reconnect.
		// If unsuccessful, return an ErrNorConnected error.
		c.closeAndReconnect()
		if !c.IsConnected() {
			return ErrNotConnected
		}
	}
	// It is the call to NextWriter that detects the connection
	// has been closed by reading the last update to writeErr:
	// https://github.com/gorilla/websocket/blob/78cf1bc733a927f673fd1988a25256b425552a8a/conn.go#L489.
	// Calling WriteMessage by itself has a delay of one call to detect a disconnect.
	w, err := c.ws.NextWriter(messageType)
	if err == nil {
		err = c.ws.WriteMessage(messageType, data)
		w.Close()
	}
	if err != nil {
		c.closeAndReconnect()
	}
	return err
}

// IsConnected returns the WebSocket connection state.
func (c *Conn) IsConnected() bool {
	return c.isConnected
}

// Close stops the reconnection ticker and closes the
// underlying network connection.
func (c *Conn) Close() error {
	c.dialed = false
	c.ticker.Stop()
	return c.close()
}

// resetReconnections sets the number of disconnects followed
// by reconnects.
func (c *Conn) resetReconnections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnections = 0
}

// canConnect checks whether it is possible to reconnect
// given the recent number of attempts.
func (c *Conn) canConnect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reconnections < c.MaxReconnectionsTotal
}

// closeAndReconnect calls close and reconnect.
func (c *Conn) closeAndReconnect() {
	c.close()
	c.reconnect()
}

// close closes the underlying network connection without
// sending or waiting for a close frame.
func (c *Conn) close() error {
	c.isConnected = false
	if c.ws != nil {
		return c.ws.Close()
	}
	return nil
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

	var ws *websocket.Conn
	var resp *http.Response
	var err error
	for range ticker.C {
		ws, resp, err = c.dialer.Dial(c.url.String(), c.header)
		if err != nil {
			// Check for client errors indicating we should not retry.
			if resp != nil {
				_, retry := retryClientErrors[resp.StatusCode]
				if resp.StatusCode >= 400 && resp.StatusCode <= 451 && !retry {
					log.Printf("client error trying to establish a connection with %s, err: %v",
						c.url.String(), err)
					return err
				}
			}

			log.Printf("could not establish a connection with %s (will retry), err: %v",
				c.url.String(), err)
			continue
		}

		c.ws = ws
		c.isConnected = true
		ticker.Stop()
		log.Printf("successfully established a connection with %s", c.url.String())
		return nil
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
