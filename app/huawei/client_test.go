package huawei

import (
	"errors"
	"testing"
)

func TestStatusChangedIgnoresTimestamp(t *testing.T) {
	a := InverterStatus{PVPower: 100, UpdatedAt: "2026-06-07T10:00:00Z"}
	b := a
	b.UpdatedAt = "2026-06-07T10:00:30Z"
	if statusChanged(a, b) {
		t.Error("status should be unchanged when only UpdatedAt differs")
	}

	c := a
	c.PVPower = 200
	if !statusChanged(a, c) {
		t.Error("status should be changed when PVPower differs")
	}
}

// fakeBackend is a controllable Backend for client tests.
type fakeBackend struct {
	status InverterStatus
	err    error
	calls  int
}

func (f *fakeBackend) Name() string                   { return "fake" }
func (f *fakeBackend) Fetch() (InverterStatus, error) { f.calls++; return f.status, f.err }
func (f *fakeBackend) Close() error                   { return nil }

func TestClientPollNotifiesOnChange(t *testing.T) {
	b := &fakeBackend{status: InverterStatus{PVPower: 1000}}
	c := NewClient(b)

	var got InverterStatus
	var n int
	c.AddStatusChangeListener(func(s InverterStatus) {
		got = s
		n++
	})

	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if n != 1 {
		t.Fatalf("listener calls = %d, want 1", n)
	}
	if !got.Connected || got.Source != "fake" || got.PVPower != 1000 {
		t.Errorf("unexpected status: %+v", got)
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt should be set")
	}

	// Same value -> no new notification.
	if err := c.poll(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if n != 1 {
		t.Errorf("listener calls = %d, want still 1 (no change)", n)
	}

	// Changed value -> notify again.
	b.status.PVPower = 2000
	if err := c.poll(); err != nil {
		t.Fatalf("poll: %v", err)
	}
	if n != 2 || got.PVPower != 2000 {
		t.Errorf("expected notification for change, n=%d pv=%v", n, got.PVPower)
	}
}

func TestClientPollErrorMarksDisconnected(t *testing.T) {
	b := &fakeBackend{status: InverterStatus{PVPower: 1000}}
	c := NewClient(b)
	if err := c.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if !c.GetStatus().Connected {
		t.Fatal("expected connected after first poll")
	}

	b.err = errors.New("boom")
	if err := c.poll(); err == nil {
		t.Fatal("expected error from poll")
	}
	if c.GetStatus().Connected {
		t.Error("status should be marked disconnected after fetch error")
	}
}
