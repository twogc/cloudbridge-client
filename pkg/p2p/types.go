package p2p

import (
	"fmt"
	"net"
	"time"
)

// ConnectionType represents the type of P2P connection
type ConnectionType string

const (
	ConnectionTypeClientServer ConnectionType = "client-server"
	ConnectionTypeServerServer ConnectionType = "server-server"
	ConnectionTypeP2PMesh      ConnectionType = "p2p-mesh"
)

// QUICConfig represents QUIC configuration
type QUICConfig struct {
	ListenPort         int    `json:"listen_port,omitempty"`
	HandshakeTimeout   string `json:"handshake_timeout,omitempty"`
	IdleTimeout        string `json:"idle_timeout,omitempty"`
	MaxStreams         int    `json:"max_streams,omitempty"`
	MaxStreamData      int    `json:"max_stream_data,omitempty"`
	KeepAlivePeriod    string `json:"keep_alive_period,omitempty"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// MeshConfig represents mesh network configuration from JWT
type MeshConfig struct {
	AutoDiscovery     bool        `json:"auto_discovery"`
	Persistent        bool        `json:"persistent"`
	Routing           string      `json:"routing"`    // "hybrid", "direct", "relay"
	Encryption        string      `json:"encryption"` // "quic", "tls"
	HeartbeatInterval interface{} `json:"heartbeat_interval"`
	// Новые поля для конфигурируемых таймингов
	TopologyInterval time.Duration `json:"topology_interval"`
	RoutingInterval  time.Duration `json:"routing_interval"`
	HealthInterval   time.Duration `json:"health_interval"`
	PeerStaleAfter   time.Duration `json:"peer_stale_after"`
	PeerDeadAfter    time.Duration `json:"peer_dead_after"`
}

// PeerWhitelist represents peer whitelist configuration from JWT
type PeerWhitelist struct {
	AllowedPeers []string `json:"allowed_peers"`
	AutoApprove  bool     `json:"auto_approve"`
	MaxPeers     int      `json:"max_peers"`
}

// NetworkConfig represents network configuration from JWT
type NetworkConfig struct {
	Subnet      string   `json:"subnet"`
	DNS         []string `json:"dns"`
	MTU         int      `json:"mtu"`
	STUNServers []string `json:"stun_servers,omitempty"`
	TURNServers []string `json:"turn_servers,omitempty"`
	QUICPort    int      `json:"quic_port,omitempty"`
	ICEPort     int      `json:"ice_port,omitempty"`
}

// P2PConfig represents complete P2P configuration
type P2PConfig struct {
	ConnectionType    ConnectionType `json:"connection_type"`
	QUICConfig        *QUICConfig    `json:"quic_config,omitempty"`
	MeshConfig        *MeshConfig    `json:"mesh_config,omitempty"`
	PeerWhitelist     *PeerWhitelist `json:"peer_whitelist,omitempty"`
	NetworkConfig     *NetworkConfig `json:"network_config,omitempty"`
	TenantID          string         `json:"tenant_id,omitempty"`
	Permissions       []string       `json:"permissions,omitempty"`
	HeartbeatInterval time.Duration  `json:"heartbeat_interval,omitempty"`
	HeartbeatTimeout  time.Duration  `json:"heartbeat_timeout,omitempty"`
}

// Peer represents a discovered peer in the mesh network
type Peer struct {
	ID          string   `json:"id"`
	PublicKey   string   `json:"public_key"`
	Endpoint    string   `json:"endpoint"`
	AllowedIPs  []string `json:"allowed_ips"`
	QUICPort    int      `json:"quic_port,omitempty"`
	ICEPort     int      `json:"ice_port,omitempty"`
	Persistent  bool     `json:"persistent"`
	LastSeen    int64    `json:"last_seen"`
	Latency     int64    `json:"latency_ms"`
	IsConnected bool     `json:"is_connected"`
}

// MeshTopology represents the current mesh network topology
type MeshTopology struct {
	LocalPeerID     string              `json:"local_peer_id"`
	ConnectedPeers  map[string]*Peer    `json:"connected_peers"`
	DiscoveredPeers map[string]*Peer    `json:"discovered_peers"`
	RoutingTable    map[string][]string `json:"routing_table"`
}

// P2PStatus represents the current status of P2P connection
type P2PStatus struct {
	IsConnected       bool           `json:"is_connected"`
	ConnectionType    ConnectionType `json:"connection_type"`
	ActivePeers       int            `json:"active_peers"`
	TotalPeers        int            `json:"total_peers"`
	MeshEnabled       bool           `json:"mesh_enabled"`
	QUICReady         bool           `json:"quic_ready"`
	ICEReady          bool           `json:"ice_ready"`
	ActiveConnections int            `json:"active_connections"`
	LastError         string         `json:"last_error,omitempty"`
	// L3-overlay network status
	L3OverlayReady bool   `json:"l3_overlay_ready"`
	PeerIP         string `json:"peer_ip,omitempty"`
	TenantCIDR     string `json:"tenant_cidr,omitempty"`
	WireGuardReady bool   `json:"wireguard_ready"`
}

// P2PMessage represents a P2P protocol message
type P2PMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
	PeerID    string      `json:"peer_id,omitempty"`
}

// P2PHandshake represents a P2P handshake message
type P2PHandshake struct {
	Type        string   `json:"type"`
	PublicKey   string   `json:"public_key"`
	ListenPort  int      `json:"listen_port"`
	Mode        string   `json:"mode"`
	Peers       []*Peer  `json:"peers,omitempty"`
	Timestamp   int64    `json:"timestamp"`
	TenantID    string   `json:"tenant_id,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// P2PHandshakeResponse represents a P2P handshake response
type P2PHandshakeResponse struct {
	Type         string      `json:"type"`
	Status       string      `json:"status"`
	Message      string      `json:"message,omitempty"`
	PeerID       string      `json:"peer_id,omitempty"`
	MeshConfig   *MeshConfig `json:"mesh_config,omitempty"`
	AllowedPeers []string    `json:"allowed_peers,omitempty"`
}

// PeerDiscoveryRequest represents a peer discovery request
type PeerDiscoveryRequest struct {
	Type      string `json:"type"`
	TenantID  string `json:"tenant_id"`
	PublicKey string `json:"public_key"`
	Endpoint  string `json:"endpoint"`
}

// PeerDiscoveryResponse represents a peer discovery response
type PeerDiscoveryResponse struct {
	Type  string  `json:"type"`
	Peers []*Peer `json:"peers"`
}

// MeshRouteRequest represents a mesh routing request
type MeshRouteRequest struct {
	Type        string `json:"type"`
	Destination string `json:"destination"`
	Source      string `json:"source"`
}

// MeshRouteResponse represents a mesh routing response
type MeshRouteResponse struct {
	Type    string   `json:"type"`
	Route   []string `json:"route"`
	Latency int64    `json:"latency_ms"`
}

// --- helpers & validation ---

// Routing / Encryption enums (не ломают текущие строки, служат справочником)
const (
	RoutingHybrid = "hybrid"
	RoutingDirect = "direct"
	RoutingRelay  = "relay"

	EncryptionQUIC = "quic"
	EncryptionTLS  = "tls"
)

// Valid проверяет корректность типа соединения
func (ct ConnectionType) Valid() bool {
	switch ct {
	case ConnectionTypeClientServer, ConnectionTypeServerServer, ConnectionTypeP2PMesh:
		return true
	default:
		return false
	}
}

// FillDefaults нормализует P2PConfig дефолтами (ничего не перезатирает, если уже задано)
func (c *P2PConfig) FillDefaults() {
	if c.HeartbeatInterval <= 0 {
		// разумный дефолт для клиентской стороны
		c.HeartbeatInterval = 30 * time.Second
	}
	if c.HeartbeatTimeout <= 0 {
		c.HeartbeatTimeout = 10 * time.Second
	}

	if c.MeshConfig != nil {
		c.MeshConfig.FillDefaults()
	}
	if c.NetworkConfig != nil {
		c.NetworkConfig.FillDefaults()
	}
	if !c.ConnectionType.Valid() {
		// дефолт к p2p-mesh для L3 overlay
		c.ConnectionType = ConnectionTypeP2PMesh
	}
}

// FillDefaults нормализует MeshConfig
func (m *MeshConfig) FillDefaults() {
	if m.Routing == "" {
		m.Routing = RoutingHybrid
	}
	if m.Encryption == "" {
		m.Encryption = EncryptionQUIC
	}
	// HeartbeatInterval может быть строкой "30s", числом (ms) или nil
	if d, ok := ParseFlexibleDuration(m.HeartbeatInterval); ok {
		m.HeartbeatInterval = d
	}
}

// FillDefaults нормализует NetworkConfig
func (n *NetworkConfig) FillDefaults() {
	if n.MTU == 0 {
		n.MTU = 1420
	}
	// Порты полезны для авто-дискавери/диагностики
	if n.QUICPort == 0 {
		n.QUICPort = 5553
	}
	if n.ICEPort == 0 {
		n.ICEPort = 19302
	}
}

// ParseFlexibleDuration парсит duration из string ("30s"), float64/int (миллисекунды) или time.Duration
// Возвращает (duration, true) при успехе.
func ParseFlexibleDuration(v interface{}) (time.Duration, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case time.Duration:
		return t, true
	case string:
		if t == "" {
			return 0, false
		}
		if d, err := time.ParseDuration(t); err == nil {
			return d, true
		}
	case float64:
		// часто приходит из JSON как float64 миллисекунд
		return time.Duration(int64(t)) * time.Millisecond, true
	case int:
		return time.Duration(t) * time.Millisecond, true
	case int64:
		return time.Duration(t) * time.Millisecond, true
	case uint64:
		return time.Duration(int64(t)) * time.Millisecond, true
	}
	return 0, false
}

// ContainsIP сообщает, входит ли ip (строка) в Subnet (CIDR) данной NetworkConfig.
// Возвращает false, если Subnet пустой или некорректный.
func (n *NetworkConfig) ContainsIP(ip string) bool {
	if n == nil || n.Subnet == "" {
		return false
	}
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	_, cidr, err := net.ParseCIDR(n.Subnet)
	if err != nil || cidr == nil {
		return false
	}
	return cidr.Contains(parsedIP)
}

// Validate базовая валидация конфигурации (для раннего fail-fast)
func (c *P2PConfig) Validate() error {
	if !c.ConnectionType.Valid() {
		return fmt.Errorf("invalid connection_type: %q", c.ConnectionType)
	}
	if c.MeshConfig != nil {
		switch c.MeshConfig.Routing {
		case RoutingHybrid, RoutingDirect, RoutingRelay, "":
			// ok
		default:
			return fmt.Errorf("invalid mesh.routing: %q", c.MeshConfig.Routing)
		}
		switch c.MeshConfig.Encryption {
		case EncryptionQUIC, EncryptionTLS, "":
			// ok
		default:
			return fmt.Errorf("invalid mesh.encryption: %q", c.MeshConfig.Encryption)
		}
	}
	return nil
}
