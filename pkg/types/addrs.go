package types

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// portOr returns v if > 0, otherwise def.
func portOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// EffectiveP2PAPIPort returns the REST/P2P API TCP port (canonical 5552).
func (c *Config) EffectiveP2PAPIPort() int {
	if c == nil {
		return DefaultP2PAPIPort
	}
	return portOr(c.Relay.Ports.P2PAPI, DefaultP2PAPIPort)
}

// EffectiveGRPCPort returns the gRPC control-plane port (canonical 8444).
func (c *Config) EffectiveGRPCPort() int {
	if c == nil {
		return DefaultGRPCPort
	}
	return portOr(c.Relay.Ports.GRPC, DefaultGRPCPort)
}

// EffectiveP2PQUICPort returns the P2P QUIC UDP port (canonical 5553).
func (c *Config) EffectiveP2PQUICPort() int {
	if c == nil {
		return DefaultP2PQUICPort
	}
	return portOr(c.Relay.Ports.QUIC, DefaultP2PQUICPort)
}

// EffectiveQUICMainPort returns the main QUIC UDP port (canonical 9090).
func (c *Config) EffectiveQUICMainPort() int {
	if c == nil {
		return DefaultQUICMainPort
	}
	return portOr(c.Relay.Ports.QUICMain, DefaultQUICMainPort)
}

// EffectiveHTTPAPIPort returns the HTTP API TCP port (canonical 9090).
func (c *Config) EffectiveHTTPAPIPort() int {
	if c == nil {
		return DefaultHTTPAPIPort
	}
	return portOr(c.Relay.Ports.HTTPAPI, DefaultHTTPAPIPort)
}

// EffectiveLegacyTCPPort returns main/legacy TCP port (canonical 5550).
// If only old configs set relay.port to 5553 (historical QUIC confusion),
// callers should prefer role-specific helpers for gRPC/REST/QUIC.
func (c *Config) EffectiveLegacyTCPPort() int {
	if c == nil {
		return DefaultLegacyTCPPort
	}
	if c.Relay.Port > 0 {
		return c.Relay.Port
	}
	return DefaultLegacyTCPPort
}

// RelayHost returns relay host or empty.
func (c *Config) RelayHost() string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Relay.Host)
}

// P2PAPIBaseURL returns https://host:p2p_api for REST calls.
// If api.base_url is set non-empty, it wins (explicit override).
func (c *Config) P2PAPIBaseURL() string {
	if c == nil {
		return ""
	}
	if u := strings.TrimSpace(c.API.BaseURL); u != "" {
		return strings.TrimRight(u, "/")
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return fmt.Sprintf("https://%s", net.JoinHostPort(host, strconv.Itoa(c.EffectiveP2PAPIPort())))
}

// GRPCTarget returns host:grpc for grpc.Dial.
func (c *Config) GRPCTarget() string {
	if c == nil {
		return ""
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(c.EffectiveGRPCPort()))
}

// P2PQUICAddr returns host:quic for P2P QUIC UDP dial.
func (c *Config) P2PQUICAddr() string {
	if c == nil {
		return ""
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(c.EffectiveP2PQUICPort()))
}

// QUICMainAddr returns host:quic_main for main QUIC UDP dial.
func (c *Config) QUICMainAddr() string {
	if c == nil {
		return ""
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(c.EffectiveQUICMainPort()))
}

// LegacyTCPAddr returns host:legacy TCP for deprecated JSON/TLS control path.
func (c *Config) LegacyTCPAddr() string {
	if c == nil {
		return ""
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(c.EffectiveLegacyTCPPort()))
}

// WebSocketURL returns WSS endpoint; uses config override if set.
func (c *Config) WebSocketURL() string {
	if c == nil {
		return ""
	}
	if ep := strings.TrimSpace(c.WebSocket.Endpoint); ep != "" {
		return ep
	}
	host := c.RelayHost()
	if host == "" {
		return ""
	}
	return fmt.Sprintf("wss://%s/ws", net.JoinHostPort(host, strconv.Itoa(c.EffectiveP2PAPIPort())))
}
