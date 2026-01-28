package security

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "io/ioutil"
    "net/http"
    "time"
)

type SecurityConfig struct {
    TLS        TLSConfig        `yaml:"tls"`
    Auth       AuthConfig       `yaml:"auth"`
    Encryption EncryptionConfig `yaml:"encryption"`
    Audit      AuditConfig      `yaml:"audit"`
}

type TLSConfig struct {
    Enabled       bool     `yaml:"enabled"`
    CertFile      string   `yaml:"cert_file"`
    KeyFile       string   `yaml:"key_file"`
    CAFile        string   `yaml:"ca_file"`
    MinVersion    string   `yaml:"min_version"`
    MaxVersion    string   `yaml:"max_version"`
    CipherSuites  []string `yaml:"cipher_suites"`
}

func ConfigureTLS(config TLSConfig) (*tls.Config, error) {
    if !config.Enabled {
        return nil, nil
    }

    // Load certificate
    cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load TLS certificate: %v", err)
    }

    // Load CA certificate
    caCert, err := ioutil.ReadFile(config.CAFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load CA certificate: %v", err)
    }

    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    // Create TLS config
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
        ClientCAs:    caCertPool,
        MinVersion:   tls.VersionTLS12,
        MaxVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
        },
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
        ClientAuth:               tls.RequireAndVerifyClientCert,
        PreferServerCipherSuites: true,
        SessionTicketsDisabled:   true,
    }

    return tlsConfig, nil
}

// SecureHTTPClient creates an HTTP client with security best practices
func SecureHTTPClient() *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
                MaxVersion: tls.VersionTLS13,
            },
            MaxIdleConns:          100,
            MaxIdleConnsPerHost:   10,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
        },
        Timeout: 30 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }
}

// Security middleware for API
func SecurityMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Set security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
        w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

        // Add request ID for tracing
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = generateUUID()
        }
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        r = r.WithContext(ctx)

        next.ServeHTTP(w, r)
    })
}

// Placeholder types for completeness – implement as needed
type AuthConfig struct{}
type EncryptionConfig struct{}
type AuditConfig struct{}

func generateUUID() string {
    // Simple placeholder – replace with proper UUID generation
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
