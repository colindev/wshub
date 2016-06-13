package wshub

import (
	"sync"
	"testing"
	"time"
)

func TestIsRunning(t *testing.T) {

	wg := &sync.WaitGroup{}

	wg.Add(3)

	h := New(nil)

	go func() {
		if h.IsRunning() {
			t.Error("step 1 running MUST be false")
		}
	}()

	time.Sleep(time.Nanosecond * 50)
	h.running = true

	if !h.IsRunning() {
		t.Error("step 2 running MUST be true")
	}

}