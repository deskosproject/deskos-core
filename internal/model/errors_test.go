package model

import (
	"errors"
	"fmt"
	"testing"
)

func TestDirectErrorListFlattens(t *testing.T) {
	var l ErrorList
	l.Add(ErrorList{errors.New("one"), errors.New("two")})
	if got := len(l); got != 2 {
		t.Fatalf("got %d errors, want 2", got)
	}
}

func TestWrappedErrorListKeepsItsPrefix(t *testing.T) {
	inner := ErrorList{errors.New("inner")}
	var l ErrorList
	l.Add(fmt.Errorf("Workstation/ws: %w", inner))
	if got := l.Error(); got != "Workstation/ws: inner" {
		t.Fatalf("error = %q", got)
	}
}
