package p2p

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// MeshNetwork manages the mesh network topology and routing
type MeshNetwork struct {
	config   *MeshConfig
	topology *MeshTopology
	router   *MeshRouter
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
	logger   Logger
}

// MeshRouter handles mesh network routing
type MeshRouter struct {
	routingTable map[string][]string // destination -> route
	latencyTable map[string]int64    // destination -> latency
	ownerIndex   map[string]string   // destination -> ownerPeerID
	mu           sync.RWMutex
}

// NewMeshNetwork creates a new mesh network manager
func NewMeshNetwork(config *MeshConfig, logger Logger) *MeshNetwork {
	ctx, cancel := context.WithCancel(context.Background())

	// Нормализуем конфигурацию с дефолтами
	config.FillDefaults()

	logger.Info("Creating mesh network",
		"routing", config.Routing,
		"encryption", config.Encryption,
		"topology_interval", config.TopologyInterval,
		"routing_interval", config.RoutingInterval,
		"health_interval", config.HealthInterval)

	return &MeshNetwork{
		config: config,
		topology: &MeshTopology{
			LocalPeerID:     "local-peer",
			ConnectedPeers:  make(map[string]*Peer),
			DiscoveredPeers: make(map[string]*Peer),
			RoutingTable:    make(map[string][]string),
		},
		router: &MeshRouter{
			routingTable: make(map[string][]string),
			latencyTable: make(map[string]int64),
			ownerIndex:   make(map[string]string),
		},
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Start starts the mesh network
func (mn *MeshNetwork) Start() error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	// Валидируем конфигурацию перед стартом
	if err := mn.validateConfig(); err != nil {
		return fmt.Errorf("invalid mesh configuration: %w", err)
	}

	mn.logger.Info("Starting mesh network", "routing", mn.config.Routing, "encryption", mn.config.Encryption)

	// Initialize routing based on configuration
	if err := mn.initializeRouting(); err != nil {
		return fmt.Errorf("failed to initialize routing: %w", err)
	}

	// Start mesh management goroutines
	go mn.topologyUpdateLoop()
	go mn.routingUpdateLoop()
	go mn.healthCheckLoop()

	mn.logger.Info("Mesh network started successfully")
	return nil
}

// Stop stops the mesh network (идемпотентный)
func (mn *MeshNetwork) Stop() error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	mn.logger.Info("Stopping mesh network")

	// Cancel context to stop all goroutines (горутины завершатся по ctx.Done())
	mn.cancel()

	mn.logger.Info("Mesh network stopped")
	return nil
}

// GetTopology returns the current mesh topology
func (mn *MeshNetwork) GetTopology() *MeshTopology {
	mn.mu.RLock()
	defer mn.mu.RUnlock()

	// Create a copy to avoid race conditions
	topology := &MeshTopology{
		LocalPeerID:     mn.topology.LocalPeerID,
		ConnectedPeers:  make(map[string]*Peer),
		DiscoveredPeers: make(map[string]*Peer),
		RoutingTable:    make(map[string][]string),
	}

	// Copy connected peers
	for id, peer := range mn.topology.ConnectedPeers {
		peerCopy := *peer
		topology.ConnectedPeers[id] = &peerCopy
	}

	// Copy discovered peers
	for id, peer := range mn.topology.DiscoveredPeers {
		peerCopy := *peer
		topology.DiscoveredPeers[id] = &peerCopy
	}

	// Copy routing table
	for dest, route := range mn.topology.RoutingTable {
		routeCopy := make([]string, len(route))
		copy(routeCopy, route)
		topology.RoutingTable[dest] = routeCopy
	}

	return topology
}

// GetActivePeers returns the number of active peers in the mesh
func (mn *MeshNetwork) GetActivePeers() int {
	mn.mu.RLock()
	defer mn.mu.RUnlock()

	active := 0
	for _, peer := range mn.topology.ConnectedPeers {
		if peer.IsConnected {
			active++
		}
	}
	return active
}

// AddPeer adds a peer to the mesh network
func (mn *MeshNetwork) AddPeer(peer *Peer) error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	mn.logger.Info("Adding peer to mesh network", "peer_id", peer.ID)

	// Add to connected peers
	mn.topology.ConnectedPeers[peer.ID] = peer

	// Update routing table
	if err := mn.updateRoutingForPeer(peer); err != nil {
		mn.logger.Error("Failed to update routing for peer", "peer_id", peer.ID, "error", err)
	}

	mn.logger.Info("Peer added to mesh network successfully", "peer_id", peer.ID)
	return nil
}

// SetLocalPeerID устанавливает ID локального пира
func (mn *MeshNetwork) SetLocalPeerID(id string) {
	mn.mu.Lock()
	defer mn.mu.Unlock()
	mn.topology.LocalPeerID = id
	mn.logger.Info("Local peer ID set", "peer_id", id)
}

// UpsertPeer добавляет или обновляет пира в mesh сети
func (mn *MeshNetwork) UpsertPeer(peer *Peer) {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	if p, ok := mn.topology.ConnectedPeers[peer.ID]; ok {
		// merge: обновляем только «живые» поля
		if peer.Endpoint != "" {
			p.Endpoint = peer.Endpoint
		}
		if peer.PublicKey != "" {
			p.PublicKey = peer.PublicKey
		}
		if peer.Latency > 0 {
			p.Latency = peer.Latency
		}
		if len(peer.AllowedIPs) > 0 {
			p.AllowedIPs = peer.AllowedIPs
		}
		p.IsConnected = peer.IsConnected
		p.LastSeen = time.Now().Unix()
		_ = mn.updateRoutingForPeer(p)
		mn.logger.Debug("Updated existing peer", "peer_id", peer.ID)
	} else {
		peer.LastSeen = time.Now().Unix()
		mn.topology.ConnectedPeers[peer.ID] = peer
		_ = mn.updateRoutingForPeer(peer)
		mn.logger.Info("Added new peer", "peer_id", peer.ID)
	}
}

// UpdatePeerLatency обновляет латентность пира
func (mn *MeshNetwork) UpdatePeerLatency(peerID string, latency int64) {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	if p, ok := mn.topology.ConnectedPeers[peerID]; ok {
		p.Latency = latency
		mn.router.mu.Lock()
		mn.router.latencyTable[peerID] = latency
		mn.router.mu.Unlock()
		mn.logger.Debug("Updated peer latency", "peer_id", peerID, "latency", latency)
	}
}

// RemovePeer удаляет пира из mesh сети
func (mn *MeshNetwork) RemovePeer(peerID string) error {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	if _, exists := mn.topology.ConnectedPeers[peerID]; !exists {
		return fmt.Errorf("peer not found: %s", peerID)
	}

	delete(mn.topology.ConnectedPeers, peerID)
	mn.removeRoutingForPeer(peerID)
	mn.logger.Info("Peer removed from mesh", "peer_id", peerID)
	return nil
}

// GetOptimalRoute returns the optimal route to a destination
func (mn *MeshNetwork) GetOptimalRoute(destination string) ([]string, error) {
	mn.router.mu.RLock()
	defer mn.router.mu.RUnlock()

	if route, exists := mn.router.routingTable[destination]; exists {
		// Return a copy to avoid race conditions
		routeCopy := make([]string, len(route))
		copy(routeCopy, route)
		return routeCopy, nil
	}

	return nil, fmt.Errorf("no route found to destination: %s", destination)
}

// GetRouteLatency returns the latency to a destination
func (mn *MeshNetwork) GetRouteLatency(destination string) (int64, error) {
	mn.router.mu.RLock()
	defer mn.router.mu.RUnlock()

	if latency, exists := mn.router.latencyTable[destination]; exists {
		return latency, nil
	}

	return 0, fmt.Errorf("no latency information for destination: %s", destination)
}

// initializeRouting initializes the routing based on mesh configuration
func (mn *MeshNetwork) initializeRouting() error {
	mn.logger.Info("Initializing mesh routing", "strategy", mn.config.Routing)

	switch mn.config.Routing {
	case "hybrid":
		return mn.initializeHybridRouting()
	case "direct":
		return mn.initializeDirectRouting()
	case "relay":
		return mn.initializeRelayRouting()
	default:
		return fmt.Errorf("unsupported routing strategy: %s", mn.config.Routing)
	}
}

// initializeHybridRouting initializes hybrid routing (direct + relay)
func (mn *MeshNetwork) initializeHybridRouting() error {
	mn.logger.Info("Initializing hybrid routing")

	// Hybrid routing tries direct connections first, falls back to relay
	// This is the default and most flexible approach
	return nil
}

// initializeDirectRouting initializes direct routing (peer-to-peer only)
func (mn *MeshNetwork) initializeDirectRouting() error {
	mn.logger.Info("Initializing direct routing")

	// Direct routing only uses direct peer-to-peer connections
	// No relay fallback
	return nil
}

// initializeRelayRouting initializes relay routing (through relay server)
func (mn *MeshNetwork) initializeRelayRouting() error {
	mn.logger.Info("Initializing relay routing")

	// Relay routing uses the relay server as an intermediary
	// All traffic goes through the relay
	return nil
}

// updateRoutingForPeer updates routing table when a peer is added
func (mn *MeshNetwork) updateRoutingForPeer(peer *Peer) error {
	mn.router.mu.Lock()
	defer mn.router.mu.Unlock()

	// Add direct route to the peer
	mn.router.routingTable[peer.ID] = []string{peer.ID}
	mn.router.latencyTable[peer.ID] = peer.Latency
	mn.router.ownerIndex[peer.ID] = peer.ID

	// Add routes to the peer's allowed IPs
	for _, allowedIP := range peer.AllowedIPs {
		mn.router.routingTable[allowedIP] = []string{peer.ID}
		mn.router.latencyTable[allowedIP] = peer.Latency
		mn.router.ownerIndex[allowedIP] = peer.ID
	}

	mn.logger.Debug("Updated routing for peer", "peer_id", peer.ID, "allowed_ips", peer.AllowedIPs)
	return nil
}

// removeRoutingForPeer removes routing entries when a peer is removed (O(1) через ownerIndex)
func (mn *MeshNetwork) removeRoutingForPeer(peerID string) {
	mn.router.mu.Lock()
	defer mn.router.mu.Unlock()

	// Remove direct route to the peer
	delete(mn.router.routingTable, peerID)
	delete(mn.router.latencyTable, peerID)
	delete(mn.router.ownerIndex, peerID)

	// Remove all destinations owned by this peer (быстро через ownerIndex)
	for dest, owner := range mn.router.ownerIndex {
		if owner == peerID {
			delete(mn.router.routingTable, dest)
			delete(mn.router.latencyTable, dest)
			delete(mn.router.ownerIndex, dest)
		}
	}

	mn.logger.Debug("Removed routing for peer", "peer_id", peerID)
}

// topologyUpdateLoop continuously updates the mesh topology
func (mn *MeshNetwork) topologyUpdateLoop() {
	ticker := time.NewTicker(mn.config.TopologyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mn.ctx.Done():
			return
		case <-ticker.C:
			mn.updateTopology()
		}
	}
}

// routingUpdateLoop continuously updates routing information
func (mn *MeshNetwork) routingUpdateLoop() {
	ticker := time.NewTicker(mn.config.RoutingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mn.ctx.Done():
			return
		case <-ticker.C:
			mn.updateRouting()
		}
	}
}

// healthCheckLoop performs health checks on mesh connections
func (mn *MeshNetwork) healthCheckLoop() {
	ticker := time.NewTicker(mn.config.HealthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mn.ctx.Done():
			return
		case <-ticker.C:
			mn.performHealthChecks()
		}
	}
}

// updateTopology updates the mesh topology information
func (mn *MeshNetwork) updateTopology() {
	mn.mu.Lock()
	defer mn.mu.Unlock()

	stale := mn.config.PeerStaleAfter
	now := time.Now()

	// Update topology based on current peer status
	for id, peer := range mn.topology.ConnectedPeers {
		// Check if peer is still responsive
		if time.Since(time.Unix(peer.LastSeen, 0)) > stale {
			if peer.IsConnected {
				peer.IsConnected = false
				mn.logger.Warn("Peer became stale", "peer_id", id, "since", now.Sub(time.Unix(peer.LastSeen, 0)))
			}
		}
	}

	mn.logger.Debug("Updated mesh topology", "connected_peers", len(mn.topology.ConnectedPeers))
}

// updateRouting updates the routing table based on current topology
func (mn *MeshNetwork) updateRouting() {
	mn.router.mu.Lock()
	defer mn.router.mu.Unlock()

	// Recalculate optimal routes based on current peer status
	for dest, route := range mn.router.routingTable {
		if len(route) > 0 {
			peerID := route[0]
			if peer, exists := mn.topology.ConnectedPeers[peerID]; exists {
				if !peer.IsConnected {
					// Find alternative route
					if altRoute := mn.findAlternativeRoute(dest, peerID); altRoute != nil {
						mn.router.routingTable[dest] = altRoute
						mn.logger.Info("Updated route due to peer unavailability", "destination", dest, "new_route", altRoute)
					}
				}
			}
		}
	}

	mn.logger.Debug("Updated mesh routing table", "routes", len(mn.router.routingTable))
}

// performHealthChecks performs health checks on all mesh connections
func (mn *MeshNetwork) performHealthChecks() {
	mn.mu.RLock()
	peers := make(map[string]*Peer, len(mn.topology.ConnectedPeers))
	for id, p := range mn.topology.ConnectedPeers {
		peers[id] = p
	}
	mn.mu.RUnlock()

	dead := mn.config.PeerDeadAfter
	for id, peer := range peers {
		// Perform health check with configurable threshold
		if time.Since(time.Unix(peer.LastSeen, 0)) > dead {
			mn.logger.Warn("Peer failed health check", "peer_id", id, "since", time.Since(time.Unix(peer.LastSeen, 0)))
			// Опционально: автоснос
			// _ = mn.RemovePeer(id)
		}
	}
}

// ipInCIDR проверяет, попадает ли IP в CIDR
func ipInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return n.Contains(ip)
}

// IsIPInTenantSubnet проверяет, принадлежит ли IP к тенантной подсети
func (mn *MeshNetwork) IsIPInTenantSubnet(ip string, tenantSubnet string) bool {
	if tenantSubnet == "" {
		return false
	}
	return ipInCIDR(ip, tenantSubnet)
}

// findAlternativeRoute finds an alternative route to a destination (CIDR-совместимый)
func (mn *MeshNetwork) findAlternativeRoute(destination, excludePeerID string) []string {
	// Если destination — IP, попробуем найти владельца по CIDR
	for id, peer := range mn.topology.ConnectedPeers {
		if id == excludePeerID || !peer.IsConnected {
			continue
		}
		for _, allowed := range peer.AllowedIPs {
			// точное совпадение
			if allowed == destination {
				return []string{id}
			}
			// CIDR
			if strings.Contains(allowed, "/") && ipInCIDR(destination, allowed) {
				return []string{id}
			}
		}
	}
	return nil
}

// GetMeshStats returns statistics about the mesh network
func (mn *MeshNetwork) GetMeshStats() map[string]interface{} {
	mn.mu.RLock()
	defer mn.mu.RUnlock()

	connectedCount := 0
	totalLatency := int64(0)

	for _, peer := range mn.topology.ConnectedPeers {
		if peer.IsConnected {
			connectedCount++
			totalLatency += peer.Latency
		}
	}

	avgLatency := int64(0)
	if connectedCount > 0 {
		avgLatency = totalLatency / int64(connectedCount)
	}

	return map[string]interface{}{
		"total_peers":      len(mn.topology.ConnectedPeers),
		"connected_peers":  connectedCount,
		"routing_strategy": mn.config.Routing,
		"encryption":       mn.config.Encryption,
		"average_latency":  avgLatency,
		"routes":           len(mn.router.routingTable),
	}
}

// validateConfig валидирует конфигурацию mesh сети
func (mn *MeshNetwork) validateConfig() error {
	// Проверяем routing тип
	switch mn.config.Routing {
	case RoutingHybrid, RoutingDirect, RoutingRelay, "":
		// ok
	default:
		return fmt.Errorf("invalid routing type: %q", mn.config.Routing)
	}

	// Проверяем encryption тип
	switch mn.config.Encryption {
	case EncryptionQUIC, EncryptionTLS, "":
		// ok
	default:
		return fmt.Errorf("invalid encryption type: %q", mn.config.Encryption)
	}

	// Проверяем тайминги
	if mn.config.TopologyInterval <= 0 {
		return fmt.Errorf("topology interval must be positive: %v", mn.config.TopologyInterval)
	}
	if mn.config.RoutingInterval <= 0 {
		return fmt.Errorf("routing interval must be positive: %v", mn.config.RoutingInterval)
	}
	if mn.config.HealthInterval <= 0 {
		return fmt.Errorf("health interval must be positive: %v", mn.config.HealthInterval)
	}

	return nil
}
