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
	// ErrTooManyReconnects is returned when the number of reconnections
	// has reached MaxReconnectionsTotal within MaxElapsedTime.
	ErrTooManyReconnects = errors.New("websocket cannot reconnect right now (too many attemps)")
	// ErrNotDialed is returned when WriteMessage is called, but
	// the websocket has not been created yet (call Dial).
	ErrNotDailed = errors.New("websocket not created yet, please call Dial()")
	// retryErrors contains the list of errors that may become successful
	// if the request is retried.
	retryErrors = map[int]bool{408: true, 425: true, 500: true, 502: true, 503: true, 504: true}
)

// Conn contains the state needed to connect, reconnect, and send
// messages.
// Default values must be updated before calling `Dial`.
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
	dialMessage          interface{}
	ticker               time.Ticker
	mu                   sync.Mutex
	reconnections        int
	isDialed             bool
	isConnected          bool
	stop                 chan bool
}

// NewConn creates a new Conn with default values.
func NewConn() *Conn {
	c := &Conn{
		InitialInterval:       static.BackoffInitialInterval,
		RandomizationFactor:   static.BackoffRandomizationFactor,
		Multiplier:            static.BackoffMultiplier,
		MaxInterval:           static.BackoffMaxInterval,
		MaxElapsedTime:        static.BackoffMaxElapsedTime,
		MaxReconnectionsTotal: static.MaxReconnectionsTotal,
		MaxReconnectionsTime:  static.MaxReconnectionsTime,
	}
	return c
}

// Dial creates a new persistent client connection and sets
// the necessary state for future reconnections. It also
// starts a goroutine to reset the number of reconnections.
//
// A call to Dial is a prerequisite to writing any messages.
// The function only needs to be called once on start to create the
// connection. Alternatively, if Close is called, Dial will have to
// be called again if the connection needs to be recreated.
//
// The function returns an error if the url is invalid or if
// a 4XX error (except 408 and 425) is received in the HTTP
// response.
func (c *Conn) Dial(address string, header http.Header, dialMsg interface{}) error {
	u, err := url.ParseRequestURI(address)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
		return errors.New("malformed ws or wss URL")
	}
	c.url = *u
	c.dialMessage = dialMsg
	c.stop = make(chan bool)
	c.header = header
	c.dialer = websocket.Dialer{}
	c.isDialed = true
	c.ticker = *time.NewTicker(c.MaxReconnectionsTime)
	go func(c *Conn) {
		for {
			select {
			case <-c.stop:
				c.ticker.Stop()
				return
			case <-c.ticker.C:
				c.resetReconnections()
			}
		}
	}(c)

	return c.connect()
}

// WriteMessage sends a message as a slice of bytes.
// If the write fails or a disconnect has been detected, it will
// close the connection and try to reconnect and resend the
// message.
//
// The write will fail under the following conditions:
//	1. The client has not called Dial (ErrNotDialed).
//	2. The connection is disconnected and it was not able to
//	   reconnect (ErrTooManyReconnects or an internal connection
//	   error).
//	3. The write call in the websocket package failed
//	   (gorilla/websocket error).
func (c *Conn) WriteMessage(messageType int, data interface{}) error {
	if !c.isDialed {
		return ErrNotDailed
	}

	// If a disconnect has already been detected, try to reconnect.
	if !c.IsConnected() {
		if err := c.closeAndReconnect(); err != nil {
			return err
		}
	}

	// If the write fails, reconnect and send the message again.
	if err := c.write(messageType, data); err != nil {
		if err := c.closeAndReconnect(); err != nil {
			return err
		}
		return c.write(messageType, data)
	}
	return nil
}

// IsConnected returns the WebSocket connection state.
func (c *Conn) IsConnected() bool {
	return c.isConnected
}

// CanConnect checks whether it is possible to reconnect
// given the recent number of attempts.
func (c *Conn) CanConnect() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reconnections < c.MaxReconnectionsTotal
}

// Close closes the network connection and cleans up private
// resources after the connection is done.
func (c *Conn) Close() error {
	if c.isDialed {
		c.isDialed = false
		c.stop <- true
	}
	return c.close()
}

// resetReconnections sets the number of disconnects followed
// by reconnects.
func (c *Conn) resetReconnections() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconnections = 0
}

// closeAndReconnect calls close and reconnect.
func (c *Conn) closeAndReconnect() error {
	err := c.close()
	if err != nil {
		return err
	}
	return c.reconnect()
}

// close closes the underlying network connection without
// sending or waiting for a close frame.
func (c *Conn) close() error {
	if c.IsConnected() {
		c.isConnected = false
		if c.ws != nil {
			return c.ws.Close()
		}
	}
	return nil
}

// reconnect updates the number of reconnections and
// re-establishes the connection.
func (c *Conn) reconnect() error {
	if !c.CanConnect() {
		return ErrTooManyReconnects
	}
	c.mu.Lock()
	c.reconnections++
	c.mu.Unlock()
	return c.connect()
}

// connect creates a new client connection and sends the
// registration message.
// In case of failure, it uses an exponential backoff to
// increase the duration of retry attempts.
func (c *Conn) connect() error {
	b := c.getBackoff()
	ticker := backoff.NewTicker(b)

	var ws *websocket.Conn
	var resp *http.Response
	var err error
	for range ticker.C {
		ws, resp, err = c.dialer.Dial(c.url.String(), c.header)
		if err != nil {
			if resp != nil && !retryErrors[resp.StatusCode] {
				log.Printf("error trying to establish a connection with %s, err: %v, status: %d",
					c.url.String(), err, resp.StatusCode)
				ticker.Stop()
			}
			log.Printf("could not establish a connection with %s (will retry), err: %v",
				c.url.String(), err)
			continue
		}

		c.ws = ws
		c.isConnected = true
		log.Printf("successfully established a connection with %s", c.url.String())
		ticker.Stop()
	}

	if c.isConnected {
		err = c.write(websocket.TextMessage, c.dialMessage)
	}
	return err
}

// write is a helper function that gets a writer using NextWriter,
// writes the message and closes the writer.
// It returns an error if the calls to NextWriter or WriteMessage
// return errors.
func (c *Conn) write(messageType int, data interface{}) error {
	// We want to identify and return write errors as soon as they occur.
	// The supported interface for WriteMessage does not do that.
	// Therefore, we are using NextWriter explicitly with Close
	// to update the error.
	// NextWriter is called with a PingMessage type because it is
	// effectively a no-op, while using other message types can
	// cause side-effects (e.g, loading an empty msg to the buffer).
	w, err := c.ws.NextWriter(websocket.PingMessage)
	if err == nil {
		err = c.ws.WriteJSON(data)
		w.Close()
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
