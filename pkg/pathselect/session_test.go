package pathselect

import (
	"errors"
	"testing"
)

func TestSession_TransitionHappyPath(t *testing.T) {
	s := NewSession("s1", "t1", "a", "b")
	steps := []SessionState{
		StateAuthenticating,
		StateControlReady,
		StateProbing,
		StatePathSelected,
		StateDegraded,
		StateProbing,
		StatePathSelected,
		StateClosed,
	}
	for _, next := range steps {
		if err := s.Transition(next); err != nil {
			t.Fatalf("transition to %s: %v (from %s)", next, err, s.State)
		}
		if s.State != next {
			t.Fatalf("want %s got %s", next, s.State)
		}
	}
}

func TestSession_InvalidTransition(t *testing.T) {
	s := NewSession("s1", "t1", "a", "b")
	// idle → path_selected is illegal
	err := s.Transition(StatePathSelected)
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	if s.State != StateIdle {
		t.Fatalf("state should stay idle, got %s", s.State)
	}
}

func TestSession_IdempotentSameState(t *testing.T) {
	s := NewSession("s1", "t1", "a", "b")
	if err := s.Transition(StateIdle); err != nil {
		t.Fatal(err)
	}
	if s.State != StateIdle {
		t.Fatal(s.State)
	}
}
