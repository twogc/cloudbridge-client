package tunnel

import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/interfaces"
)

// BufferManager manages buffer pools for efficient data transfer
type BufferManager struct {
	BufferSize int
	MaxBuffers int
	BufferPool chan []byte
}

// NewBufferManager creates a new buffer manager
func NewBufferManager(bufferSize, maxBuffers int) *BufferManager {
	bm := &BufferManager{
		BufferSize: bufferSize,
		MaxBuffers: maxBuffers,
		BufferPool: make(chan []byte, maxBuffers),
	}

	// Pre-allocate buffers
	for i := 0; i < maxBuffers; i++ {
		bm.BufferPool <- make([]byte, bufferSize)
	}

	return bm
}

// GetBuffer gets a buffer from the pool
func (bm *BufferManager) GetBuffer() []byte {
	select {
	case buf := <-bm.BufferPool:
		return buf
	default:
		// If pool is empty, create a new buffer
		return make([]byte, bm.BufferSize)
	}
}

// ReturnBuffer returns a buffer to the pool
func (bm *BufferManager) ReturnBuffer(buf []byte) {
	// Reset buffer
	for i := range buf {
		buf[i] = 0
	}

	select {
	case bm.BufferPool <- buf:
		// Buffer returned to pool
	default:
		// Pool is full, discard buffer
	}
}

// Tunnel represents a tunnel configuration
type Tunnel struct {
	ID         string
	LocalPort  int
	RemoteHost string
	RemotePort int
	Active     bool
	CreatedAt  time.Time
	LastUsed   time.Time
	BufferMgr  *BufferManager
	Stats      *TunnelStats
	mu         sync.RWMutex // Mutex for Active field
}

// IsActive safely checks if tunnel is active
func (t *Tunnel) IsActive() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Active
}

// SetActive safely sets tunnel active status
func (t *Tunnel) SetActive(active bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Active = active
}

// TunnelStats represents tunnel statistics
type TunnelStats struct {
	BytesTransferred   int64
	ConnectionsHandled int64
	ActiveConnections  int32
	LastActivity       time.Time
	mu                 sync.RWMutex
}

// NewTunnelStats creates new tunnel statistics
func NewTunnelStats() *TunnelStats {
	return &TunnelStats{
		LastActivity: time.Now(),
	}
}

// UpdateBytesTransferred updates bytes transferred count
func (ts *TunnelStats) UpdateBytesTransferred(bytes int64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.BytesTransferred += bytes
	ts.LastActivity = time.Now()
}

// IncrementConnections increments connection count
func (ts *TunnelStats) IncrementConnections() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.ConnectionsHandled++
	ts.ActiveConnections++
	ts.LastActivity = time.Now()
}

// DecrementConnections decrements active connection count
func (ts *TunnelStats) DecrementConnections() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.ActiveConnections--
	ts.LastActivity = time.Now()
}

// GetStats returns a copy of current statistics
func (ts *TunnelStats) GetStats() map[string]interface{} {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return map[string]interface{}{
		"bytes_transferred":   ts.BytesTransferred,
		"connections_handled": ts.ConnectionsHandled,
		"active_connections":  ts.ActiveConnections,
		"last_activity":       ts.LastActivity,
	}
}

// Manager handles tunnel operations
type Manager struct {
	client  interfaces.ClientInterface
	tunnels map[string]*Tunnel
	mu      sync.RWMutex
}

// NewManager creates a new tunnel manager
func NewManager(client interfaces.ClientInterface) *Manager {
	return &Manager{
		client:  client,
		tunnels: make(map[string]*Tunnel),
	}
}

// RegisterTunnel registers a new tunnel
func (m *Manager) RegisterTunnel(tunnelID string, localPort int, remoteHost string, remotePort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate tunnel parameters
	if err := m.validateTunnelParams(localPort, remoteHost, remotePort); err != nil {
		return fmt.Errorf("invalid tunnel parameters: %w", err)
	}

	// Check if tunnel already exists
	if _, exists := m.tunnels[tunnelID]; exists {
		return fmt.Errorf("tunnel %s already exists", tunnelID)
	}

	// Create tunnel
	tunnel := &Tunnel{
		ID:         tunnelID,
		LocalPort:  localPort,
		RemoteHost: remoteHost,
		RemotePort: remotePort,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
		BufferMgr:  NewBufferManager(4096, 100),
		Stats:      NewTunnelStats(),
	}
	tunnel.SetActive(true)

	m.tunnels[tunnelID] = tunnel

	// Start tunnel proxy
	go m.startTunnelProxy(tunnel)

	return nil
}

// UnregisterTunnel removes a tunnel
func (m *Manager) UnregisterTunnel(tunnelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tunnel, exists := m.tunnels[tunnelID]
	if !exists {
		return fmt.Errorf("tunnel %s not found", tunnelID)
	}

	tunnel.SetActive(false)
	delete(m.tunnels, tunnelID)

	return nil
}

// GetTunnel returns a tunnel by ID
func (m *Manager) GetTunnel(tunnelID string) (*Tunnel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnel, exists := m.tunnels[tunnelID]
	return tunnel, exists
}

// ListTunnels returns all registered tunnels
func (m *Manager) ListTunnels() []*Tunnel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(m.tunnels))
	for _, tunnel := range m.tunnels {
		tunnels = append(tunnels, tunnel)
	}

	return tunnels
}

// validateTunnelParams validates tunnel parameters
func (m *Manager) validateTunnelParams(localPort int, remoteHost string, remotePort int) error {
	// Validate local port
	if localPort <= 0 || localPort > 65535 {
		return fmt.Errorf("invalid local port: %d", localPort)
	}

	// Validate remote host
	if remoteHost == "" {
		return fmt.Errorf("remote host cannot be empty")
	}

	// Validate remote port
	if remotePort <= 0 || remotePort > 65535 {
		return fmt.Errorf("invalid remote port: %d", remotePort)
	}

	// Check if local port is already in use
	if m.isPortInUse(localPort) {
		return fmt.Errorf("local port %d is already in use", localPort)
	}

	return nil
}

// isPortInUse checks if a port is already in use
func (m *Manager) isPortInUse(port int) bool {
	// Check if any existing tunnel uses this port
	for _, tunnel := range m.tunnels {
		if tunnel.LocalPort == port && tunnel.IsActive() {
			return true
		}
	}

	// Check if port is actually in use by trying to bind to it
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		_ = err // Игнорируем ошибку закрытия при проверке порта
		return true
	}
	if err := ln.Close(); err != nil {
		_ = err // Игнорируем ошибку закрытия при проверке порта
	}
	return false
}

// startTunnelProxy starts a proxy for the tunnel
func (m *Manager) startTunnelProxy(tunnel *Tunnel) {
	// Listen on local port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", tunnel.LocalPort))
	if err != nil {
		fmt.Printf("Failed to start tunnel %s: %v\n", tunnel.ID, err)
		return
	}
	defer func() {
		if err := listener.Close(); err != nil {
			fmt.Printf("Failed to close listener for tunnel %s: %v\n", tunnel.ID, err)
		}
	}()

	fmt.Printf("Tunnel %s started: localhost:%d -> %s:%d\n",
		tunnel.ID, tunnel.LocalPort, tunnel.RemoteHost, tunnel.RemotePort)

	for tunnel.IsActive() {
		// Accept local connection
		localConn, err := listener.Accept()
		if err != nil {
			if tunnel.IsActive() {
				fmt.Printf("Failed to accept connection for tunnel %s: %v\n", tunnel.ID, err)
			}
			continue
		}

		// Handle connection in goroutine
		go m.handleTunnelConnection(tunnel, localConn)
	}
}

// handleTunnelConnection handles a single tunnel connection
func (m *Manager) handleTunnelConnection(tunnel *Tunnel, localConn net.Conn) {
	defer func() {
		if err := localConn.Close(); err != nil {
			fmt.Printf("Failed to close local connection for tunnel %s: %v\n", tunnel.ID, err)
		}
	}()

	// Update last used time and increment connection count
	tunnel.LastUsed = time.Now()
	tunnel.Stats.IncrementConnections()
	defer tunnel.Stats.DecrementConnections()

	// Connect to remote host
	remoteConn, err := net.Dial("tcp", net.JoinHostPort(tunnel.RemoteHost, strconv.Itoa(tunnel.RemotePort)))
	if err != nil {
		fmt.Printf("Failed to connect to remote host for tunnel %s: %v\n", tunnel.ID, err)
		return
	}
	defer func() {
		if err := remoteConn.Close(); err != nil {
			fmt.Printf("Failed to close remote connection for tunnel %s: %v\n", tunnel.ID, err)
		}
	}()

	// Start bidirectional data transfer
	done := make(chan bool, 2)

	// Local to remote
	go func() {
		buffer := tunnel.BufferMgr.GetBuffer()
		defer tunnel.BufferMgr.ReturnBuffer(buffer)
		for {
			n, err := localConn.Read(buffer)
			if err != nil {
				break
			}
			if n > 0 {
				_, err = remoteConn.Write(buffer[:n])
				if err != nil {
					break
				}
				tunnel.Stats.UpdateBytesTransferred(int64(n))
			}
		}
		done <- true
	}()

	// Remote to local
	go func() {
		buffer := tunnel.BufferMgr.GetBuffer()
		defer tunnel.BufferMgr.ReturnBuffer(buffer)
		for {
			n, err := remoteConn.Read(buffer)
			if err != nil {
				break
			}
			if n > 0 {
				_, err = localConn.Write(buffer[:n])
				if err != nil {
					break
				}
				tunnel.Stats.UpdateBytesTransferred(int64(n))
			}
		}
		done <- true
	}()

	// Wait for both directions to complete
	<-done
	<-done
}

// GetTunnelStats returns statistics for all tunnels
func (m *Manager) GetTunnelStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_tunnels"] = len(m.tunnels)

	activeCount := 0
	for _, tunnel := range m.tunnels {
		if tunnel.IsActive() {
			activeCount++
		}
	}
	stats["active_tunnels"] = activeCount

	return stats
}
