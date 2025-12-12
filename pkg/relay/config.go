package relay

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Config represents the client configuration
type Config struct {
	UseTLS         bool
	TLSCertFile    string
	TLSKeyFile     string
	TLSCAFile      string
	ServerHost     string
	ServerPort     int
	JWTToken       string
	LocalPort      int
	ReconnectDelay int
	MaxRetries     int
}

// NewTLSConfig creates a new TLS configuration
func NewTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load client certificate if provided
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if caFile != "" {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}

		config.RootCAs = caCertPool
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServerHost == "" {
		return fmt.Errorf("server host is required")
	}

	if c.ServerPort <= 0 || c.ServerPort > 65535 {
		return fmt.Errorf("invalid server port")
	}

	if c.LocalPort <= 0 || c.LocalPort > 65535 {
		return fmt.Errorf("invalid local port")
	}

	if c.UseTLS {
		if c.TLSCertFile != "" && c.TLSKeyFile == "" {
			return fmt.Errorf("TLS key file is required when certificate file is provided")
		}
		if c.TLSKeyFile != "" && c.TLSCertFile == "" {
			return fmt.Errorf("TLS certificate file is required when key file is provided")
		}
	}

	return nil
}
