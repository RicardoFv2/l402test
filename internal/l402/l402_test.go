package l402

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/zpay32"
)

func TestBOLT11DecodesWithLND(t *testing.T) {
	t.Parallel()
	paymentHash := sha256.Sum256([]byte("payment"))
	paymentSecret := sha256.Sum256([]byte("payment-secret"))
	invoice, err := NewBOLT11(paymentHash[:], paymentSecret[:], 21, time.Unix(1_700_000_000, 0), time.Minute)
	if err != nil {
		t.Fatalf("NewBOLT11() error = %v", err)
	}
	decoded, err := zpay32.Decode(invoice, &chaincfg.RegressionNetParams)
	if err != nil {
		t.Fatalf("zpay32.Decode() error = %v", err)
	}
	if decoded.PaymentHash == nil || !bytes.Equal(decoded.PaymentHash[:], paymentHash[:]) {
		t.Fatalf("payment hash = %x", decoded.PaymentHash)
	}
	if !decoded.PaymentAddr.IsSome() || decoded.PaymentAddr.UnwrapOr([32]byte{}) != paymentSecret {
		t.Fatal("invoice does not contain the expected payment secret")
	}
	if decoded.MilliSat == nil || *decoded.MilliSat != 21_000 {
		t.Fatalf("amount = %v, want 21000 msat", decoded.MilliSat)
	}
	if decoded.Expiry() != time.Minute {
		t.Fatalf("expiry = %s, want %s", decoded.Expiry(), time.Minute)
	}
	if decoded.MinFinalCLTVExpiry() != 18 {
		t.Fatalf("minimum final CLTV = %d, want 18", decoded.MinFinalCLTVExpiry())
	}
}

func TestIssuerVerifiesChallenge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	issuer, err := NewIssuer(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := issuer.Issue("GET /premium", 21, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := issuer.VerifyAuthorization(challenge.AuthorizationValue(), challenge); err != nil {
		t.Fatalf("VerifyAuthorization() error = %v", err)
	}
	if err := issuer.VerifyAuthorization("L402 "+challenge.Macaroon+":"+strings.Repeat("00", 32), challenge); err == nil {
		t.Fatal("VerifyAuthorization() accepted an incorrect preimage")
	}
}

func TestIssuerRejectsExpiredChallenge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	issuer, err := NewIssuer(func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := issuer.Issue("GET /premium", 21, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if err := issuer.VerifyAuthorization(challenge.AuthorizationValue(), challenge); err == nil {
		t.Fatal("VerifyAuthorization() accepted an expired credential")
	}
}

func TestParseWWWAuthenticate(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name    string
		header  string
		wantErr bool
	}{
		{name: "valid", header: `L402 macaroon="mac", invoice="lnbcrt1invoice"`},
		{name: "parameters may be reordered", header: `l402 invoice="lnbcrt1invoice", macaroon="mac"`},
		{name: "missing scheme", header: `macaroon="mac", invoice="lnbcrt1invoice"`, wantErr: true},
		{name: "unquoted value", header: `L402 macaroon=mac, invoice="lnbcrt1invoice"`, wantErr: true},
		{name: "missing invoice", header: `L402 macaroon="mac"`, wantErr: true},
		{name: "duplicate parameter", header: `L402 macaroon="mac", macaroon="again", invoice="lnbcrt1invoice"`, wantErr: true},
		{name: "unterminated", header: `L402 macaroon="mac`, wantErr: true},
		{name: "trailing separator", header: `L402 macaroon="mac", invoice="lnbcrt1invoice",`, wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			macaroonValue, invoice, err := ParseWWWAuthenticate(testCase.header)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("ParseWWWAuthenticate() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWWWAuthenticate() error = %v", err)
			}
			if macaroonValue != "mac" || invoice != "lnbcrt1invoice" {
				t.Fatalf("ParseWWWAuthenticate() = %q, %q", macaroonValue, invoice)
			}
		})
	}
}

func FuzzParseAuthorization(f *testing.F) {
	f.Add("L402 YWJj:0011")
	f.Add("")
	f.Add("Bearer token")
	f.Fuzz(func(t *testing.T, header string) {
		_, _, _ = ParseAuthorization(header)
	})
}

func FuzzParseWWWAuthenticate(f *testing.F) {
	f.Add(`L402 macaroon="mac", invoice="lnbcrt1invoice"`)
	f.Add(`L402 macaroon="unterminated`)
	f.Add("")
	f.Fuzz(func(t *testing.T, header string) {
		_, _, _ = ParseWWWAuthenticate(header)
	})
}
