package mail

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A bounded, local-only IMAP fixture. All protocol traffic is carried over TLS.
func startSelfSignedIMAP(t *testing.T, rejectLogin bool) (int, *atomic.Int32) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "Knot test mailbox"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
	})
	if err != nil {
		t.Fatal(err)
	}
	logins := &atomic.Int32{}
	done := make(chan struct{})
	// Tests disconnect before opening the next connection, so a serial server suffices.
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(5 * time.Second))
				if err := conn.(*tls.Conn).Handshake(); err != nil {
					return
				}
				fmt.Fprint(conn, "* OK [CAPABILITY IMAP4rev1] ready\r\n")
				scanner := bufio.NewScanner(conn)
				for scanner.Scan() {
					fields := strings.Fields(scanner.Text())
					if len(fields) < 2 {
						return
					}
					tag := fields[0]
					switch strings.ToUpper(fields[1]) {
					case "CAPABILITY":
						fmt.Fprintf(conn, "* CAPABILITY IMAP4rev1\r\n%s OK capabilities\r\n", tag)
					case "LOGIN":
						logins.Add(1)
						if rejectLogin {
							fmt.Fprintf(conn, "%s NO authentication failed\r\n", tag)
						} else {
							fmt.Fprintf(conn, "%s OK logged in\r\n", tag)
						}
					case "SELECT":
						fmt.Fprintf(conn, "* FLAGS ()\r\n* 0 EXISTS\r\n* 0 RECENT\r\n%s OK [READ-WRITE] selected\r\n", tag)
					case "NOOP":
						// Force the application's reconnect path to establish a new TLS session.
						return
					case "LOGOUT":
						fmt.Fprintf(conn, "* BYE closing\r\n%s OK logout\r\n", tag)
						return
					default:
						fmt.Fprintf(conn, "%s BAD unsupported\r\n", tag)
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		listener.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("test IMAP server did not stop")
		}
	})
	return listener.Addr().(*net.TCPAddr).Port, logins
}

func TestCertificateCompatibilityAndReconnect(t *testing.T) {
	port, logins := startSelfSignedIMAP(t, false)
	strict := NewMailClient("127.0.0.1", port, "test-user", "test-password", true, false)
	err := strict.Connect()
	var verificationError *tls.CertificateVerificationError
	if !errors.As(err, &verificationError) {
		t.Fatalf("strict mode must reject the self-signed certificate, got %v", err)
	}
	if strict.conn != nil || logins.Load() != 0 {
		t.Fatal("strict mode must not log in or retain a failed connection")
	}
	if !strings.Contains(err.Error(), "内网兼容") {
		t.Fatalf("missing actionable certificate diagnostic: %v", err)
	}

	compat := NewMailClient("127.0.0.1", port, "test-user", "test-password", true, true)
	defer compat.Disconnect()
	if err := compat.Connect(); err != nil {
		t.Fatalf("explicit compatibility mode should connect: %v", err)
	}
	if !compat.conn.IsTLS() {
		t.Fatal("compatibility mode must retain TLS encryption")
	}
	if err := compat.ensureConnection(); err != nil {
		t.Fatalf("reconnect must retain the mailbox's certificate policy: %v", err)
	}
	if logins.Load() != 2 {
		t.Fatalf("expected login and reconnect, got %d logins", logins.Load())
	}
	compat.Disconnect()

	if err := strict.Connect(); !errors.As(err, &verificationError) {
		t.Fatalf("another mailbox's compatibility mode must not affect strict mode: %v", err)
	}
	if logins.Load() != 2 {
		t.Fatal("strict mode must not automatically retry without certificate verification")
	}
}

func TestCompatibilityStillRejectsLoginFailure(t *testing.T) {
	port, _ := startSelfSignedIMAP(t, true)
	mc := NewMailClient("127.0.0.1", port, "test-user", "wrong-password", true, true)
	err := mc.Connect()
	if err == nil || !strings.Contains(err.Error(), "login error:") {
		t.Fatalf("expected a login failure, got %v", err)
	}
	if mc.conn != nil {
		t.Fatal("failed authentication must not retain a connection")
	}
}
