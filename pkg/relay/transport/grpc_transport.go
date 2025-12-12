package transport

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/relay/transport/proto"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GRPCTransport implements the Transport interface using gRPC
type GRPCTransport struct {
	client *GRPCClient
	logger Logger
}

// NewGRPCTransport creates a new gRPC transport
func NewGRPCTransport(client *GRPCClient, logger Logger) *GRPCTransport {
	return &GRPCTransport{
		client: client,
		logger: logger,
	}
}

// Connect establishes connection to the relay server
func (gt *GRPCTransport) Connect() error {
	return gt.client.Connect()
}

// Disconnect closes the connection
func (gt *GRPCTransport) Disconnect() error {
	return gt.client.Disconnect()
}

// Hello performs initial handshake
func (gt *GRPCTransport) Hello(version string, features []string) (*HelloResult, error) {
	if !gt.client.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	gt.logger.Debug("Sending gRPC Hello request", "version", version, "features", features)

	// Create gRPC client
	client := proto.NewControlServiceClient(gt.client.GetConnection())

	// Create request
	req := &proto.HelloRequest{
		Version:   version,
		Features:  features,
		ClientId:  "client-id",
		Timestamp: timestamppb.Now(),
	}

	// Make gRPC call with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Add Authorization header to gRPC metadata
	// Try to get token from environment variable or use empty for anonymous mode
	if authToken := os.Getenv("CLOUDBRIDGE_TOKEN"); authToken != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+authToken)
		gt.logger.Debug("Added authorization header to gRPC request from env")
	}

	resp, err := client.Hello(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC Hello failed: %w", err)
	}

	// Convert response
	result := &HelloResult{
		Status:            resp.Status,
		ServerVersion:     resp.ServerVersion,
		SupportedFeatures: resp.SupportedFeatures,
		SessionID:         resp.SessionId,
		ErrorMessage:      resp.ErrorMessage,
	}

	gt.logger.Info("gRPC Hello completed", "status", result.Status, "session_id", result.SessionID)
	return result, nil
}

// Authenticate performs authentication
func (gt *GRPCTransport) Authenticate(token string) (*AuthResult, error) {
	if !gt.client.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	gt.logger.Debug("Sending gRPC Auth request")

	// Create gRPC client
	client := proto.NewControlServiceClient(gt.client.GetConnection())

	// Create request with token from environment or parameter
	authToken := token
	if envToken := os.Getenv("CLOUDBRIDGE_TOKEN"); envToken != "" {
		authToken = envToken
	}

	req := &proto.AuthRequest{
		Token:     authToken, // ИСПРАВЛЕНО: используем токен из env или параметра
		AuthType:  "jwt",
		ClientId:  "client-id",
		Timestamp: timestamppb.Now(),
	}

	// Make gRPC call with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Add Authorization header to gRPC metadata for Auth request too
	if authEnvToken := os.Getenv("CLOUDBRIDGE_TOKEN"); authEnvToken != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+authEnvToken)
		gt.logger.Debug("Added authorization header to gRPC auth request from env")
	} else if token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		gt.logger.Debug("Added authorization header to gRPC auth request from param")
	}

	resp, err := client.Authenticate(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC Authenticate failed: %w", err)
	}

	// Convert response
	result := &AuthResult{
		Status:       resp.Status,
		ClientID:     resp.ClientId,
		TenantID:     resp.TenantId,
		SessionToken: resp.SessionToken,
		ExpiresAt:    resp.ExpiresAt.AsTime(),
		ErrorMessage: resp.ErrorMessage,
	}

	gt.logger.Info("gRPC Authentication completed", "status", result.Status, "client_id", result.ClientID)
	return result, nil
}

// CreateTunnel creates a new tunnel
func (gt *GRPCTransport) CreateTunnel(tunnelID, tenantID string, localPort int, remoteHost string, remotePort int) (*TunnelResult, error) {
	if !gt.client.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	gt.logger.Debug("Sending gRPC CreateTunnel request",
		"tunnel_id", tunnelID,
		"tenant_id", tenantID,
		"local_port", localPort,
		"remote_host", remoteHost,
		"remote_port", remotePort)

	// Create gRPC client
	client := proto.NewTunnelServiceClient(gt.client.GetConnection())

	// Create request
	req := &proto.CreateTunnelRequest{
		TunnelId:   tunnelID,
		TenantId:   tenantID,
		LocalPort:  int32(localPort),
		RemoteHost: remoteHost,
		RemotePort: int32(remotePort),
		Config: &proto.TunnelConfig{
			BufferSize:         4096,
			MaxBuffers:         100,
			TimeoutSeconds:     30,
			CompressionEnabled: false,
			EncryptionEnabled:  true,
		},
		Timestamp: timestamppb.Now(),
	}

	// Make gRPC call with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Add Authorization header to gRPC metadata for CreateTunnel
	if authToken := os.Getenv("CLOUDBRIDGE_TOKEN"); authToken != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+authToken)
		gt.logger.Debug("Added authorization header to gRPC CreateTunnel request")
	}

	resp, err := client.CreateTunnel(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC CreateTunnel failed: %w", err)
	}

	// Convert response
	result := &TunnelResult{
		Status:       resp.Status,
		TunnelID:     resp.TunnelId,
		Endpoint:     resp.Endpoint,
		ErrorMessage: resp.ErrorMessage,
	}

	gt.logger.Info("gRPC Tunnel created", "status", result.Status, "tunnel_id", result.TunnelID)
	return result, nil
}

// SendHeartbeat sends a heartbeat
func (gt *GRPCTransport) SendHeartbeat(clientID, tenantID string, metrics *ClientMetrics) (*HeartbeatResult, error) {
	if !gt.client.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	gt.logger.Debug("Sending gRPC Heartbeat", "client_id", clientID, "tenant_id", tenantID)

	// Create gRPC client
	client := proto.NewHeartbeatServiceClient(gt.client.GetConnection())

	// Convert metrics
	var protoMetrics *proto.ClientMetrics
	if metrics != nil {
		var lastSwitch *timestamppb.Timestamp
		if !metrics.LastSwitch.IsZero() {
			lastSwitch = timestamppb.New(metrics.LastSwitch)
		}

		protoMetrics = &proto.ClientMetrics{
			BytesSent:         metrics.BytesSent,
			BytesReceived:     metrics.BytesReceived,
			PacketsSent:       metrics.PacketsSent,
			PacketsReceived:   metrics.PacketsReceived,
			ActiveTunnels:     metrics.ActiveTunnels,
			ActiveP2PSessions: metrics.ActiveP2PSessions,
			CpuUsage:          metrics.CPUUsage,
			MemoryUsage:       metrics.MemoryUsage,
			TransportMode:     metrics.TransportMode,
			LastSwitch:        lastSwitch,
		}
	}

	// Create request
	req := &proto.HeartbeatRequest{
		ClientId:      clientID,
		TenantId:      tenantID,
		Timestamp:     timestamppb.Now(),
		Metrics:       protoMetrics,
		TransportMode: "grpc",
	}

	// Make gRPC call with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendHeartbeat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gRPC SendHeartbeat failed: %w", err)
	}

	// Convert response
	result := &HeartbeatResult{
		Status:          resp.Status,
		ServerTimestamp: resp.ServerTimestamp.AsTime(),
		IntervalSeconds: resp.IntervalSeconds,
		ErrorMessage:    resp.ErrorMessage,
	}

	gt.logger.Debug("gRPC Heartbeat completed", "status", result.Status)
	return result, nil
}

// IsConnected returns connection status
func (gt *GRPCTransport) IsConnected() bool {
	return gt.client.IsConnected()
}

// Close closes the transport and cleans up resources
func (gt *GRPCTransport) Close() error {
	return gt.client.Close()
}

// GetClient returns the underlying gRPC client
func (gt *GRPCTransport) GetClient() *GRPCClient {
	return gt.client
}

// StreamHeartbeat implements streaming heartbeat functionality
func (gt *GRPCTransport) StreamHeartbeat(clientID, tenantID string, metricsChan <-chan *ClientMetrics) error {
	if !gt.client.IsConnected() {
		return fmt.Errorf("not connected")
	}

	gt.logger.Debug("Starting gRPC streaming heartbeat", "client_id", clientID, "tenant_id", tenantID)

	// Create gRPC client
	client := proto.NewHeartbeatServiceClient(gt.client.GetConnection())

	// Create streaming context
	ctx, cancel := context.WithCancel(gt.client.GetContext())
	defer cancel()

	// Start streaming heartbeat
	stream, err := client.StreamHeartbeat(ctx)
	if err != nil {
		return fmt.Errorf("failed to start streaming heartbeat: %w", err)
	}

	// Send heartbeats from metrics channel
	go func() {
		defer func() {
			_ = stream.CloseSend() //nolint:errcheck // Ignore error in cleanup
		}()

		for metrics := range metricsChan {
			// Convert metrics
			var protoMetrics *proto.ClientMetrics
			if metrics != nil {
				var lastSwitch *timestamppb.Timestamp
				if !metrics.LastSwitch.IsZero() {
					lastSwitch = timestamppb.New(metrics.LastSwitch)
				}

				protoMetrics = &proto.ClientMetrics{
					BytesSent:         metrics.BytesSent,
					BytesReceived:     metrics.BytesReceived,
					PacketsSent:       metrics.PacketsSent,
					PacketsReceived:   metrics.PacketsReceived,
					ActiveTunnels:     metrics.ActiveTunnels,
					ActiveP2PSessions: metrics.ActiveP2PSessions,
					CpuUsage:          metrics.CPUUsage,
					MemoryUsage:       metrics.MemoryUsage,
					TransportMode:     metrics.TransportMode,
					LastSwitch:        lastSwitch,
				}
			}

			// Create request
			req := &proto.HeartbeatRequest{
				ClientId:      clientID,
				TenantId:      tenantID,
				Timestamp:     timestamppb.Now(),
				Metrics:       protoMetrics,
				TransportMode: "grpc",
			}

			if err := stream.Send(req); err != nil {
				gt.logger.Error("Failed to send streaming heartbeat", "error", err)
				return
			}
		}
	}()

	// Receive responses
	for {
		resp, err := stream.Recv()
		if err != nil {
			return fmt.Errorf("streaming heartbeat receive error: %w", err)
		}

		gt.logger.Debug("Received streaming heartbeat response", "status", resp.Status)
	}
}
