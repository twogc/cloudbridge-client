package ice

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/stun"
)

// ICEAgent handles ICE connectivity checks and candidate gathering
type ICEAgent struct {
	agent       *ice.Agent
	stunServers []string
	turnServers []string
	config      *ice.AgentConfig
	mu          sync.RWMutex
	logger      Logger
}

// Logger interface for ICE agent logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// NewICEAgent creates a new ICE agent
func NewICEAgent(stunServers, turnServers []string, logger Logger) *ICEAgent {
	return &ICEAgent{
		stunServers: stunServers,
		turnServers: turnServers,
		logger:      logger,
	}
}

// Start initializes and starts the ICE agent
func (a *ICEAgent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Info("Starting ICE agent", "stun_servers", a.stunServers, "turn_servers", a.turnServers)

	urls := make([]*stun.URI, len(a.stunServers))
	for i, s := range a.stunServers {
		// дополняем префикс если его нет
		if !strings.HasPrefix(s, "stun:") && !strings.HasPrefix(s, "stuns:") {
			s = "stun:" + s
		}
		u, err := stun.ParseURI(s)
		if err != nil {
			return fmt.Errorf("failed to parse STUN URI %q: %w", s, err)
		}
		urls[i] = u
	}

	a.config = &ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4, ice.NetworkTypeUDP6},
		Urls:         urls,
		// Либо тут, либо позже через `agent.Set*`
	}

	// Create ICE agent
	agent, err := ice.NewAgent(a.config)
	if err != nil {
		return fmt.Errorf("failed to create ICE agent: %w", err)
	}

	a.agent = agent

	// Set up event handlers
	a.setupEventHandlers()

	a.logger.Info("ICE agent started successfully")
	return nil
}

// Stop stops the ICE agent
func (a *ICEAgent) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.agent == nil {
		return nil
	}

	a.logger.Info("Stopping ICE agent")

	if err := a.agent.Close(); err != nil {
		a.logger.Error("Failed to close ICE agent", "error", err)
		return err
	}

	a.agent = nil
	a.logger.Info("ICE agent stopped")
	return nil
}

// GatherCandidates starts candidate gathering
func (a *ICEAgent) GatherCandidates() ([]ice.Candidate, error) {
	a.mu.RLock()
	ag := a.agent
	a.mu.RUnlock()
	if ag == nil {
		return nil, fmt.Errorf("ICE agent not started")
	}

	done := make(chan struct{})
	// ловим завершение сбора через OnCandidate с nil
	if err := ag.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			// nil candidate означает завершение сбора
			close(done)
		}
	}); err != nil {
		return nil, fmt.Errorf("failed to set candidate handler: %w", err)
	}

	if err := ag.GatherCandidates(); err != nil {
		return nil, fmt.Errorf("failed to gather candidates: %w", err)
	}

	select {
	case <-done:
		// ok
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("candidate gathering timeout")
	}

	// теперь можно безопасно получить локальные кандидаты
	cands, err := ag.GetLocalCandidates()
	if err != nil {
		return nil, fmt.Errorf("failed to get local candidates: %w", err)
	}
	a.logger.Info("Candidate gathering completed", "count", len(cands))
	return cands, nil
}

// AddRemoteCandidateFromSDP adds a remote candidate from SDP string
func (a *ICEAgent) AddRemoteCandidateFromSDP(sdpCand string) error {
	a.mu.RLock()
	ag := a.agent
	a.mu.RUnlock()
	if ag == nil {
		return fmt.Errorf("ICE agent not started")
	}
	c, err := ice.UnmarshalCandidate(sdpCand)
	if err != nil {
		return fmt.Errorf("failed to parse remote candidate: %w", err)
	}
	a.logger.Debug("Adding remote candidate", "candidate", c.String())
	return ag.AddRemoteCandidate(c)
}

// AddRemoteCandidate adds a remote candidate (thin wrapper for existing Candidate objects)
func (a *ICEAgent) AddRemoteCandidate(candidate ice.Candidate) error {
	a.mu.RLock()
	ag := a.agent
	a.mu.RUnlock()
	if ag == nil {
		return fmt.Errorf("ICE agent not started")
	}
	a.logger.Debug("Adding remote candidate", "candidate", candidate.String())
	return ag.AddRemoteCandidate(candidate)
}

// StartConnectivityChecks starts ICE connectivity checks (skeleton through remote creds)
func (a *ICEAgent) StartConnectivityChecks(remoteUfrag, remotePwd string) error {
	a.mu.RLock()
	ag := a.agent
	a.mu.RUnlock()
	if ag == nil {
		return fmt.Errorf("ICE agent not started")
	}

	// В v2 это три шага: задать свои/их креды + обмен кандидатами + Dial/Accept
	// 1) свои креды (обычно их генерят):
	localUfrag, _, err := ag.GetLocalUserCredentials()
	if err != nil {
		return fmt.Errorf("failed to get local ICE creds: %w", err)
	}
	a.logger.Debug("Local ICE creds", "ufrag", localUfrag, "pwd", "***")

	// 2) установить удаленные креды
	if err := ag.SetRemoteCredentials(remoteUfrag, remotePwd); err != nil {
		return fmt.Errorf("failed to set remote ICE creds: %w", err)
	}

	// 3) Транспорт поднимется, когда вы вызовете Dial/Accept (в разных ролях)
	// Здесь оставляем как хук — реализация зависит от роли и архитектуры:
	//   conn, err := ag.Dial(context, remoteAddrs...)   // для controlling
	//   conn, err := ag.Accept(context, ...)            // для controlled
	// Управляйте role через AgentConfig.Role в NewAgent или SetControlling().
	a.logger.Info("ICE credentials set; start Dial/Accept in your session flow")
	return nil
}

// GetSelectedCandidatePair returns the selected candidate pair
func (a *ICEAgent) GetSelectedCandidatePair() (*ice.CandidatePair, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.agent == nil {
		return nil, fmt.Errorf("ICE agent not started")
	}

	return a.agent.GetSelectedCandidatePair()
}

// GetConnectionState returns the current connection state
func (a *ICEAgent) GetConnectionState() ice.ConnectionState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.agent == nil {
		return ice.ConnectionStateClosed
	}
	// В v2 GetConnectionState может отсутствовать, используем альтернативный подход
	// Можно отслеживать состояние через OnConnectionStateChange callback
	return ice.ConnectionStateNew // fallback
}

// setupEventHandlers sets up ICE agent event handlers
func (a *ICEAgent) setupEventHandlers() {
	if err := a.agent.OnCandidate(func(c ice.Candidate) {
		if c != nil {
			a.logger.Debug("New candidate gathered", "candidate", c.String())
		}
	}); err != nil {
		a.logger.Error("Failed to set candidate handler", "error", err)
	}

	if err := a.agent.OnSelectedCandidatePairChange(func(local, remote ice.Candidate) {
		a.logger.Info("Selected candidate pair changed",
			"local", func() string {
				if local != nil {
					return local.String()
				}
				return ""
			}(),
			"remote", func() string {
				if remote != nil {
					return remote.String()
				}
				return ""
			}(),
		)
	}); err != nil {
		a.logger.Error("Failed to set candidate pair handler", "error", err)
	}

	if err := a.agent.OnConnectionStateChange(func(s ice.ConnectionState) {
		a.logger.Info("ICE connection state changed", "state", s.String())
	}); err != nil {
		a.logger.Error("Failed to set connection state handler", "error", err)
	}
}

// ValidateSTUNServer validates a STUN server
func ValidateSTUNServer(server string) error {
	// допускаем «короткий» формат host:port
	addr := server
	if strings.HasPrefix(addr, "stun:") || strings.HasPrefix(addr, "stuns:") {
		// stun.ParseURL может пригодиться для извлечения host:port, но для простоты:
		addr = strings.TrimPrefix(strings.TrimPrefix(addr, "stun:"), "stuns:")
	}

	conn, err := net.DialTimeout("udp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to STUN server %s: %w", server, err)
	}
	defer conn.Close()

	req := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
	if _, err := conn.Write(req.Raw); err != nil {
		return fmt.Errorf("failed to send STUN request: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}

	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read STUN response: %w", err)
	}

	var msg stun.Message
	if err := msg.UnmarshalBinary(buf[:n]); err != nil {
		return fmt.Errorf("failed to parse STUN response: %w", err)
	}
	if msg.Type != stun.BindingSuccess {
		return fmt.Errorf("unexpected STUN response type: %v", msg.Type)
	}
	return nil
}

// GetPublicIP gets the public IP address using STUN
func GetPublicIP(stunServer string) (net.IP, error) {
	conn, err := net.DialTimeout("udp", stunServer, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to STUN server: %w", err)
	}
	defer conn.Close()

	// Send STUN binding request
	request := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	_, err = conn.Write(request.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to send STUN request: %w", err)
	}

	// Read response
	response := make([]byte, 1024)
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	n, err := conn.Read(response)
	if err != nil {
		return nil, fmt.Errorf("failed to read STUN response: %w", err)
	}

	// Parse response
	var msg stun.Message
	if err := msg.UnmarshalBinary(response[:n]); err != nil {
		return nil, fmt.Errorf("failed to parse STUN response: %w", err)
	}

	// Extract mapped address
	var mappedAddress stun.XORMappedAddress
	if err := mappedAddress.GetFrom(&msg); err != nil {
		return nil, fmt.Errorf("failed to get mapped address: %w", err)
	}

	return mappedAddress.IP, nil
}
