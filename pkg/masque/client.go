package masque

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// MASQUEClient представляет клиент для MASQUE протокола с HTTP/3
type MASQUEClient struct {
	h3rt   *http3.Transport
	http   *http.Client
	config *MASQUEConfig
}

// UDPSession представляет сессию для работы с UDP датаграммами
type UDPSession struct {
	resp *http.Response
	// TODO: Добавить реальные DatagramWriter/DatagramReader когда API стабилизируется
	// w    http3.DatagramWriter
	// r    http3.DatagramReader
}

// MASQUEConfig конфигурация MASQUE клиента
type MASQUEConfig struct {
	ServerName         string        `json:"server_name"`
	HandshakeTimeout   time.Duration `json:"handshake_timeout"`
	IdleTimeout        time.Duration `json:"idle_timeout"`
	EnableDatagrams    bool          `json:"enable_datagrams"`
	Enable0RTT         bool          `json:"enable_0rtt"` // CRITICAL: false по умолчанию
	InsecureSkipVerify bool          `json:"insecure_skip_verify"`
}

// DefaultMASQUEConfig возвращает безопасную конфигурацию по умолчанию
func DefaultMASQUEConfig() *MASQUEConfig {
	return &MASQUEConfig{
		ServerName:         "edge.2gc.ru",
		HandshakeTimeout:   8 * time.Second,
		IdleTimeout:        30 * time.Second,
		EnableDatagrams:    true,
		Enable0RTT:         false, // CRITICAL: выключено по умолчанию
		InsecureSkipVerify: false,
	}
}

// NewMASQUEClient создает новый MASQUE клиент с HTTP/3
func NewMASQUEClient(cfg *MASQUEConfig) (*MASQUEClient, error) {
	if cfg == nil {
		cfg = DefaultMASQUEConfig()
	}

	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		ServerName:         cfg.ServerName,
		// NextProtos выставляет сам http3.Transport
	}

	qconf := &quic.Config{
		EnableDatagrams:       cfg.EnableDatagrams,
		HandshakeIdleTimeout:  cfg.HandshakeTimeout,
		MaxIdleTimeout:        cfg.IdleTimeout,
		Allow0RTT:             cfg.Enable0RTT,
		KeepAlivePeriod:       0,
		MaxIncomingStreams:    0, // дефолт
		MaxIncomingUniStreams: 0,
	}

	h3rt := &http3.Transport{
		TLSClientConfig: tlsConf,
		QUICConfig:      qconf,
		EnableDatagrams: cfg.EnableDatagrams,
	}

	// net/http клиент, который ездит через http3.Transport
	httpClient := &http.Client{
		Transport: h3rt,
		Timeout:   cfg.IdleTimeout + cfg.HandshakeTimeout + 5*time.Second,
	}

	return &MASQUEClient{
		h3rt:   h3rt,
		http:   httpClient,
		config: cfg,
	}, nil
}

// ConnectUDP устанавливает CONNECT-UDP туннель через MASQUE
func (mc *MASQUEClient) ConnectUDP(ctx context.Context, masqueURL, target string) (*UDPSession, error) {
	if mc.h3rt == nil {
		return nil, errors.New("http3 transport not initialized")
	}
	u, err := url.Parse(masqueURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MASQUE URL: %w", err)
	}

	// Extended CONNECT: :protocol = "connect-udp".
	// В quic-go/http3 это поддерживается на уровне транспорта.
	req, err := http.NewRequestWithContext(ctx, http.MethodConnect, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create CONNECT request: %w", err)
	}

	// authority целевого хоста:порт (RFC 9298)
	req.Host = target

	// TODO: После стабилизации API добавить Extended CONNECT с :protocol="connect-udp"
	// В некоторых версиях http3 есть helper наподобие:
	// http3.SetExtendedConnect(req, "connect-udp")
	// Если его нет — используйте соответствующий API вашей версии quic-go/http3.

	resp, err := mc.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CONNECT-UDP failed: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("CONNECT-UDP failed: status=%s", resp.Status)
	}

	// Далее нужно получить datagram writer/reader — это делается не из *http.Response,
	// а из транспорта/коннекта (API зависит от версии http3).
	// Оставим TODO-хуки:
	// w, r := http3.ExtractDatagramRW(resp) // псевдокод

	return &UDPSession{resp: resp}, nil
}

// ConnectIP устанавливает CONNECT-IP туннель через MASQUE
func (mc *MASQUEClient) ConnectIP(ctx context.Context, masqueURL, target string) (*http.Response, error) {
	if mc.h3rt == nil {
		return nil, errors.New("http3 transport not initialized")
	}
	u, err := url.Parse(masqueURL)
	if err != nil {
		return nil, fmt.Errorf("invalid MASQUE URL: %w", err)
	}

	// Extended CONNECT: :protocol = "connect-ip" (RFC 9484)
	req, err := http.NewRequestWithContext(ctx, http.MethodConnect, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create CONNECT request: %w", err)
	}
	req.Host = target

	// TODO: После стабилизации API добавить Extended CONNECT с :protocol="connect-ip"
	// http3.SetExtendedConnect(req, "connect-ip") // псевдокод

	resp, err := mc.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CONNECT-IP failed: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("CONNECT-IP failed: status=%s", resp.Status)
	}

	return resp, nil
}

// Close закрывает MASQUE клиент
func (mc *MASQUEClient) Close() error {
	if mc.h3rt != nil {
		mc.h3rt.Close() // закроет все H3 соединения
	}
	return nil
}

// GetConfig возвращает конфигурацию клиента
func (mc *MASQUEClient) GetConfig() *MASQUEConfig {
	return mc.config
}

// IsConnected проверяет, подключен ли клиент
func (mc *MASQUEClient) IsConnected() bool {
	// В HTTP/3 проверка соединения сложнее, так как это не TCP
	// Можно проверить через попытку создания нового запроса или
	// через состояние QUIC соединения
	return mc.h3rt != nil
}

// SendDatagram отправляет UDP датаграмму
// TODO: Реализовать после стабилизации API http3
func (s *UDPSession) SendDatagram(data []byte) error {
	// TODO: Реализовать отправку датаграммы через http3.DatagramWriter
	// w.Write(data)
	return fmt.Errorf("datagram sending not implemented yet - waiting for stable http3 API")
}

// ReceiveDatagram получает UDP датаграмму
// TODO: Реализовать после стабилизации API http3
func (s *UDPSession) ReceiveDatagram() ([]byte, error) {
	// TODO: Реализовать получение датаграммы через http3.DatagramReader
	// return r.Read()
	return nil, fmt.Errorf("datagram receiving not implemented yet - waiting for stable http3 API")
}

// Close закрывает UDP сессию
func (s *UDPSession) Close() error {
	if s.resp != nil && s.resp.Body != nil {
		return s.resp.Body.Close()
	}
	return nil
}
