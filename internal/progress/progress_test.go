package progress

import (
	"testing"
)

func TestSpinner_StopTwice(t *testing.T) {
	// Calling Stop twice should not deadlock (sync.Once protects the channel send)
	s := Start("test")
	s.Stop()
	s.Stop() // second call should be a no-op, not block
}

func TestSpinner_StopFailTwice(t *testing.T) {
	s := Start("test")
	s.StopFail()
	s.StopFail() // second call should be a no-op
}

func TestSpinner_StopThenStopFail(t *testing.T) {
	s := Start("test")
	s.Stop()
	s.StopFail() // mixed calls should not deadlock
}
