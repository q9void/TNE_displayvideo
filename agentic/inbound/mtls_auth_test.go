package inbound

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

// generateTestCert produces a self-signed leaf cert with the given
// CommonName. Returns the cert and its SPKI fingerprint so tests can pin
// it. P-256 ECDSA keeps generation fast in CI.
func generateTestCert(t *testing.T, commonName string) (*x509.Certificate, string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, SPKIFingerprint(cert)
}

// ctxWithCert builds a context with the cert installed in peer info, the
// shape gRPC's TLS handshake produces.
func ctxWithCert(cert *x509.Certificate) context.Context {
	state := credentials.TLSInfo{}
	state.State.PeerCertificates = []*x509.Certificate{cert}
	return peer.NewContext(context.Background(), &peer.Peer{
		Addr:     &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
		AuthInfo: state,
	})
}

func newTestMTLSAuth(t *testing.T, cfg MTLSConfig) *MTLSAuthenticator {
	t.Helper()
	if cfg.CARoots == nil {
		cfg.CARoots = x509.NewCertPool()
	}
	a, err := NewMTLSAuthenticator(cfg)
	require.NoError(t, err)
	return a
}

func TestMTLSAuthenticator_acceptsCurrentSPKI(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com", AgentType: "DSP"}},
	})
	auth.AddPinned("curator.example.com", spki, "")

	id, err := auth.Verify(ctxWithCert(cert))
	require.NoError(t, err)
	assert.Equal(t, "curator.example.com", id.AgentID)
	assert.Equal(t, "DSP", id.AgentType)
	assert.Equal(t, spki, id.SPKIFingerprint)
	assert.True(t, id.RegistryVerified, "no registry configured ⇒ verified by default")
}

func TestMTLSAuthenticator_acceptsSuccessorSPKI(t *testing.T) {
	// Simulates a 30-day rotation overlap: cert presents the new key, our
	// table still has the old key as current and the new one as successor.
	rotatedCert, rotatedSPKI := generateTestCert(t, "curator.example.com")
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
	})
	auth.AddPinned("curator.example.com", "deadbeef-old-fingerprint", rotatedSPKI)

	id, err := auth.Verify(ctxWithCert(rotatedCert))
	require.NoError(t, err)
	assert.Equal(t, rotatedSPKI, id.SPKIFingerprint)
}

func TestMTLSAuthenticator_rejectsUnpinnedSPKI(t *testing.T) {
	cert, _ := generateTestCert(t, "curator.example.com")
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
	})
	auth.AddPinned("curator.example.com", "wrong-current", "wrong-successor")

	_, err := auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailedSPKI))
}

func TestMTLSAuthenticator_rejectsAgentMissingFromAllowList(t *testing.T) {
	cert, spki := generateTestCert(t, "stranger.example.com")
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
	})
	auth.AddPinned("stranger.example.com", spki, "")

	_, err := auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestMTLSAuthenticator_rejectsMissingPinEntry(t *testing.T) {
	cert, _ := generateTestCert(t, "curator.example.com")
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
	})
	// Note: no AddPinned call.

	_, err := auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailedSPKI))
}

func TestMTLSAuthenticator_rejectsCertWithoutCommonName(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	auth := newTestMTLSAuth(t, MTLSConfig{})
	_, err = auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestMTLSAuthenticator_rejectsContextWithoutPeer(t *testing.T) {
	auth := newTestMTLSAuth(t, MTLSConfig{})
	_, err := auth.Verify(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestMTLSAuthenticator_rejectsContextWithoutTLS(t *testing.T) {
	// peer present but AuthInfo is nil — no TLS handshake.
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: &net.IPAddr{IP: net.ParseIP("127.0.0.1")},
	})
	auth := newTestMTLSAuth(t, MTLSConfig{})
	_, err := auth.Verify(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailed))
}

func TestMTLSAuthenticator_registryAcceptsListedAgent(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	reg := &StaticRegistryClient{IDs: []string{"curator.example.com"}}
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
		Registry:  reg,
	})
	auth.AddPinned("curator.example.com", spki, "")
	require.NoError(t, auth.RefreshRegistry(context.Background()))

	id, err := auth.Verify(ctxWithCert(cert))
	require.NoError(t, err)
	assert.True(t, id.RegistryVerified)
}

func TestMTLSAuthenticator_registryRejectsUnlistedAgent(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	reg := &StaticRegistryClient{IDs: []string{"someone.else.com"}}
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
		Registry:  reg,
	})
	auth.AddPinned("curator.example.com", spki, "")
	require.NoError(t, auth.RefreshRegistry(context.Background()))

	_, err := auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailedRegistry))
}

func TestMTLSAuthenticator_openWindowAllowsColdStart(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	reg := &StaticRegistryClient{IDs: nil} // never refreshed
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList:  []AgentEntry{{AgentID: "curator.example.com"}},
		Registry:   reg,
		OpenWindow: true,
	})
	auth.AddPinned("curator.example.com", spki, "")
	// Note: no RefreshRegistry call — regOK stays false.

	id, err := auth.Verify(ctxWithCert(cert))
	require.NoError(t, err)
	assert.False(t, id.RegistryVerified, "OpenWindow accepts call but flags registry as unverified")
}

func TestMTLSAuthenticator_strictMode_rejectsBeforeFirstRefresh(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	reg := &StaticRegistryClient{IDs: []string{"curator.example.com"}}
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList:  []AgentEntry{{AgentID: "curator.example.com"}},
		Registry:   reg,
		OpenWindow: false,
	})
	auth.AddPinned("curator.example.com", spki, "")
	// Deliberately skip RefreshRegistry: regOK=false, OpenWindow=false ⇒ reject.

	_, err := auth.Verify(ctxWithCert(cert))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrAuthFailedRegistry))
}

func TestMTLSAuthenticator_RefreshRegistry_swapsAtomically(t *testing.T) {
	cert, spki := generateTestCert(t, "curator.example.com")
	reg := &StaticRegistryClient{IDs: []string{"someone.else.com"}}
	auth := newTestMTLSAuth(t, MTLSConfig{
		AllowList: []AgentEntry{{AgentID: "curator.example.com"}},
		Registry:  reg,
	})
	auth.AddPinned("curator.example.com", spki, "")
	require.NoError(t, auth.RefreshRegistry(context.Background()))

	// Initially unlisted ⇒ rejected.
	_, err := auth.Verify(ctxWithCert(cert))
	assert.True(t, errors.Is(err, ErrAuthFailedRegistry))

	// Swap the registry contents and refresh — now accepted.
	reg.IDs = []string{"curator.example.com"}
	require.NoError(t, auth.RefreshRegistry(context.Background()))
	_, err = auth.Verify(ctxWithCert(cert))
	require.NoError(t, err)
}

func TestMTLSAuthenticator_RefreshRegistry_nilClientNoop(t *testing.T) {
	auth := newTestMTLSAuth(t, MTLSConfig{}) // no registry
	assert.NoError(t, auth.RefreshRegistry(context.Background()))
}

func TestMTLSAuthenticator_RefreshRegistry_propagatesError(t *testing.T) {
	want := errors.New("upstream down")
	auth := newTestMTLSAuth(t, MTLSConfig{
		Registry: &erroringRegistry{err: want},
	})
	err := auth.RefreshRegistry(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

func TestNewMTLSAuthenticator_requiresCARoots(t *testing.T) {
	_, err := NewMTLSAuthenticator(MTLSConfig{})
	require.Error(t, err)
}

func TestSPKIFingerprint_stableAcrossParse(t *testing.T) {
	cert, spki := generateTestCert(t, "x")
	assert.Equal(t, spki, SPKIFingerprint(cert))
	// Re-parsing the same DER produces the same fingerprint.
	again, err := x509.ParseCertificate(cert.Raw)
	require.NoError(t, err)
	assert.Equal(t, spki, SPKIFingerprint(again))
}

func TestSPKIFingerprint_nilSafe(t *testing.T) {
	assert.Equal(t, "", SPKIFingerprint(nil))
}

// erroringRegistry is a RegistryClient that always errors. Used to prove
// RefreshRegistry surfaces the upstream error verbatim.
type erroringRegistry struct{ err error }

func (e *erroringRegistry) VerifiedSellerEligible(context.Context) ([]string, error) {
	return nil, e.err
}
