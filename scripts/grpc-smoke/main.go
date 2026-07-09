// grpc-smoke: Track B — Connect + Hello + Authenticate against local relay :8444.
// Usage:
//
//	go run ./scripts/grpc-smoke -host 127.0.0.1 -port 8444 -token "$TOKEN"
//
// Local-smoke relay: grpc.enabled=true, tls_enabled=false, auth often no-op (any token OK).
package main

import (
	"flag"
	"fmt"
	"log"
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

func main() {
	host := flag.String("host", "127.0.0.1", "relay host")
	port := flag.Int("port", types.DefaultGRPCPort, "gRPC port")
	token := flag.String("token", "", "JWT (or any string if relay auth is no-op)")
	flag.Parse()

	if *token == "" {
		*token = os.Getenv("CLOUDBRIDGE_TOKEN")
	}
	if *token == "" {
		*token = "local-smoke-grpc-token"
	}
	// Hello RBAC requires authorization metadata; transport reads CLOUDBRIDGE_TOKEN for Hello.
	_ = os.Setenv("CLOUDBRIDGE_TOKEN", *token)

	cfg := &types.Config{
		Relay: types.RelayConfig{
			Host: *host,
			Ports: types.RelayPorts{
				GRPC: *port,
			},
			TLS: types.TLSConfig{
				Enabled:    false, // local-smoke plaintext gRPC
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
	log.Printf("GRPC_STEP=hello_ok status=%s session=%s server=%s features=%v",
		hello.Status, hello.SessionID, hello.ServerVersion, hello.SupportedFeatures)

	auth, err := tr.Authenticate(*token)
	if err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL auth: %v", err)
	}
	if auth.Status != "ok" {
		log.Fatalf("GRPC_SMOKE_FAIL auth status=%s err=%s", auth.Status, auth.ErrorMessage)
	}
	log.Printf("GRPC_STEP=auth_ok status=%s client_id=%s tenant_id=%s",
		auth.Status, auth.ClientID, auth.TenantID)

	// Optional: GetStatus if available on control service via transport — skip if not exposed on Transport iface
	log.Printf("GRPC_SMOKE_PASS=1 host=%s port=%d elapsed_ok=true at=%s",
		*host, *port, time.Now().Format(time.RFC3339))
	fmt.Println("Track B: gRPC Connect+Hello+Auth OK")
}
