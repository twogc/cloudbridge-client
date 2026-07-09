// Package pathselect provides session state and path ladder interfaces for
// A↔B connectivity (GAP Phase A). Library only — no CLI wiring yet.
package pathselect

import (
	"context"
	"errors"
	"time"
)

// Well-known path names (adapters land in Phase B).
const (
	PathRelayQUIC   = "relay_quic"
	PathGRPCTunnel  = "grpc_tunnel"
	PathDirectQUIC  = "direct_quic"
	PathICE         = "ice"
	PathWireGuard   = "wireguard"
	PathMASQUE      = "masque"
)

// SessionState is the high-level connectivity state machine
// (see docs/plans/PATH_SELECTION_STATE_MACHINE.md).
type SessionState string

const (
	StateIdle            SessionState = "idle"
	StateAuthenticating  SessionState = "authenticating"
	StateControlReady    SessionState = "control_ready"
	StateProbing         SessionState = "probing"
	StatePathSelected    SessionState = "path_selected"
	StateDegraded        SessionState = "degraded"
	StateHandingOver     SessionState = "handing_over"
	StateFailed          SessionState = "failed"
	StateClosed          SessionState = "closed"
)

// ErrAllPathsFailed is returned when the ladder exhausts every path.
var ErrAllPathsFailed = errors.New("pathselect: all paths failed")

// ErrInvalidTransition is returned for illegal session state changes.
var ErrInvalidTransition = errors.New("pathselect: invalid state transition")

// ErrSelectorDisabled means path_select.enabled is false.
var ErrSelectorDisabled = errors.New("pathselect: selector disabled")

// ErrNoPathsConfigured means the path order list is empty.
var ErrNoPathsConfigured = errors.New("pathselect: no paths in order")

// OpenRequest is the app-facing request to open a data path A↔B.
type OpenRequest struct {
	SessionID string
	TenantID  string
	LocalPeer string
	RemotePeer string
	// Service / tunnel mode extras (optional).
	LocalListen string // e.g. "127.0.0.1:0"
	RemoteHost  string
	RemotePort  int
	// Metadata for adapters.
	Meta map[string]string
}

// ProbeResult is a cheap liveness observation for one path.
type ProbeResult struct {
	PathName  string
	OK        bool
	Latency   time.Duration
	Err       error
	CheckedAt time.Time
}

// Handle is an open path instance held by the session.
// Adapters implement this; Phase A uses a minimal stub for tests.
type Handle interface {
	// PathName returns the path that owns this handle.
	PathName() string
	// Close releases resources.
	Close(ctx context.Context) error
}

// Path is one rung of the probe ladder.
type Path interface {
	Name() string
	Probe(ctx context.Context) error
	Open(ctx context.Context, req OpenRequest) (Handle, error)
	Close(ctx context.Context) error
}

// Session holds logical A↔B state. Not concurrency-safe; Selector serializes.
type Session struct {
	ID         string
	TenantID   string
	LocalPeer  string
	RemotePeer string
	State      SessionState
	ActivePath string
	Active     Handle
	LastError  error
	UpdatedAt  time.Time
	// LastFailover is when active path last changed due to failure.
	LastFailover time.Time
}

// Selector tries paths in configured order and monitors health.
type Selector interface {
	Ensure(ctx context.Context, sess *Session, req OpenRequest) (Path, Handle, error)
	HealthTick(ctx context.Context, sess *Session) error
}
