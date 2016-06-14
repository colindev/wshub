package wshub

import (
	"errors"
	"io"
	"testing"
)

func TestResolveInCome(t *testing.T) {
	c := &Client{}
	err := errors.New("test error")

	accept := func(s string) {
		if s != "ok" {
			t.Errorf("accept recieve %#v", s)
		}
	}

	reject := func(e error) {
		if e.Error() != "wshub client: test error" {
			t.Errorf("reject recieve %#v", e)
		}
	}

	// test ok
	if !c.resolveInCome("ok", nil, accept, reject) {
		t.Error("test ok MUST return true for go on")
	}

	// test error
	if !c.resolveInCome("x", err, accept, reject) {
		t.Error("test error MUST return true for go on")
	}

	// test EOF
	if c.resolveInCome("x", io.EOF, accept, reject) {
		t.Error("test EOF MUST return false to stop")
	}
}
