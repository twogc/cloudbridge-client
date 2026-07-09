package p2p

import (
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/api"
)

func TestApplyClientP2PDefaults_YAMLWins(t *testing.T) {
	cfg := &P2PConfig{} // zero from JWT extract
	ApplyClientP2PDefaults(cfg, 10*time.Second, 5*time.Second)
	if cfg.HeartbeatInterval != 10*time.Second {
		t.Fatalf("interval=%v want 10s", cfg.HeartbeatInterval)
	}
	if cfg.HeartbeatTimeout != 5*time.Second {
		t.Fatalf("timeout=%v want 5s", cfg.HeartbeatTimeout)
	}
}

func TestFillDefaults_HeartbeatWhenZero(t *testing.T) {
	cfg := &P2PConfig{TenantID: "default"}
	cfg.FillDefaults()
	if cfg.HeartbeatInterval != 30*time.Second {
		t.Fatalf("got %v want 30s", cfg.HeartbeatInterval)
	}
}

func TestNewManagerWithAPI_SetsHeartbeatOnZeroConfig(t *testing.T) {
	logger := &testLogger{}
	m := NewManagerWithAPI(&P2PConfig{TenantID: "t"}, &api.ManagerConfig{
		BaseURL: "http://127.0.0.1:5552",
	}, nil, "tok", logger)
	if m.config.HeartbeatInterval <= 0 {
		t.Fatal("expected default heartbeat on manager config")
	}
}
