package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/relay/transport/proto"
	"github.com/twogc/cloudbridge-client/pkg/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// mockGRPCServer implements the gRPC services for testing
type mockGRPCServer struct {
	proto.UnimplementedControlServiceServer
	proto.UnimplementedTunnelServiceServer
	proto.UnimplementedHeartbeatServiceServer
}

func (s *mockGRPCServer) Hello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloResponse, error) {
	return &proto.HelloResponse{
		Status:            "ok",
		ServerVersion:     "1.0.0-test",
		SupportedFeatures: []string{"tls", "heartbeat", "tunnel_info", "grpc"},
		SessionId:         "test-session-123",
		Timestamp:         timestamppb.Now(),
	}, nil
}

func (s *mockGRPCServer) Authenticate(ctx context.Context, req *proto.AuthRequest) (*proto.AuthResponse, error) {
	return &proto.AuthResponse{
		Status:       "ok",
		ClientId:     "test-client-123",
		TenantId:     "test-tenant-1",
		SessionToken: "test-session-token",
		ExpiresAt:    timestamppb.New(time.Now().Add(24 * time.Hour)),
	}, nil
}

func (s *mockGRPCServer) CreateTunnel(ctx context.Context, req *proto.CreateTunnelRequest) (*proto.CreateTunnelResponse, error) {
	return &proto.CreateTunnelResponse{
		Status:   "ok",
		TunnelId: req.TunnelId,
		Endpoint: "grpc://test-endpoint",
		TunnelInfo: &proto.TunnelInfo{
			TunnelId:   req.TunnelId,
			TenantId:   req.TenantId,
			LocalPort:  req.LocalPort,
			RemoteHost: req.RemoteHost,
			RemotePort: req.RemotePort,
			Status:     "active",
			CreatedAt:  timestamppb.Now(),
		},
	}, nil
}

func (s *mockGRPCServer) SendHeartbeat(ctx context.Context, req *proto.HeartbeatRequest) (*proto.HeartbeatResponse, error) {
	return &proto.HeartbeatResponse{
		Status:          "ok",
		ServerTimestamp: timestamppb.Now(),
		IntervalSeconds: 30,
		ServerMetrics: &proto.ServerMetrics{
			ConnectedClients: 5,
			ActiveTunnels:    3,
			ServerLoad:       0.3,
			Timestamp:        timestamppb.Now(),
		},
	}, nil
}

// createTestGRPCServer creates a test gRPC server with bufconn
func createTestGRPCServer() (*grpc.Server, *bufconn.Listener) {
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	server := grpc.NewServer()
	mockServer := &mockGRPCServer{}

	proto.RegisterControlServiceServer(server, mockServer)
	proto.RegisterTunnelServiceServer(server, mockServer)
	proto.RegisterHeartbeatServiceServer(server, mockServer)

	go func() {
		if err := server.Serve(lis); err != nil {
			// Server stopped
		}
	}()

	return server, lis
}

// createTestGRPCClient creates a test gRPC client connected to bufconn
func createTestGRPCClient(lis *bufconn.Listener) (*grpc.ClientConn, error) {
	return grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}

// TestGRPCTransport_HelloAuthCreateTunnel tests the complete gRPC flow
func TestGRPCTransport_HelloAuthCreateTunnel(t *testing.T) {
	// Create test gRPC server
	server, lis := createTestGRPCServer()
	defer server.Stop()

	// Create test gRPC client connection
	conn, err := createTestGRPCClient(lis)
	if err != nil {
		t.Fatalf("Failed to create test gRPC client: %v", err)
	}
	defer conn.Close()

	// Create test config
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Port: 8443,
			TLS: types.TLSConfig{
				Enabled: false, // Using insecure for test
			},
		},
	}

	// Create test logger
	logger := newTestLogger()

	// Create GRPCClient with mock connection
	grpcClient := &GRPCClient{
		config:    config,
		conn:      conn,
		logger:    logger,
		connected: true,
	}

	// Create GRPCTransport
	transport := NewGRPCTransport(grpcClient, logger)

	// Test Hello
	helloResult, err := transport.Hello("1.0.0-test", []string{"tls", "heartbeat"})
	if err != nil {
		t.Fatalf("Hello failed: %v", err)
	}

	if helloResult.Status != "ok" {
		t.Errorf("Expected Hello status 'ok', got '%s'", helloResult.Status)
	}

	if helloResult.ServerVersion != "1.0.0-test" {
		t.Errorf("Expected server version '1.0.0-test', got '%s'", helloResult.ServerVersion)
	}

	if helloResult.SessionID != "test-session-123" {
		t.Errorf("Expected session ID 'test-session-123', got '%s'", helloResult.SessionID)
	}

	// Test Authenticate
	authResult, err := transport.Authenticate("test-jwt-token")
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	if authResult.Status != "ok" {
		t.Errorf("Expected Auth status 'ok', got '%s'", authResult.Status)
	}

	if authResult.ClientID != "test-client-123" {
		t.Errorf("Expected client ID 'test-client-123', got '%s'", authResult.ClientID)
	}

	if authResult.TenantID != "test-tenant-1" {
		t.Errorf("Expected tenant ID 'test-tenant-1', got '%s'", authResult.TenantID)
	}

	// Test CreateTunnel
	tunnelResult, err := transport.CreateTunnel("test-tunnel-1", "test-tenant-1", 8080, "192.168.1.100", 80)
	if err != nil {
		t.Fatalf("CreateTunnel failed: %v", err)
	}

	if tunnelResult.Status != "ok" {
		t.Errorf("Expected Tunnel status 'ok', got '%s'", tunnelResult.Status)
	}

	if tunnelResult.TunnelID != "test-tunnel-1" {
		t.Errorf("Expected tunnel ID 'test-tunnel-1', got '%s'", tunnelResult.TunnelID)
	}

	if tunnelResult.Endpoint != "grpc://test-endpoint" {
		t.Errorf("Expected endpoint 'grpc://test-endpoint', got '%s'", tunnelResult.Endpoint)
	}

	// Test SendHeartbeat
	metrics := &ClientMetrics{
		BytesSent:         1024,
		BytesReceived:     2048,
		ActiveTunnels:     1,
		ActiveP2PSessions: 0,
		TransportMode:     "grpc",
	}

	heartbeatResult, err := transport.SendHeartbeat("test-client-123", "test-tenant-1", metrics)
	if err != nil {
		t.Fatalf("SendHeartbeat failed: %v", err)
	}

	if heartbeatResult.Status != "ok" {
		t.Errorf("Expected Heartbeat status 'ok', got '%s'", heartbeatResult.Status)
	}

	if heartbeatResult.IntervalSeconds != 30 {
		t.Errorf("Expected interval 30 seconds, got %d", heartbeatResult.IntervalSeconds)
	}
}

// TestGRPCTransport_ConnectionStates tests connection state handling
func TestGRPCTransport_ConnectionStates(t *testing.T) {
	// Create test config
	config := &types.Config{
		Relay: types.RelayConfig{
			Host: "test.example.com",
			Port: 8443,
		},
	}

	// Create test logger
	logger := newTestLogger()

	// Create GRPCClient without connection
	grpcClient := &GRPCClient{
		config:    config,
		logger:    logger,
		connected: false, // Not connected
	}

	// Create GRPCTransport
	transport := NewGRPCTransport(grpcClient, logger)

	// Test operations when not connected
	_, err := transport.Hello("1.0.0", []string{})
	if err == nil {
		t.Error("Expected Hello to fail when not connected")
	}

	_, err = transport.Authenticate("token")
	if err == nil {
		t.Error("Expected Authenticate to fail when not connected")
	}

	_, err = transport.CreateTunnel("tunnel", "tenant", 8080, "host", 80)
	if err == nil {
		t.Error("Expected CreateTunnel to fail when not connected")
	}

	_, err = transport.SendHeartbeat("client", "tenant", nil)
	if err == nil {
		t.Error("Expected SendHeartbeat to fail when not connected")
	}

	// Test IsConnected
	if transport.IsConnected() {
		t.Error("Expected IsConnected to return false")
	}

	// Test with connected client
	grpcClient.connected = true
	if !transport.IsConnected() {
		t.Error("Expected IsConnected to return true")
	}
}

// newTestLogger creates a simple logger for testing
func newTestLogger() Logger {
	return &testLogger{}
}

type testLogger struct{}

func (tl *testLogger) Info(msg string, fields ...interface{})  {}
func (tl *testLogger) Error(msg string, fields ...interface{}) {}
func (tl *testLogger) Debug(msg string, fields ...interface{}) {}
func (tl *testLogger) Warn(msg string, fields ...interface{})  {}
