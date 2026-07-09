// grpc-smoke: Track B+ — Connect + Hello + Authenticate + CreateTunnel against local relay :8444.
// Usage:
//
//	go run ./scripts/grpc-smoke -host 127.0.0.1 -port 8444 -token "$TOKEN"
//	go run ./scripts/grpc-smoke -tunnel -echo-port 18080
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"github.com/twogc/cloudbridge-client/pkg/relay/transport"
	"github.com/twogc/cloudbridge-client/pkg/types"
)

type logAdapter struct{}

func formatFields(fields []interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	return " " + fmt.Sprint(fields...)
}

func (logAdapter) Info(msg string, fields ...interface{})  { log.Print("INFO  " + msg + formatFields(fields)) }
func (logAdapter) Error(msg string, fields ...interface{}) { log.Print("ERROR " + msg + formatFields(fields)) }
func (logAdapter) Debug(msg string, fields ...interface{}) { log.Print("DEBUG " + msg + formatFields(fields)) }
func (logAdapter) Warn(msg string, fields ...interface{})  { log.Print("WARN  " + msg + formatFields(fields)) }

func startEcho(port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				_, _ = io.Copy(conn, conn)
			}(c)
		}
	}()
	return ln, nil
}

func main() {
	host := flag.String("host", "127.0.0.1", "relay host")
	port := flag.Int("port", types.DefaultGRPCPort, "gRPC port")
	token := flag.String("token", "", "JWT (any string if relay auth is no-op)")
	doTunnel := flag.Bool("tunnel", true, "also CreateTunnel to local echo")
	echoPort := flag.Int("echo-port", 18080, "local TCP echo for CreateTunnel remote")
	flag.Parse()

	if *token == "" {
		*token = os.Getenv("CLOUDBRIDGE_TOKEN")
	}
	if *token == "" {
		*token = "local-smoke-grpc-token"
	}
	_ = os.Setenv("CLOUDBRIDGE_TOKEN", *token)

	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: *host,
			Ports: types.RelayPorts{
				GRPC: *port,
			},
			TLS: types.TLSConfig{
				Enabled:    false,
				VerifyCert: false,
			},
		},
	}

	logger := logAdapter{}
	client := transport.NewGRPCClient(cfg, logger)
	tr := transport.NewGRPCTransport(client, logger)

	log.Printf("Connecting gRPC %s:%d (TLS=off)", *host, *port)
	if err := tr.Connect(); err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL connect: %v", err)
	}
	defer func() { _ = tr.Disconnect() }()
	log.Printf("GRPC_STEP=connect_ok target=%s", cfg.GRPCTarget())

	hello, err := tr.Hello("smoke-1.0", []string{"tls", "heartbeat", "tunnel_info", "grpc"})
	if err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL hello: %v", err)
	}
	if hello.Status != "ok" {
		log.Fatalf("GRPC_SMOKE_FAIL hello status=%s err=%s", hello.Status, hello.ErrorMessage)
	}
	log.Printf("GRPC_STEP=hello_ok status=%s session=%s server=%s",
		hello.Status, hello.SessionID, hello.ServerVersion)

	auth, err := tr.Authenticate(*token)
	if err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL auth: %v", err)
	}
	if auth.Status != "ok" {
		log.Fatalf("GRPC_SMOKE_FAIL auth status=%s err=%s", auth.Status, auth.ErrorMessage)
	}
	log.Printf("GRPC_STEP=auth_ok status=%s client_id=%s tenant_id=%s",
		auth.Status, auth.ClientID, auth.TenantID)

	if *doTunnel {
		ln, err := startEcho(*echoPort)
		if err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL echo listen: %v", err)
		}
		defer ln.Close()
		// wait briefly for accept loop
		time.Sleep(50 * time.Millisecond)

		tunnelID := fmt.Sprintf("smoke-tun-%d", time.Now().Unix())
		tenantID := auth.TenantID
		if tenantID == "" {
			tenantID = "default"
		}
		// CreateTunnel RPC: server opens dial to remote_host:remote_port
		tun, err := tr.CreateTunnel(tunnelID, tenantID, 0, "127.0.0.1", *echoPort)
		if err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL CreateTunnel: %v", err)
		}
		if tun.Status != "ok" {
			log.Fatalf("GRPC_SMOKE_FAIL CreateTunnel status=%s err=%s", tun.Status, tun.ErrorMessage)
		}
		log.Printf("GRPC_STEP=tunnel_ok status=%s tunnel_id=%s endpoint=%s",
			tun.Status, tun.TunnelID, tun.Endpoint)

		// ListTunnels is not on Transport interface — optional skip
		_ = context.Background()
	}

	log.Printf("GRPC_SMOKE_PASS=1 host=%s port=%d tunnel=%v at=%s",
		*host, *port, *doTunnel, time.Now().Format(time.RFC3339))
	fmt.Println("Track B+: gRPC Connect+Hello+Auth+CreateTunnel OK")
}
