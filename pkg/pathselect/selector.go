package pathselect

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// LadderSelector implements Selector over a registry of Path adapters.
type LadderSelector struct {
	cfg   Config
	paths map[string]Path
	mu    sync.Mutex
}

// NewLadderSelector builds a selector. Paths not in cfg.Order are ignored by Ensure.
func NewLadderSelector(cfg Config, paths []Path) *LadderSelector {
	cfg = cfg.Normalize()
	m := make(map[string]Path, len(paths))
	for _, p := range paths {
		if p == nil {
			continue
		}
		m[p.Name()] = p
	}
	return &LadderSelector{cfg: cfg, paths: m}
}

// Ensure runs the probe ladder and opens the first healthy path.
// On success sess is PathSelected with Active set.
func (s *LadderSelector) Ensure(ctx context.Context, sess *Session, req OpenRequest) (Path, Handle, error) {
	if s == nil {
		return nil, nil, fmt.Errorf("pathselect: nil selector")
	}
	if sess == nil {
		return nil, nil, fmt.Errorf("pathselect: nil session")
	}
	if !s.cfg.Enabled {
		return nil, nil, ErrSelectorDisabled
	}
	if len(s.cfg.Order) == 0 {
		return nil, nil, ErrNoPathsConfigured
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Enter probing from control_ready / failed / degraded / idle (after auth stub).
	switch sess.State {
	case StateIdle, StateAuthenticating:
		// Phase A: library allows starting ladder without full auth wiring.
		if err := sess.Transition(StateAuthenticating); err != nil && sess.State != StateAuthenticating {
			// already authenticating
		}
		_ = sess.Transition(StateControlReady)
		_ = sess.Transition(StateProbing)
	case StateControlReady, StateFailed, StateDegraded, StatePathSelected:
		_ = sess.Transition(StateProbing)
	case StateProbing:
		// continue
	default:
		if err := sess.Transition(StateProbing); err != nil {
			return nil, nil, err
		}
	}

	ladderCtx, cancel := context.WithTimeout(ctx, s.cfg.LadderTimeout)
	defer cancel()

	var lastErr error
	for _, name := range s.cfg.Order {
		p, ok := s.paths[name]
		if !ok {
			lastErr = fmt.Errorf("path %q not registered", name)
			continue
		}

		probeCtx, probeCancel := context.WithTimeout(ladderCtx, s.cfg.ProbeTimeout)
		err := p.Probe(probeCtx)
		probeCancel()
		if err != nil {
			lastErr = fmt.Errorf("%s probe: %w", name, err)
			continue
		}

		openCtx, openCancel := context.WithTimeout(ladderCtx, s.cfg.ProbeTimeout)
		h, err := p.Open(openCtx, req)
		openCancel()
		if err != nil {
			lastErr = fmt.Errorf("%s open: %w", name, err)
			continue
		}

		// Success: replace previous active.
		if sess.Active != nil && sess.Active != h {
			_ = sess.Active.Close(ctx)
		}
		sess.Active = h
		sess.ActivePath = name
		sess.LastError = nil
		_ = sess.Transition(StatePathSelected)
		notePathSelected()
		return p, h, nil
	}

	sess.LastError = lastErr
	if lastErr == nil {
		lastErr = ErrAllPathsFailed
	}
	_ = sess.Transition(StateFailed)
	notePathFail()
	return nil, nil, fmt.Errorf("%w: %v", ErrAllPathsFailed, lastErr)
}

// HealthTick probes the active path; on failure marks Degraded and optionally re-ladders.
func (s *LadderSelector) HealthTick(ctx context.Context, sess *Session) error {
	if s == nil || sess == nil {
		return fmt.Errorf("pathselect: nil selector or session")
	}
	if !s.cfg.Enabled {
		return ErrSelectorDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if sess.State != StatePathSelected && sess.State != StateDegraded {
		return nil
	}
	if sess.ActivePath == "" {
		return nil
	}
	p, ok := s.paths[sess.ActivePath]
	if !ok {
		_ = sess.Transition(StateDegraded)
		return fmt.Errorf("active path %q not registered", sess.ActivePath)
	}

	probeCtx, cancel := context.WithTimeout(ctx, s.cfg.ProbeTimeout)
	err := p.Probe(probeCtx)
	cancel()
	if err == nil {
		if sess.State == StateDegraded {
			_ = sess.Transition(StatePathSelected)
		}
		return nil
	}

	sess.LastError = err
	_ = sess.Transition(StateDegraded)

	// Cooldown: do not re-ladder if last failover was too recent.
	if !sess.LastFailover.IsZero() && time.Since(sess.LastFailover) < s.cfg.FailoverCooldown {
		return err
	}

	// Soft re-ladder (Phase A/C): try Ensure without holding forever — unlock via re-entry pattern.
	// We already hold mu; call internal ensure body is nested — so unlock is not possible.
	// Perform inline re-ladder under same lock.
	req := OpenRequest{
		SessionID:  sess.ID,
		TenantID:   sess.TenantID,
		LocalPeer:  sess.LocalPeer,
		RemotePeer: sess.RemotePeer,
	}
	_ = sess.Transition(StateProbing)

	ladderCtx, lcancel := context.WithTimeout(ctx, s.cfg.LadderTimeout)
	defer lcancel()

	var lastErr error
	for _, name := range s.cfg.Order {
		if name == sess.ActivePath && !s.cfg.SoftFail {
			// still try other paths first when hard fail; skip current after failed probe
			continue
		}
		pp, ok := s.paths[name]
		if !ok {
			continue
		}
		pctx, pc := context.WithTimeout(ladderCtx, s.cfg.ProbeTimeout)
		perr := pp.Probe(pctx)
		pc()
		if perr != nil {
			lastErr = perr
			continue
		}
		octx, oc := context.WithTimeout(ladderCtx, s.cfg.ProbeTimeout)
		h, oerr := pp.Open(octx, req)
		oc()
		if oerr != nil {
			lastErr = oerr
			continue
		}
		if sess.Active != nil {
			_ = sess.Active.Close(ctx)
		}
		sess.Active = h
		sess.ActivePath = name
		sess.LastFailover = time.Now()
		sess.LastError = nil
		_ = sess.Transition(StatePathSelected)
		noteFailover()
		notePathSelected()
		return nil
	}

	if lastErr == nil {
		lastErr = err
	}
	sess.LastError = lastErr
	_ = sess.Transition(StateFailed)
	notePathFail()
	return fmt.Errorf("%w: health failover: %v", ErrAllPathsFailed, lastErr)
}

// Config returns a copy of the selector config.
func (s *LadderSelector) Config() Config {
	if s == nil {
		return DefaultConfig()
	}
	return s.cfg
}
