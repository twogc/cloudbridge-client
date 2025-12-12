package p2p

import (
	"testing"
	"time"
)

func TestParseFlexibleDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected time.Duration
		ok       bool
	}{
		{
			name:     "string duration",
			input:    "30s",
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "string duration with minutes",
			input:    "2m30s",
			expected: 2*time.Minute + 30*time.Second,
			ok:       true,
		},
		{
			name:     "float64 milliseconds",
			input:    float64(30000),
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "int milliseconds",
			input:    30000,
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "int64 milliseconds",
			input:    int64(30000),
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "uint64 milliseconds",
			input:    uint64(30000),
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "time.Duration",
			input:    30 * time.Second,
			expected: 30 * time.Second,
			ok:       true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
			ok:       false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: 0,
			ok:       false,
		},
		{
			name:     "invalid string",
			input:    "invalid",
			expected: 0,
			ok:       false,
		},
		{
			name:     "unsupported type",
			input:    []string{"test"},
			expected: 0,
			ok:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseFlexibleDuration(tt.input)
			if ok != tt.ok {
				t.Errorf("ParseFlexibleDuration() ok = %v, want %v", ok, tt.ok)
			}
			if result != tt.expected {
				t.Errorf("ParseFlexibleDuration() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNetworkConfig_ContainsIP(t *testing.T) {
	tests := []struct {
		name     string
		config   *NetworkConfig
		ip       string
		expected bool
	}{
		{
			name: "IP in subnet",
			config: &NetworkConfig{
				Subnet: "10.42.17.0/24",
			},
			ip:       "10.42.17.5",
			expected: true,
		},
		{
			name: "IP not in subnet",
			config: &NetworkConfig{
				Subnet: "10.42.17.0/24",
			},
			ip:       "10.42.18.5",
			expected: false,
		},
		{
			name: "IP at subnet boundary",
			config: &NetworkConfig{
				Subnet: "10.42.17.0/24",
			},
			ip:       "10.42.17.255",
			expected: true,
		},
		{
			name: "IP at network address",
			config: &NetworkConfig{
				Subnet: "10.42.17.0/24",
			},
			ip:       "10.42.17.0",
			expected: true,
		},
		{
			name:     "nil config",
			config:   nil,
			ip:       "10.42.17.5",
			expected: false,
		},
		{
			name: "empty subnet",
			config: &NetworkConfig{
				Subnet: "",
			},
			ip:       "10.42.17.5",
			expected: false,
		},
		{
			name: "invalid IP",
			config: &NetworkConfig{
				Subnet: "10.42.17.0/24",
			},
			ip:       "invalid-ip",
			expected: false,
		},
		{
			name: "invalid subnet",
			config: &NetworkConfig{
				Subnet: "invalid-subnet",
			},
			ip:       "10.42.17.5",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ContainsIP(tt.ip)
			if result != tt.expected {
				t.Errorf("NetworkConfig.ContainsIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConnectionType_Valid(t *testing.T) {
	tests := []struct {
		name     string
		ct       ConnectionType
		expected bool
	}{
		{
			name:     "valid client-server",
			ct:       ConnectionTypeClientServer,
			expected: true,
		},
		{
			name:     "valid server-server",
			ct:       ConnectionTypeServerServer,
			expected: true,
		},
		{
			name:     "valid p2p-mesh",
			ct:       ConnectionTypeP2PMesh,
			expected: true,
		},
		{
			name:     "invalid type",
			ct:       ConnectionType("invalid"),
			expected: false,
		},
		{
			name:     "empty type",
			ct:       ConnectionType(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ct.Valid()
			if result != tt.expected {
				t.Errorf("ConnectionType.Valid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestP2PConfig_FillDefaults(t *testing.T) {
	config := &P2PConfig{
		MeshConfig: &MeshConfig{
			HeartbeatInterval: "30s",
		},
		NetworkConfig: &NetworkConfig{},
	}

	config.FillDefaults()

	// Проверяем, что дефолты установлены
	if config.HeartbeatInterval != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v, want %v", config.HeartbeatInterval, 30*time.Second)
	}
	if config.HeartbeatTimeout != 10*time.Second {
		t.Errorf("HeartbeatTimeout = %v, want %v", config.HeartbeatTimeout, 10*time.Second)
	}
	if config.ConnectionType != ConnectionTypeP2PMesh {
		t.Errorf("ConnectionType = %v, want %v", config.ConnectionType, ConnectionTypeP2PMesh)
	}

	// Проверяем MeshConfig
	if config.MeshConfig.Routing != RoutingHybrid {
		t.Errorf("MeshConfig.Routing = %v, want %v", config.MeshConfig.Routing, RoutingHybrid)
	}
	if config.MeshConfig.Encryption != EncryptionQUIC {
		t.Errorf("MeshConfig.Encryption = %v, want %v", config.MeshConfig.Encryption, EncryptionQUIC)
	}

	// Проверяем NetworkConfig
	if config.NetworkConfig.MTU != 1420 {
		t.Errorf("NetworkConfig.MTU = %v, want %v", config.NetworkConfig.MTU, 1420)
	}
	if config.NetworkConfig.QUICPort != 5553 {
		t.Errorf("NetworkConfig.QUICPort = %v, want %v", config.NetworkConfig.QUICPort, 5553)
	}
	if config.NetworkConfig.ICEPort != 19302 {
		t.Errorf("NetworkConfig.ICEPort = %v, want %v", config.NetworkConfig.ICEPort, 19302)
	}
}

func TestP2PConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *P2PConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &P2PConfig{
				ConnectionType: ConnectionTypeP2PMesh,
				MeshConfig: &MeshConfig{
					Routing:    RoutingHybrid,
					Encryption: EncryptionQUIC,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid connection type",
			config: &P2PConfig{
				ConnectionType: ConnectionType("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid mesh routing",
			config: &P2PConfig{
				ConnectionType: ConnectionTypeP2PMesh,
				MeshConfig: &MeshConfig{
					Routing: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid mesh encryption",
			config: &P2PConfig{
				ConnectionType: ConnectionTypeP2PMesh,
				MeshConfig: &MeshConfig{
					Routing:    RoutingHybrid,
					Encryption: "invalid",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("P2PConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
