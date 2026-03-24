package tlsfingerprint

import (
	"context"
	"crypto/tls"
	"net"

	utls "github.com/refraction-networking/utls"
)

// Dialer wraps a net.Dialer and provides TLS fingerprinting using utls.
type Dialer struct {
	dialer *net.Dialer
	config *tls.Config
}

// NewDialer creates a new Dialer with TLS fingerprinting capability.
func NewDialer(config *tls.Config) *Dialer {
	if config == nil {
		config = &tls.Config{}
	}
	return &Dialer{
		dialer: &net.Dialer{},
		config: config,
	}
}

// DialTLS establishes a TLS connection with Node.js 20.x fingerprint.
func (d *Dialer) DialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	// Establish TCP connection
	conn, err := d.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Extract hostname for SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Create utls config
	utlsConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: d.config.InsecureSkipVerify,
		RootCAs:            d.config.RootCAs,
	}

	// Create utls connection with Node.js 20.x fingerprint
	utlsConn := utls.UClient(conn, utlsConfig, utls.HelloCustom)

	// Build Node.js 20.x ClientHelloSpec
	spec := nodeJS20Spec()
	if err := utlsConn.ApplyPreset(&spec); err != nil {
		conn.Close()
		return nil, err
	}

	// Perform handshake
	if err := utlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return utlsConn, nil
}

// nodeJS20Spec returns a ClientHelloSpec mimicking Node.js 20.x with OpenSSL 3.x.
func nodeJS20Spec() utls.ClientHelloSpec {
	return utls.ClientHelloSpec{
		CipherSuites: []uint16{
			utls.TLS_AES_256_GCM_SHA384,
			utls.TLS_CHACHA20_POLY1305_SHA256,
			utls.TLS_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			utls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			utls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		Extensions: []utls.TLSExtension{
			&utls.SNIExtension{},
			&utls.SupportedCurvesExtension{Curves: []utls.CurveID{
				utls.X25519,
				utls.CurveP256,
				utls.CurveP384,
			}},
			&utls.SupportedPointsExtension{SupportedPoints: []byte{0}},
			&utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}},
			&utls.SupportedVersionsExtension{Versions: []uint16{
				utls.VersionTLS13,
				utls.VersionTLS12,
			}},
			&utls.SignatureAlgorithmsExtension{SupportedSignatureAlgorithms: []utls.SignatureScheme{
				utls.ECDSAWithP256AndSHA256,
				utls.PSSWithSHA256,
				utls.PKCS1WithSHA256,
				utls.ECDSAWithP384AndSHA384,
				utls.PSSWithSHA384,
				utls.PKCS1WithSHA384,
				utls.PSSWithSHA512,
				utls.PKCS1WithSHA512,
			}},
			&utls.KeyShareExtension{KeyShares: []utls.KeyShare{
				{Group: utls.X25519},
			}},
		},
	}
}

// PerformHandshake performs TLS fingerprint handshake on an existing connection.
func PerformHandshake(conn net.Conn, host string, config *tls.Config) (net.Conn, error) {
	if config == nil {
		config = &tls.Config{}
	}

	utlsConfig := &utls.Config{
		ServerName:         host,
		InsecureSkipVerify: config.InsecureSkipVerify,
		RootCAs:            config.RootCAs,
	}

	utlsConn := utls.UClient(conn, utlsConfig, utls.HelloCustom)

	spec := nodeJS20Spec()
	if err := utlsConn.ApplyPreset(&spec); err != nil {
		conn.Close()
		return nil, err
	}

	if err := utlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return utlsConn, nil
}
