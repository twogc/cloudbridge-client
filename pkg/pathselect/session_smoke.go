package pathselect

import (
	"context"
	"fmt"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/types"
)

// SmokeResult is the outcome of a pathselect session smoke (library / CLI).
type SmokeResult struct {
	ActivePath string
	Metrics    MetricsSnapshot
	Duration   time.Duration
}

// RunSessionSmoke runs Ensure once with production paths from cfg.
// Requires path_select.enabled=true (or force=true) and a non-empty token.
func RunSessionSmoke(ctx context.Context, cfg *types.Config, token, tenant, peer string, force bool) (*SmokeResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("pathselect smoke: nil config")
	}
	ps := ConfigFromTypes(cfg.PathSelect)
	if !ps.Enabled && !force {
		return nil, fmt.Errorf("pathselect smoke: path_select.enabled is false (pass force or enable in config)")
	}
	if force {
		ps.Enabled = true
	}
	ps = ps.Normalize()

	if token == "" {
		return nil, fmt.Errorf("pathselect smoke: token required")
	}

	start := time.Now()
	sel := NewLadderSelector(ps, NewDefaultPaths(cfg))
	sess := NewSession("smoke", tenant, peer, "")
	req := OpenRequest{
		SessionID:  "smoke",
		TenantID:   tenant,
		LocalPeer:  peer,
		RemotePeer: "",
		RemoteHost: "127.0.0.1",
		RemotePort: 80,
		Meta:       map[string]string{"token": token},
	}
	_ = sess.Transition(StateAuthenticating)
	_ = sess.Transition(StateControlReady)

	_, h, err := sel.Ensure(ctx, sess, req)
	if err != nil {
		return &SmokeResult{
			Metrics:  SnapshotMetrics(),
			Duration: time.Since(start),
		}, err
	}
	// optional single health tick
	_ = sel.HealthTick(ctx, sess)

	active := sess.ActivePath
	if h != nil {
		active = h.PathName()
	}
	return &SmokeResult{
		ActivePath: active,
		Metrics:    SnapshotMetrics(),
		Duration:   time.Since(start),
	}, nil
}
