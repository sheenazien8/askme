package spinner

import (
	"fmt"
	"time"
)

type Spinner struct {
	stop    chan struct{}
	stopped chan struct{}
}

func NewSpinner() *Spinner {
	return &Spinner{
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (s *Spinner) Start() {
	go func() {
		spinChars := []string{"|", "/", "-", "\\"}
		i := 0
		for {
			select {
			case <-s.stop:
				close(s.stopped)
				return
			default:
				fmt.Printf("\r%s Generating response", spinChars[i])
				i = (i + 1) % len(spinChars)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.stopped
	fmt.Print("\r")
}

