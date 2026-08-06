package l402

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/macaroon.v2"
)

const scheme = "L402"

// Challenge is the credential pair returned in a WWW-Authenticate header.
// Preimage is deliberately for test fixtures only and must never be logged.
type Challenge struct {
	Macaroon  string
	Invoice   string
	Preimage  string
	ExpiresAt time.Time
	Resource  string
}

// Issuer mints test-only macaroons that bind a resource to a payment hash.
type Issuer struct {
	rootKey []byte
	now     func() time.Time
}

func NewIssuer(now func() time.Time) (*Issuer, error) {
	if now == nil {
		now = time.Now
	}
	rootKey := make([]byte, 32)
	if _, err := rand.Read(rootKey); err != nil {
		return nil, fmt.Errorf("create macaroon root key: %w", err)
	}
	return &Issuer{rootKey: rootKey, now: now}, nil
}

func (issuer *Issuer) Issue(resource string, priceSats int64, expiresAt time.Time) (Challenge, error) {
	preimage := make([]byte, sha256.Size)
	if _, err := rand.Read(preimage); err != nil {
		return Challenge{}, fmt.Errorf("create payment preimage: %w", err)
	}
	paymentHash := sha256.Sum256(preimage)
	paymentSecret, err := newPaymentSecret()
	if err != nil {
		return Challenge{}, err
	}
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return Challenge{}, fmt.Errorf("create macaroon identifier: %w", err)
	}
	credential, err := macaroon.New(issuer.rootKey, identifier, "l402test", macaroon.V2)
	if err != nil {
		return Challenge{}, fmt.Errorf("create macaroon: %w", err)
	}
	caveats := []string{
		"payment_hash=" + hex.EncodeToString(paymentHash[:]),
		"resource=" + resource,
		fmt.Sprintf("expires_at=%d", expiresAt.Unix()),
	}
	for _, caveat := range caveats {
		if err := credential.AddFirstPartyCaveat([]byte(caveat)); err != nil {
			return Challenge{}, fmt.Errorf("add macaroon caveat: %w", err)
		}
	}
	encoded, err := credential.MarshalBinary()
	if err != nil {
		return Challenge{}, fmt.Errorf("encode macaroon: %w", err)
	}
	issuedAt := issuer.now()
	invoiceExpiry := expiresAt.Sub(issuedAt)
	// An expired scenario still needs a syntactically valid invoice. Backdate its
	// creation time instead of trying to encode a negative BOLT11 expiry.
	if invoiceExpiry <= 0 {
		invoiceExpiry = time.Minute
		issuedAt = expiresAt.Add(-invoiceExpiry)
	}
	invoice, err := NewBOLT11(paymentHash[:], paymentSecret, priceSats, issuedAt, invoiceExpiry)
	if err != nil {
		return Challenge{}, fmt.Errorf("create BOLT11 invoice: %w", err)
	}
	return Challenge{
		Macaroon:  base64.StdEncoding.EncodeToString(encoded),
		Invoice:   invoice,
		Preimage:  hex.EncodeToString(preimage),
		ExpiresAt: expiresAt,
		Resource:  resource,
	}, nil
}

// WWWAuthenticate formats the documented HTTP challenge value.
func (challenge Challenge) WWWAuthenticate() string {
	return fmt.Sprintf(`%s macaroon="%s", invoice="%s"`, scheme, challenge.Macaroon, challenge.Invoice)
}

// ParseWWWAuthenticate extracts the macaroon and BOLT11 invoice from an L402
// challenge. It accepts parameter order variation but rejects malformed,
// duplicate, or incomplete credentials.
func ParseWWWAuthenticate(header string) (string, string, error) {
	schemeName, parameters, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(schemeName, scheme) {
		return "", "", errors.New("missing L402 challenge")
	}

	values := make(map[string]string, 2)
	for parameters = strings.TrimSpace(parameters); parameters != ""; parameters = strings.TrimSpace(parameters) {
		name, rest, found := strings.Cut(parameters, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" || !strings.HasPrefix(rest, `"`) {
			return "", "", errors.New("invalid L402 challenge parameter")
		}
		rest = rest[1:]
		endQuote := strings.IndexByte(rest, '"')
		if endQuote < 0 {
			return "", "", errors.New("unterminated L402 challenge parameter")
		}
		value := rest[:endQuote]
		if value == "" {
			return "", "", errors.New("empty L402 challenge parameter")
		}
		if _, exists := values[name]; exists {
			return "", "", errors.New("duplicate L402 challenge parameter")
		}
		values[name] = value

		parameters = strings.TrimSpace(rest[endQuote+1:])
		if parameters == "" {
			break
		}
		if !strings.HasPrefix(parameters, ",") {
			return "", "", errors.New("invalid L402 challenge separator")
		}
		parameters = parameters[1:]
		if strings.TrimSpace(parameters) == "" {
			return "", "", errors.New("trailing L402 challenge separator")
		}
	}

	macaroonValue, macaroonFound := values["macaroon"]
	invoice, invoiceFound := values["invoice"]
	if !macaroonFound || !invoiceFound {
		return "", "", errors.New("incomplete L402 challenge")
	}
	return macaroonValue, invoice, nil
}

// VerifyAuthorization verifies a client Authorization header against one
// challenge without storing server-side payment state.
func (issuer *Issuer) VerifyAuthorization(header string, challenge Challenge) error {
	macaroonValue, preimage, err := ParseAuthorization(header)
	if err != nil {
		return err
	}
	if preimage != challenge.Preimage {
		return errors.New("payment preimage does not match challenge")
	}
	encoded, err := base64.StdEncoding.DecodeString(macaroonValue)
	if err != nil {
		return errors.New("invalid L402 macaroon encoding")
	}
	var credential macaroon.Macaroon
	if err := credential.UnmarshalBinary(encoded); err != nil {
		return errors.New("invalid L402 macaroon")
	}
	return credential.Verify(issuer.rootKey, func(caveat string) error {
		key, value, found := strings.Cut(caveat, "=")
		if !found {
			return errors.New("invalid macaroon caveat")
		}
		switch key {
		case "payment_hash":
			decoded, err := hex.DecodeString(preimage)
			if err != nil {
				return errors.New("invalid L402 preimage")
			}
			hash := sha256.Sum256(decoded)
			if value != hex.EncodeToString(hash[:]) {
				return errors.New("payment hash caveat failed")
			}
		case "resource":
			if value != challenge.Resource {
				return errors.New("resource caveat failed")
			}
		case "expires_at":
			if issuer.now().Unix() > challenge.ExpiresAt.Unix() {
				return errors.New("L402 credential expired")
			}
		default:
			return errors.New("unsupported macaroon caveat")
		}
		return nil
	}, nil)
}

// ParseAuthorization returns the base64 macaroon and hexadecimal preimage.
func ParseAuthorization(header string) (string, string, error) {
	if !strings.HasPrefix(header, scheme+" ") {
		return "", "", errors.New("missing L402 authorization")
	}
	value := strings.TrimPrefix(header, scheme+" ")
	macaroonValue, preimage, found := strings.Cut(value, ":")
	if !found || macaroonValue == "" || preimage == "" || strings.Contains(preimage, ":") {
		return "", "", errors.New("invalid L402 authorization")
	}
	if _, err := hex.DecodeString(preimage); err != nil {
		return "", "", errors.New("invalid L402 preimage")
	}
	return macaroonValue, preimage, nil
}

func (challenge Challenge) AuthorizationValue() string {
	return scheme + " " + challenge.Macaroon + ":" + challenge.Preimage
}
