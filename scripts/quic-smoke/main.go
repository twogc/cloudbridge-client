// quic-smoke: full P2P QUIC dial to relay :5553 —
// TLS handshake + AUTH stream + AUTH_OK + post-AUTH PING/PONG (data plane).
// Usage:
//
//	JWT_SECRET=test-secret go run ./scripts/quic-smoke -addr 127.0.0.1:5553
//	JWT_SECRET=test-secret go run ./scripts/quic-smoke -addr 127.0.0.1:5553 -no-payload
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quic-go/quic-go"
)

func makeP2PToken(secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":             fmt.Sprintf("quic-%d", now.UnixNano()),
		"sub":             "quic-smoke",
		"peer_id":         "quic-smoke-peer", // required by internal/p2p.TokenValidator
		"tenant_id":       "default",
		"server_id":       "quic-smoke-server",
		"connection_type": "quic",
		"protocol_type":   "p2p-mesh",
		"permissions":     []string{"p2p_connect"},
		"iat":             now.Unix(),
		"nbf":             now.Add(-time.Minute).Unix(),
		"exp":             now.Add(time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(secret))
}

func main() {
	addr := flag.String("addr", "127.0.0.1:5553", "P2P QUIC UDP address")
	secret := flag.String("secret", "", "HMAC secret (default JWT_SECRET or test-secret)")
	timeout := flag.Duration("timeout", 15*time.Second, "overall timeout")
	noPayload := flag.Bool("no-payload", false, "stop after AUTH_OK (skip PING/PONG)")
	payload := flag.String("payload", "quic-payload-smoke", "payload after PING ")
	flag.Parse()

	if *secret == "" {
		*secret = os.Getenv("JWT_SECRET")
	}
	if *secret == "" {
		*secret = "test-secret"
	}

	token, err := makeP2PToken(*secret)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL token: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{"cloudbridge-p2p", "h3"},
		ServerName:         "localhost",
	}
	qconf := &quic.Config{
		HandshakeIdleTimeout: 8 * time.Second,
		MaxIdleTimeout:       20 * time.Second,
		KeepAlivePeriod:      10 * time.Second,
	}

	log.Printf("Dialing QUIC %s alpn=%v", *addr, tlsConf.NextProtos)
	conn, err := quic.DialAddr(ctx, *addr, tlsConf, qconf)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL dial: %v", err)
	}
	defer conn.CloseWithError(0, "smoke done")
	log.Printf("QUIC_STEP=dial_ok remote=%s local=%s", conn.RemoteAddr(), conn.LocalAddr())

	// --- AUTH on first stream ---
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL open stream: %v", err)
	}

	authMsg := "AUTH " + token
	if _, err := stream.Write([]byte(authMsg)); err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL write AUTH: %v", err)
	}
	log.Printf("QUIC_STEP=auth_sent bytes=%d", len(authMsg))

	_ = stream.SetReadDeadline(time.Now().Add(8 * time.Second))
	buf := make([]byte, 256)
	n, err := stream.Read(buf)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL read AUTH response: %v", err)
	}
	resp := string(buf[:n])
	log.Printf("QUIC_STEP=auth_resp %q", resp)
	if resp != "AUTH_OK" {
		log.Fatalf("QUIC_SMOKE_FAIL expected AUTH_OK got %q", resp)
	}
	_ = stream.Close()

	if *noPayload {
		log.Printf("QUIC_SMOKE_PASS=1 addr=%s mode=auth_only at=%s", *addr, time.Now().Format(time.RFC3339))
		fmt.Println("Full QUIC: dial + AUTH_OK OK")
		return
	}

	// --- Post-AUTH data plane: second stream PING → PONG ---
	// Brief yield so relay finishes auth registration and enters AcceptStream loop.
	time.Sleep(50 * time.Millisecond)

	dataStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL open data stream: %v", err)
	}
	defer dataStream.Close()

	pingMsg := "PING " + *payload
	if _, err := dataStream.Write([]byte(pingMsg)); err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL write PING: %v", err)
	}
	log.Printf("QUIC_STEP=ping_sent %q", pingMsg)

	_ = dataStream.SetReadDeadline(time.Now().Add(8 * time.Second))
	n, err = dataStream.Read(buf)
	if err != nil {
		log.Fatalf("QUIC_SMOKE_FAIL read PONG: %v", err)
	}
	pong := string(buf[:n])
	log.Printf("QUIC_STEP=pong_resp %q", pong)
	want := "PONG " + *payload
	if pong != want {
		log.Fatalf("QUIC_SMOKE_FAIL expected %q got %q", want, pong)
	}

	log.Printf("QUIC_SMOKE_PASS=1 addr=%s mode=auth+payload at=%s", *addr, time.Now().Format(time.RFC3339))
	fmt.Println("Full QUIC: dial + AUTH_OK + PING/PONG payload OK")
}
