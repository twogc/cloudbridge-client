// quic-mesh-smoke: two QUIC peers AUTH, then A → B via TO:<peer_id>:<payload>.
// Usage:
//
//	JWT_SECRET=test-secret go run ./scripts/quic-mesh-smoke -addr 127.0.0.1:5553
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quic-go/quic-go"
)

func makeP2PToken(secret, peerID string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":             fmt.Sprintf("mesh-%s-%d", peerID, now.UnixNano()),
		"sub":             peerID,
		"peer_id":         peerID,
		"tenant_id":       "default",
		"server_id":       "quic-mesh-smoke",
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

func dialAndAuth(ctx context.Context, addr, secret, peerID string) (*quic.Conn, error) {
	token, err := makeP2PToken(secret, peerID)
	if err != nil {
		return nil, err
	}
	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{"cloudbridge-p2p", "h3"},
		ServerName:         "localhost",
	}
	qconf := &quic.Config{
		HandshakeIdleTimeout:  8 * time.Second,
		MaxIdleTimeout:        30 * time.Second,
		KeepAlivePeriod:       10 * time.Second,
		MaxIncomingStreams:    256, // accept server-initiated relay streams
		MaxIncomingUniStreams: 256,
	}
	conn, err := quic.DialAddr(ctx, addr, tlsConf, qconf)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(0, "auth stream fail")
		return nil, fmt.Errorf("open auth stream: %w", err)
	}
	authMsg := "AUTH " + token
	if _, err := stream.Write([]byte(authMsg)); err != nil {
		_ = stream.Close()
		_ = conn.CloseWithError(0, "auth write fail")
		return nil, fmt.Errorf("write AUTH: %w", err)
	}
	_ = stream.SetReadDeadline(time.Now().Add(8 * time.Second))
	buf := make([]byte, 256)
	n, err := stream.Read(buf)
	if err != nil {
		_ = stream.Close()
		_ = conn.CloseWithError(0, "auth read fail")
		return nil, fmt.Errorf("read AUTH response: %w", err)
	}
	resp := string(buf[:n])
	if resp != "AUTH_OK" {
		_ = stream.Close()
		_ = conn.CloseWithError(0, "auth bad")
		return nil, fmt.Errorf("expected AUTH_OK got %q", resp)
	}
	_ = stream.Close()
	log.Printf("MESH_STEP=auth_ok peer=%s", peerID)
	return conn, nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:5553", "P2P QUIC UDP address")
	secret := flag.String("secret", "", "HMAC secret")
	timeout := flag.Duration("timeout", 20*time.Second, "overall timeout")
	payload := flag.String("payload", "mesh-payload-ab", "payload body after TO header")
	flag.Parse()

	if *secret == "" {
		*secret = os.Getenv("JWT_SECRET")
	}
	if *secret == "" {
		*secret = "test-secret"
	}

	peerA := "mesh-peer-a"
	peerB := "mesh-peer-b"

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("Dialing peer A %s", *addr)
	connA, err := dialAndAuth(ctx, *addr, *secret, peerA)
	if err != nil {
		log.Fatalf("QUIC_MESH_FAIL peer A: %v", err)
	}
	defer connA.CloseWithError(0, "mesh done")

	log.Printf("Dialing peer B %s", *addr)
	connB, err := dialAndAuth(ctx, *addr, *secret, peerB)
	if err != nil {
		log.Fatalf("QUIC_MESH_FAIL peer B: %v", err)
	}
	defer connB.CloseWithError(0, "mesh done")

	// Receiver must AcceptStream — relay opens a stream toward the target peer.
	recvCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		sctx, scancel := context.WithTimeout(ctx, 12*time.Second)
		defer scancel()
		stream, err := connB.AcceptStream(sctx)
		if err != nil {
			errCh <- fmt.Errorf("B AcceptStream: %w", err)
			return
		}
		defer stream.Close()
		_ = stream.SetReadDeadline(time.Now().Add(8 * time.Second))
		data, err := io.ReadAll(stream)
		if err != nil && err != io.EOF {
			// partial read may still be valid
			if len(data) == 0 {
				errCh <- fmt.Errorf("B read: %w", err)
				return
			}
		}
		recvCh <- string(data)
	}()

	// Brief yield so B's accept loop and server registration settle.
	time.Sleep(100 * time.Millisecond)

	// A → B: TO:<peer_id>:<payload>
	msg := fmt.Sprintf("TO:%s:%s", peerB, *payload)
	stream, err := connA.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("QUIC_MESH_FAIL open data stream: %v", err)
	}
	if _, err := stream.Write([]byte(msg)); err != nil {
		log.Fatalf("QUIC_MESH_FAIL write TO packet: %v", err)
	}
	// Give the peer/relay a moment to read before FIN (quic-go edge cases).
	time.Sleep(200 * time.Millisecond)
	// Close write side so relay/receiver sees FIN after full frame.
	_ = stream.Close()
	log.Printf("MESH_STEP=sent_to_b bytes=%d msg=%q", len(msg), msg)

	select {
	case got := <-recvCh:
		log.Printf("MESH_STEP=recv_on_b %q", got)
		if !strings.Contains(got, *payload) {
			log.Fatalf("QUIC_MESH_FAIL expected payload %q in %q", *payload, got)
		}
		// Prefer exact frame, tolerate if only payload arrives
		if got != msg && !strings.HasSuffix(got, *payload) {
			log.Fatalf("QUIC_MESH_FAIL unexpected frame %q", got)
		}
	case err := <-errCh:
		log.Fatalf("QUIC_MESH_FAIL receive: %v", err)
	case <-ctx.Done():
		log.Fatalf("QUIC_MESH_FAIL timeout waiting for B")
	}

	// Reverse: B → A (start acceptor before open)
	recvA := make(chan string, 1)
	errA := make(chan error, 1)
	go func() {
		sctx, scancel := context.WithTimeout(ctx, 12*time.Second)
		defer scancel()
		stream, err := connA.AcceptStream(sctx)
		if err != nil {
			errA <- fmt.Errorf("A AcceptStream: %w", err)
			return
		}
		defer stream.Close()
		_ = stream.SetReadDeadline(time.Now().Add(8 * time.Second))
		data, err := io.ReadAll(stream)
		if err != nil && err != io.EOF && len(data) == 0 {
			errA <- fmt.Errorf("A read: %w", err)
			return
		}
		recvA <- string(data)
	}()
	time.Sleep(100 * time.Millisecond)

	revPayload := *payload + "-ba"
	rev := fmt.Sprintf("TO:%s:%s", peerA, revPayload)
	s2, err := connB.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("QUIC_MESH_FAIL reverse open: %v", err)
	}
	if _, err := s2.Write([]byte(rev)); err != nil {
		log.Fatalf("QUIC_MESH_FAIL reverse write: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	_ = s2.Close()
	log.Printf("MESH_STEP=sent_to_a bytes=%d msg=%q", len(rev), rev)

	select {
	case got := <-recvA:
		log.Printf("MESH_STEP=recv_on_a %q", got)
		if !strings.Contains(got, revPayload) {
			log.Fatalf("QUIC_MESH_FAIL reverse payload missing in %q", got)
		}
	case err := <-errA:
		log.Fatalf("QUIC_MESH_FAIL reverse receive: %v", err)
	case <-ctx.Done():
		log.Fatalf("QUIC_MESH_FAIL reverse timeout")
	}

	log.Printf("QUIC_MESH_SMOKE_PASS=1 addr=%s a=%s b=%s at=%s",
		*addr, peerA, peerB, time.Now().Format(time.RFC3339))
	fmt.Println("QUIC multi-peer mesh: A↔B TO:<peer> payload OK")
}
