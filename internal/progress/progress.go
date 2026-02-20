package progress

import (
	"fmt"
	"sync"
	"time"
)

// Spinner displays a simple animated spinner with a message.
type Spinner struct {
	msg  string
	done chan bool // true = failed, false = success
	wg   sync.WaitGroup
	once sync.Once
}

// Start begins a spinner with the given message.
func Start(msg string) *Spinner {
	s := &Spinner{
		msg:  msg,
		done: make(chan bool, 1),
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
		case failed := <-s.done:
			if failed {
				fmt.Printf("\r  ✗ %s (failed)\n", s.msg)
			} else {
				fmt.Printf("\r  ✓ %s\n", s.msg)
			}
			return
		case <-ticker.C:
			fmt.Printf("\r  %s %s", frames[i%len(frames)], s.msg)
			i++
		}
	}
}

// Stop stops the spinner and marks it as complete.
func (s *Spinner) Stop() {
	s.once.Do(func() { s.done <- false })
	s.wg.Wait()
}

// StopFail stops the spinner and marks it as failed.
func (s *Spinner) StopFail() {
	s.once.Do(func() { s.done <- true })
	s.wg.Wait()
}

// Step prints a progress step message.
func Step(msg string) {
	fmt.Printf("  → %s\n", msg)
}
