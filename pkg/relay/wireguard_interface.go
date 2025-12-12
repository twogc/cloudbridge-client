package relay

// WireGuardManagerInterface defines the common interface for WireGuard managers across platforms
type WireGuardManagerInterface interface {
	Connect() error
	Disconnect() error
	Stop() error
	TearDown() error
	IsConnected() bool
	GetInterfaceName() string
	GetStatus() (map[string]interface{}, error)
}

// Ensure both implementations satisfy the interface
var _ WireGuardManagerInterface = (*WireGuardManager)(nil)
