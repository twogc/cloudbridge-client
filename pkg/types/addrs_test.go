package types

import "testing"

func TestEffectivePorts_Defaults(t *testing.T) {
	c := &Config{Relay: RelayConfig{Host: "edge.example"}}
	if c.EffectiveP2PAPIPort() != DefaultP2PAPIPort {
		t.Fatalf("p2p api: got %d want %d", c.EffectiveP2PAPIPort(), DefaultP2PAPIPort)
	}
	if c.EffectiveGRPCPort() != DefaultGRPCPort {
		t.Fatalf("grpc: got %d want %d", c.EffectiveGRPCPort(), DefaultGRPCPort)
	}
	if c.EffectiveP2PQUICPort() != DefaultP2PQUICPort {
		t.Fatalf("quic: got %d want %d", c.EffectiveP2PQUICPort(), DefaultP2PQUICPort)
	}
	if c.EffectiveLegacyTCPPort() != DefaultLegacyTCPPort {
		t.Fatalf("legacy: got %d want %d", c.EffectiveLegacyTCPPort(), DefaultLegacyTCPPort)
	}
}

func TestEffectivePorts_Overrides(t *testing.T) {
	c := &Config{
		Relay: RelayConfig{
			Host: "h",
			Port: 5550,
			Ports: RelayPorts{
				P2PAPI: 15552,
				GRPC:   18444,
				QUIC:   15553,
			},
		},
	}
	if c.EffectiveP2PAPIPort() != 15552 {
		t.Fatal(c.EffectiveP2PAPIPort())
	}
	if c.EffectiveGRPCPort() != 18444 {
		t.Fatal(c.EffectiveGRPCPort())
	}
	if c.EffectiveP2PQUICPort() != 15553 {
		t.Fatal(c.EffectiveP2PQUICPort())
	}
}

func TestAddrHelpers(t *testing.T) {
	c := &Config{
		Relay: RelayConfig{
			Host: "edge.2gc.ru",
			Ports: RelayPorts{
				P2PAPI: DefaultP2PAPIPort,
				GRPC:   DefaultGRPCPort,
				QUIC:   DefaultP2PQUICPort,
			},
		},
	}
	if got := c.GRPCTarget(); got != "edge.2gc.ru:8444" {
		t.Fatalf("GRPCTarget: %q", got)
	}
	if got := c.P2PQUICAddr(); got != "edge.2gc.ru:5553" {
		t.Fatalf("P2PQUICAddr: %q", got)
	}
	if got := c.P2PAPIBaseURL(); got != "https://edge.2gc.ru:5552" {
		t.Fatalf("P2PAPIBaseURL: %q", got)
	}
}

func TestP2PAPIBaseURL_ExplicitOverride(t *testing.T) {
	c := &Config{
		Relay: RelayConfig{Host: "edge.2gc.ru", Ports: RelayPorts{P2PAPI: 5552}},
		API:   APIConfig{BaseURL: "https://custom.example:9443/"},
	}
	if got := c.P2PAPIBaseURL(); got != "https://custom.example:9443" {
		t.Fatalf("got %q", got)
	}
}
