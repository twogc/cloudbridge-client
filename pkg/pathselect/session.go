package pathselect

import (
	"context"
	"fmt"
	"time"
)

// allowedTransitions defines legal SessionState edges.
var allowedTransitions = map[SessionState]map[SessionState]bool{
	StateIdle: {
		StateAuthenticating: true,
		StateClosed:         true,
	},
	StateAuthenticating: {
		StateControlReady: true,
		StateFailed:       true,
		StateClosed:       true,
	},
	StateControlReady: {
		StateProbing: true,
		StateFailed:  true,
		StateClosed:  true,
	},
	StateProbing: {
		StatePathSelected: true,
		StateFailed:       true,
		StateClosed:       true,
	},
	StatePathSelected: {
		StateDegraded:    true,
		StateHandingOver: true,
		StateProbing:     true, // re-ladder
		StateFailed:      true,
		StateClosed:      true,
	},
	StateDegraded: {
		StateProbing:     true,
		StateHandingOver: true,
		StatePathSelected: true,
		StateFailed:      true,
		StateClosed:      true,
	},
	StateHandingOver: {
		StatePathSelected: true,
		StateDegraded:     true,
		StateFailed:       true,
		StateClosed:       true,
	},
	StateFailed: {
		StateProbing:        true,
		StateAuthenticating: true,
		StateClosed:         true,
	},
	StateClosed: {},
}

// Transition moves sess to next if the edge is allowed.
func (s *Session) Transition(next SessionState) error {
	if s == nil {
		return fmt.Errorf("%w: nil session", ErrInvalidTransition)
	}
	from := s.State
	if from == "" {
		from = StateIdle
		s.State = StateIdle
	}
	if from == next {
		s.UpdatedAt = time.Now()
		return nil
	}
	ok := allowedTransitions[from][next]
	if !ok {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, next)
	}
	s.State = next
	s.UpdatedAt = time.Now()
	return nil
}

// NewSession creates an Idle session.
func NewSession(id, tenant, localPeer, remotePeer string) *Session {
	return &Session{
		ID:         id,
		TenantID:   tenant,
		LocalPeer:  localPeer,
		RemotePeer: remotePeer,
		State:      StateIdle,
		UpdatedAt:  time.Now(),
	}
}

// ClearActive closes and drops the active handle (best-effort).
func (s *Session) ClearActive() {
	if s == nil {
		return
	}
	if s.Active != nil {
		_ = s.Active.Close(context.Background())
	}
	s.Active = nil
	s.ActivePath = ""
}
