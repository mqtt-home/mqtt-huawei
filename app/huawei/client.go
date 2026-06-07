package huawei

import (
	"reflect"
	"sync"
	"time"

	"github.com/philipparndt/go-logger"
)

// Client polls a Backend for inverter status and notifies listeners on change.
type Client struct {
	backend Backend

	mu     sync.RWMutex
	status InverterStatus

	listeners     []func(InverterStatus)
	listenersLock sync.RWMutex
}

func NewClient(backend Backend) *Client {
	return &Client{
		backend: backend,
		status:  InverterStatus{Source: backend.Name()},
	}
}

func (c *Client) AddStatusChangeListener(listener func(InverterStatus)) {
	c.listenersLock.Lock()
	defer c.listenersLock.Unlock()
	c.listeners = append(c.listeners, listener)
}

// Connect performs an initial fetch so that an early status is available.
func (c *Client) Connect() error {
	return c.poll()
}

func (c *Client) GetStatus() InverterStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// poll fetches a fresh snapshot and notifies listeners when it changed.
func (c *Client) poll() error {
	status, err := c.backend.Fetch()
	if err != nil {
		// Mark disconnected but keep last known values.
		c.mu.Lock()
		changed := c.status.Connected
		c.status.Connected = false
		current := c.status
		c.mu.Unlock()
		if changed {
			c.notify(current)
		}
		return err
	}

	status.Source = c.backend.Name()
	status.Connected = true
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	c.mu.Lock()
	changed := statusChanged(c.status, status)
	c.status = status
	c.mu.Unlock()

	if changed {
		c.notify(status)
	}
	return nil
}

// statusChanged compares two snapshots ignoring the UpdatedAt timestamp.
func statusChanged(a, b InverterStatus) bool {
	a.UpdatedAt = ""
	b.UpdatedAt = ""
	return !reflect.DeepEqual(a, b)
}

func (c *Client) notify(status InverterStatus) {
	c.listenersLock.RLock()
	listeners := make([]func(InverterStatus), len(c.listeners))
	copy(listeners, c.listeners)
	c.listenersLock.RUnlock()
	for _, l := range listeners {
		l(status)
	}
}

func (c *Client) StartPolling(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.poll(); err != nil {
				logger.Error("Failed to poll inverter status", "error", err)
			}
		case <-stopCh:
			return
		}
	}
}

func (c *Client) Close() error {
	return c.backend.Close()
}
