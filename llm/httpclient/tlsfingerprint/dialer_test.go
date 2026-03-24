package tlsfingerprint

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	utls "github.com/refraction-networking/utls"
)

func TestNewDialer(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		d := NewDialer(nil)
		if d == nil {
			t.Fatal("expected non-nil dialer")
		}
		if d.config == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &tls.Config{InsecureSkipVerify: true}
		d := NewDialer(config)
		if d == nil {
			t.Fatal("expected non-nil dialer")
		}
		if d.config != config {
			t.Fatal("expected config to be preserved")
		}
	})
}

func TestDialTLS(t *testing.T) {
	t.Run("successful connection", func(t *testing.T) {
		d := NewDialer(&tls.Config{})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		conn, err := d.DialTLS(ctx, "tcp", "www.google.com:443")
		if err != nil {
			t.Skipf("skipping test due to network error: %v", err)
		}
		defer conn.Close()

		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
	})

	t.Run("invalid address", func(t *testing.T) {
		d := NewDialer(&tls.Config{})
		ctx := context.Background()

		_, err := d.DialTLS(ctx, "tcp", "invalid-host-that-does-not-exist:443")
		if err == nil {
			t.Fatal("expected error for invalid address")
		}
	})
}

func TestNodeJS20Spec(t *testing.T) {
	spec := nodeJS20Spec()

	t.Run("cipher suites", func(t *testing.T) {
		if len(spec.CipherSuites) == 0 {
			t.Fatal("expected non-empty cipher suites")
		}
	})

	t.Run("extensions", func(t *testing.T) {
		if len(spec.Extensions) == 0 {
			t.Fatal("expected non-empty extensions")
		}
	})

	t.Run("ALPN is http/1.1", func(t *testing.T) {
		var foundALPN bool
		for _, ext := range spec.Extensions {
			if alpn, ok := ext.(*utls.ALPNExtension); ok {
				foundALPN = true
				if len(alpn.AlpnProtocols) != 1 || alpn.AlpnProtocols[0] != "http/1.1" {
					t.Fatalf("expected ALPN to be [http/1.1], got %v", alpn.AlpnProtocols)
				}
			}
		}
		if !foundALPN {
			t.Fatal("expected ALPN extension")
		}
	})
}
