// grpc-smoke: Connect + Hello + Authenticate + CreateTunnel + TCP bytes.
package main

import (
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
	token := flag.String("token", "", "JWT")
	doTunnel := flag.Bool("tunnel", true, "CreateTunnel + TCP bytes")
	echoPort := flag.Int("echo-port", 18080, "local TCP echo")
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
			TLS: types.TLSConfig{Enabled: false, VerifyCert: false},
		},
	}

	logger := logAdapter{}
	client := transport.NewGRPCClient(cfg, logger)
	tr := transport.NewGRPCTransport(client, logger)

	log.Printf("Connecting gRPC %s:%d", *host, *port)
	if err := tr.Connect(); err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL connect: %v", err)
	}
	defer func() { _ = tr.Disconnect() }()
	log.Printf("GRPC_STEP=connect_ok")

	hello, err := tr.Hello("smoke-1.0", []string{"tls", "heartbeat", "grpc"})
	if err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL hello: %v", err)
	}
	if hello.Status != "ok" {
		log.Fatalf("GRPC_SMOKE_FAIL hello status=%s", hello.Status)
	}
	log.Printf("GRPC_STEP=hello_ok session=%s", hello.SessionID)

	auth, err := tr.Authenticate(*token)
	if err != nil {
		log.Fatalf("GRPC_SMOKE_FAIL auth: %v", err)
	}
	if auth.Status != "ok" {
		log.Fatalf("GRPC_SMOKE_FAIL auth status=%s", auth.Status)
	}
	log.Printf("GRPC_STEP=auth_ok client_id=%s", auth.ClientID)

	if *doTunnel {
		ln, err := startEcho(*echoPort)
		if err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL echo: %v", err)
		}
		defer ln.Close()
		time.Sleep(50 * time.Millisecond)

		tunnelID := fmt.Sprintf("smoke-tun-%d", time.Now().Unix())
		tenantID := auth.TenantID
		if tenantID == "" {
			tenantID = "default"
		}
		tun, err := tr.CreateTunnel(tunnelID, tenantID, 0, "127.0.0.1", *echoPort)
		if err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL CreateTunnel: %v", err)
		}
		if tun.Status != "ok" {
			log.Fatalf("GRPC_SMOKE_FAIL tunnel status=%s err=%s", tun.Status, tun.ErrorMessage)
		}
		log.Printf("GRPC_STEP=tunnel_ok endpoint=%s", tun.Endpoint)

		payload := []byte("tunnel-bytes-smoke")
		conn, err := net.DialTimeout("tcp", tun.Endpoint, 5*time.Second)
		if err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL dial endpoint %s: %v", tun.Endpoint, err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(payload); err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL write: %v", err)
		}
		buf := make([]byte, len(payload))
		if _, err := io.ReadFull(conn, buf); err != nil {
			log.Fatalf("GRPC_SMOKE_FAIL read echo: %v", err)
		}
		if string(buf) != string(payload) {
			log.Fatalf("GRPC_SMOKE_FAIL mismatch got=%q", buf)
		}
		log.Printf("GRPC_STEP=tunnel_bytes_ok payload=%q", buf)
	}

	log.Printf("GRPC_SMOKE_PASS=1 bytes=true")
	fmt.Println("gRPC CreateTunnel data-plane (TCP bytes) OK")
}
