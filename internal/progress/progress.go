package progress

import (
	"fmt"
	"sync"
	"time"
)

// Spinner displays a simple animated spinner with a message.
type Spinner struct {
	msg    string
	done   chan struct{}
	wg     sync.WaitGroup
}

// Start begins a spinner with the given message.
func Start(msg string) *Spinner {
	s := &Spinner{
		msg:  msg,
		done: make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer s.wg.Done()
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			fmt.Printf("\r  ✓ %s\n", s.msg)
			return
		case <-ticker.C:
			fmt.Printf("\r  %s %s", frames[i%len(frames)], s.msg)
			i++
		}
	}
}

// Stop stops the spinner and marks it as complete.
func (s *Spinner) Stop() {
	close(s.done)
	s.wg.Wait()
}

// StopFail stops the spinner and marks it as failed.
func (s *Spinner) StopFail() {
	s.msg = s.msg + " (failed)"
	close(s.done)
	s.wg.Wait()
}

// Step prints a progress step message.
func Step(msg string) {
	fmt.Printf("  → %s\n", msg)
}
