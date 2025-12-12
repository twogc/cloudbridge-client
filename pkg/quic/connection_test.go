package quic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

// mockLogger implements Logger interface for testing
type mockLogger struct {
	infoMsgs  []string
	errorMsgs []string
	debugMsgs []string
	warnMsgs  []string
}

func (m *mockLogger) Info(msg string, fields ...interface{}) {
	m.infoMsgs = append(m.infoMsgs, msg)
}

func (m *mockLogger) Error(msg string, fields ...interface{}) {
	m.errorMsgs = append(m.errorMsgs, msg)
}

func (m *mockLogger) Debug(msg string, fields ...interface{}) {
	m.debugMsgs = append(m.debugMsgs, msg)
}

func (m *mockLogger) Warn(msg string, fields ...interface{}) {
	m.warnMsgs = append(m.warnMsgs, msg)
}

// generateTestCert creates a self-signed certificate for testing
func generateTestCert() (tls.Certificate, error) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
			CommonName:   "localhost",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	// Convert to PEM
	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	return cert, nil
}

func TestNewQUICConnection(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	if conn == nil {
		t.Fatal("Expected non-nil QUICConnection")
	}

	if conn.logger != logger {
		t.Error("Expected logger to be set")
	}

	if conn.streams == nil {
		t.Error("Expected streams map to be initialized")
	}

	if conn.tlsClient == nil {
		t.Error("Expected tlsClient to be initialized")
	}

	if conn.tlsServer == nil {
		t.Error("Expected tlsServer to be initialized")
	}

	if conn.qcfg == nil {
		t.Error("Expected qcfg to be initialized")
	}
}

func TestSetClientServerName(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	serverName := "example.com"
	conn.SetClientServerName(serverName)

	if conn.tlsClient.ServerName != serverName {
		t.Errorf("Expected ServerName to be %s, got %s", serverName, conn.tlsClient.ServerName)
	}
}

func TestSetInsecureSkipVerify(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	conn.SetInsecureSkipVerify(true)

	if !conn.tlsClient.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

func TestSetALPN(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	protocols := []string{"test-protocol", "h3"}
	conn.SetALPN(protocols...)

	// Check client ALPN
	if len(conn.tlsClient.NextProtos) != len(protocols) {
		t.Errorf("Expected %d protocols in client, got %d", len(protocols), len(conn.tlsClient.NextProtos))
	}

	// Check server ALPN
	if len(conn.tlsServer.NextProtos) != len(protocols) {
		t.Errorf("Expected %d protocols in server, got %d", len(protocols), len(conn.tlsServer.NextProtos))
	}
}

func TestSetServerTLSCert(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	cert, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	conn.SetServerTLSCert(cert)

	if len(conn.tlsServer.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(conn.tlsServer.Certificates))
	}
}

func TestConnectWithoutServerName(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	ctx := context.Background()
	err := conn.Connect(ctx, "localhost:8080")

	if err == nil {
		t.Error("Expected error when connecting without ServerName")
	}

	expectedMsg := "TLS client ServerName is empty; set it or enable InsecureSkipVerify (not recommended)"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestConnectWithInsecureSkipVerify(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Enable insecure mode
	conn.SetInsecureSkipVerify(true)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should fail quickly due to timeout, not due to ServerName error
	err := conn.Connect(ctx, "localhost:8080")

	// Should not be the ServerName error
	if err != nil && err.Error() == "TLS client ServerName is empty; set it or enable InsecureSkipVerify (not recommended)" {
		t.Error("Expected connection attempt, not ServerName error")
	}
}

func TestListenWithoutCertificates(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	ctx := context.Background()
	err := conn.Listen(ctx, ":8080")

	if err == nil {
		t.Error("Expected error when listening without certificates")
	}

	expectedMsg := "server TLS config has no certificates; set via SetServerTLSCert/SetServerTLSConfig"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestListenWithCertificates(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	cert, err := generateTestCert()
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	conn.SetServerTLSCert(cert)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should fail quickly due to timeout, not due to certificate error
	err = conn.Listen(ctx, ":8080")

	// Should not be the certificate error
	if err != nil && err.Error() == "server TLS config has no certificates; set via SetServerTLSCert/SetServerTLSConfig" {
		t.Error("Expected listener attempt, not certificate error")
	}
}

func TestIsConnected(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Initially not connected
	if conn.IsConnected() {
		t.Error("Expected connection to be initially disconnected")
	}
}

func TestGetStats(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	stats := conn.GetStats()

	if stats["connected"].(bool) {
		t.Error("Expected connection to be initially disconnected")
	}

	if stats["stream_count"].(int) != 0 {
		t.Error("Expected stream count to be 0")
	}
}

func TestClose(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Close should not error when not connected
	err := conn.Close()
	if err != nil {
		t.Errorf("Expected no error when closing disconnected connection, got: %v", err)
	}
}

func TestSetClientTLSConfig(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	newConfig := &tls.Config{
		ServerName: "test.example.com",
		MinVersion: tls.VersionTLS12,
	}

	conn.SetClientTLSConfig(newConfig)

	if conn.tlsClient != newConfig {
		t.Error("Expected tlsClient to be set to new config")
	}
}

func TestSetServerTLSConfig(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	newConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	conn.SetServerTLSConfig(newConfig)

	if conn.tlsServer != newConfig {
		t.Error("Expected tlsServer to be set to new config")
	}
}

func TestSetQUICConfig(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	newConfig := &quic.Config{
		HandshakeIdleTimeout: 5 * time.Second,
		MaxIdleTimeout:       10 * time.Second,
	}

	conn.SetQUICConfig(newConfig)

	if conn.qcfg != newConfig {
		t.Error("Expected qcfg to be set to new config")
	}
}

func TestGetConnectionState(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Should return empty state when not connected
	state := conn.GetConnectionState()
	if state.Version != 0 {
		t.Error("Expected empty connection state when not connected")
	}
}

func TestStreamOperations(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Test operations on disconnected connection
	_, err := conn.CreateStream(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when creating stream on disconnected connection")
	}

	_, err = conn.AcceptStream(context.Background())
	if err == nil {
		t.Error("Expected error when accepting stream on disconnected connection")
	}

	// Test stream operations
	stream, exists := conn.GetStream("nonexistent")
	if exists {
		t.Error("Expected stream to not exist")
	}
	if stream != nil {
		t.Error("Expected stream to be nil")
	}

	err = conn.CloseStream("nonexistent")
	if err == nil {
		t.Error("Expected error when closing nonexistent stream")
	}
}

func TestContextCancellation(t *testing.T) {
	logger := &mockLogger{}
	conn := NewQUICConnection(logger)

	// Test that context cancellation works
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connection should be closed when context is canceled
	select {
	case <-conn.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		// Context is not canceled yet, which is expected since we just created it
		// This test is more about ensuring the context exists and can be canceled
	}
}
