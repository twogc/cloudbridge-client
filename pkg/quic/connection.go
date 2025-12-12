package quic

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// QUICConnection manages QUIC connections and streams
type QUICConnection struct {
	conn *quic.Conn

	streams map[string]*quic.Stream
	mu      sync.RWMutex

	logger Logger

	// QUIC config
	qcfg *quic.Config

	// TLS configs are split for client/server use-cases
	tlsClient *tls.Config
	tlsServer *tls.Config

	// lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// handler
	streamHandler func(stream *quic.Stream)
}

// Logger interface for QUIC connection logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
}

// NewQUICConnection creates a new QUIC connection manager
func NewQUICConnection(logger Logger) *QUICConnection {
	ctx, cancel := context.WithCancel(context.Background())

	q := &QUICConnection{
		streams: make(map[string]*quic.Stream),
		logger:  logger,
		qcfg: &quic.Config{
			HandshakeIdleTimeout:  10 * time.Second,
			MaxIdleTimeout:        30 * time.Second,
			MaxIncomingStreams:    1024,
			MaxIncomingUniStreams: 1024,
			KeepAlivePeriod:       15 * time.Second,
			// Другие поля по мере необходимости
		},
		tlsClient: &tls.Config{
			// Для клиента: зададим ALPN, а ServerName можно установить через SetClientServerName
			MinVersion:         tls.VersionTLS13,
			InsecureSkipVerify: false, // по умолчанию верифицируем
			NextProtos:         []string{"cloudbridge-p2p", "h3"},
		},
		tlsServer: &tls.Config{
			// Для сервера: ОБЯЗАТЕЛЬНО наличие Certificates, задается через SetServerTLSCert/SetServerTLSConfig
			MinVersion: tls.VersionTLS13,
			NextProtos: []string{"cloudbridge-p2p", "h3"},
		},
		ctx:    ctx,
		cancel: cancel,
	}
	return q
}

// -------- configuration setters --------

// SetClientTLSConfig fully overrides the client TLS config
func (q *QUICConnection) SetClientTLSConfig(cfg *tls.Config) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsClient = cfg
}

// SetServerTLSConfig fully overrides the server TLS config
func (q *QUICConnection) SetServerTLSConfig(cfg *tls.Config) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsServer = cfg
}

// SetServerTLSCert sets certificate+key for server listening
func (q *QUICConnection) SetServerTLSCert(cert tls.Certificate) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsServer.Certificates = []tls.Certificate{cert}
}

// SetClientServerName sets SNI / hostname verification target
func (q *QUICConnection) SetClientServerName(serverName string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsClient.ServerName = serverName
}

// SetInsecureSkipVerify (client-side) for debugging only
func (q *QUICConnection) SetInsecureSkipVerify(insecure bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsClient.InsecureSkipVerify = insecure
}

// SetALPN sets ALPN protocols (both client and server)
func (q *QUICConnection) SetALPN(protocols ...string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tlsClient.NextProtos = append([]string{}, protocols...)
	q.tlsServer.NextProtos = append([]string{}, protocols...)
}

// SetQUICConfig overrides quic.Config
func (q *QUICConnection) SetQUICConfig(cfg *quic.Config) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.qcfg = cfg
}

// SetStreamHandler sets the handler for incoming streams
func (q *QUICConnection) SetStreamHandler(handler func(stream *quic.Stream)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.streamHandler = handler
}

// -------- client side --------

// Connect establishes a QUIC connection to the specified address
func (q *QUICConnection) Connect(ctx context.Context, addr string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.tlsClient.ServerName == "" && !q.tlsClient.InsecureSkipVerify {
		// Верификация включена, но SNI не задан — это почти наверняка приведет к ошибке.
		return fmt.Errorf("TLS client ServerName is empty; set it or enable InsecureSkipVerify (not recommended)")
	}

	q.logger.Info("Connecting to QUIC server", "address", addr, "alpn", q.tlsClient.NextProtos)

	conn, err := quic.DialAddr(ctx, addr, q.tlsClient, q.qcfg)
	if err != nil {
		return fmt.Errorf("failed to connect to QUIC server: %w", err)
	}

	q.conn = conn
	q.logger.Info("QUIC connection established", "address", addr)

	// Start connection monitoring bound to lifecycle context
	go q.monitorConnection(q.ctx)

	return nil
}

// -------- server side --------

// Listen starts listening for incoming QUIC connections on addr (e.g., ":5553")
func (q *QUICConnection) Listen(ctx context.Context, addr string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tlsServer.Certificates) == 0 {
		return fmt.Errorf("server TLS config has no certificates; set via SetServerTLSCert/SetServerTLSConfig")
	}

	q.logger.Info("Starting QUIC listener", "address", addr, "alpn", q.tlsServer.NextProtos)

	listener, err := quic.ListenAddr(addr, q.tlsServer, q.qcfg)
	if err != nil {
		return fmt.Errorf("failed to start QUIC listener: %w", err)
	}

	q.logger.Info("QUIC listener started", "address", addr)

	// Accept connections
	go q.acceptConnections(ctx, listener)

	return nil
}

// -------- streams --------

// CreateStream creates a new bidirectional stream
func (q *QUICConnection) CreateStream(ctx context.Context, streamID string) (*quic.Stream, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.conn == nil {
		return nil, fmt.Errorf("QUIC connection not established")
	}

	stream, err := q.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	q.mu.RUnlock()
	q.mu.Lock()
	q.streams[streamID] = stream
	q.mu.Unlock()
	q.mu.RLock()

	q.logger.Debug("Stream created", "stream_id", streamID)

	return stream, nil
}

// AcceptStream accepts an incoming stream on the current connection
func (q *QUICConnection) AcceptStream(ctx context.Context) (*quic.Stream, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.conn == nil {
		return nil, fmt.Errorf("QUIC connection not established")
	}

	stream, err := q.conn.AcceptStream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to accept stream: %w", err)
	}

	streamID := fmt.Sprintf("stream_%d", stream.StreamID())
	q.mu.RUnlock()
	q.mu.Lock()
	q.streams[streamID] = stream
	q.mu.Unlock()
	q.mu.RLock()

	q.logger.Debug("Stream accepted", "stream_id", streamID)

	return stream, nil
}

// GetStream returns a stream by ID
func (q *QUICConnection) GetStream(streamID string) (*quic.Stream, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	s, ok := q.streams[streamID]
	return s, ok
}

// CloseStream closes a stream
func (q *QUICConnection) CloseStream(streamID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	stream, exists := q.streams[streamID]
	if !exists {
		return fmt.Errorf("stream not found: %s", streamID)
	}

	if err := stream.Close(); err != nil {
		q.logger.Error("Failed to close stream", "stream_id", streamID, "error", err)
		// продолжаем cleanup
	}
	delete(q.streams, streamID)
	q.logger.Debug("Stream closed", "stream_id", streamID)
	return nil
}

// -------- lifecycle / stats --------

// Close closes the QUIC connection and all streams
func (q *QUICConnection) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.conn == nil {
		return nil
	}

	q.logger.Info("Closing QUIC connection")

	// stop background routines
	q.cancel()

	// Close all streams
	for id, s := range q.streams {
		if err := s.Close(); err != nil {
			q.logger.Error("Failed to close stream", "stream_id", id, "error", err)
		}
		delete(q.streams, id)
	}

	// Close connection
	if err := q.conn.CloseWithError(0, "client shutdown"); err != nil {
		q.logger.Error("Failed to close QUIC connection", "error", err)
		// все равно обнулим
	}
	q.conn = nil

	q.logger.Info("QUIC connection closed")
	return nil
}

// IsConnected returns true if the connection is active
func (q *QUICConnection) IsConnected() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.conn != nil
}

// GetConnectionState returns the connection state
func (q *QUICConnection) GetConnectionState() quic.ConnectionState {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.conn == nil {
		return quic.ConnectionState{}
	}
	return q.conn.ConnectionState()
}

// GetStats returns connection statistics
func (q *QUICConnection) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()

	stats := map[string]interface{}{
		"connected":    q.conn != nil,
		"stream_count": len(q.streams),
	}

	if q.conn != nil {
		state := q.conn.ConnectionState()
		stats["tls_handshake_complete"] = state.TLS.HandshakeComplete
		stats["peer_certificates"] = len(state.TLS.PeerCertificates)
		stats["version"] = state.Version
	}

	return stats
}

// monitorConnection monitors the connection state
func (q *QUICConnection) monitorConnection(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.mu.RLock()
			connected := q.conn != nil
			q.mu.RUnlock()

			if !connected {
				q.logger.Warn("QUIC connection lost")
				return
			}

			stats := q.GetStats()
			q.logger.Debug("QUIC connection stats", "stats", stats)
		}
	}
}

// acceptConnections accepts incoming connections
func (q *QUICConnection) acceptConnections(ctx context.Context, listener *quic.Listener) {
	defer func() {
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept(ctx)
		if err != nil {
			// context canceled or real error
			select {
			case <-ctx.Done():
				q.logger.Info("Stopping QUIC listener")
				return
			default:
			}
			q.logger.Error("Failed to accept QUIC connection", "error", err)
			continue
		}

		q.logger.Info("QUIC connection accepted", "remote_addr", conn.RemoteAddr())
		go q.handleIncomingConnection(conn)
	}
}

// handleIncomingConnection handles an incoming QUIC connection
func (q *QUICConnection) handleIncomingConnection(conn *quic.Conn) {
	defer func() {
		_ = conn.CloseWithError(0, "connection closed") // cleanup
	}()

	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil {
			q.logger.Error("Failed to accept stream from incoming connection", "error", err)
			return
		}

		streamID := fmt.Sprintf("incoming_%d", stream.StreamID())
		q.mu.Lock()
		q.streams[streamID] = stream
		q.mu.Unlock()

		q.logger.Debug("Incoming stream accepted", "stream_id", streamID)

		go q.handleStream(streamID, stream)
	}
}

// handleStream handles a stream
func (q *QUICConnection) handleStream(streamID string, stream *quic.Stream) {
	q.mu.RLock()
	handler := q.streamHandler
	q.mu.RUnlock()

	if handler != nil {
		// Use custom handler
		handler(stream)
		
		// Cleanup after handler returns
		q.mu.Lock()
		delete(q.streams, streamID)
		q.mu.Unlock()
		return
	}

	// Default echo behavior
	defer func() {
		_ = stream.Close()
		q.mu.Lock()
		delete(q.streams, streamID)
		q.mu.Unlock()
	}()

	buf := make([]byte, 4096)
	for {
		n, err := stream.Read(buf)
		if err != nil {
			q.logger.Debug("Stream read error", "stream_id", streamID, "error", err)
			return
		}

		q.logger.Debug("Data received on stream", "stream_id", streamID, "bytes", n)

		if _, err := stream.Write(buf[:n]); err != nil {
			q.logger.Error("Failed to write to stream", "stream_id", streamID, "error", err)
			return
		}
	}
}
