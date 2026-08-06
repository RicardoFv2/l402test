// Package l402 provides the small, self-contained pieces of the L402 HTTP
// flow used by the playground. It intentionally never contacts Lightning.
package l402

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/zpay32"
)

// NewBOLT11 creates a signed, structurally valid regtest BOLT11 invoice. It
// has no route to a node and is therefore safe for local tests only.
func NewBOLT11(paymentHash, paymentSecret []byte, amountSats int64, issuedAt time.Time, expiry time.Duration) (string, error) {
	if len(paymentHash) != sha256.Size {
		return "", errors.New("payment hash must be 32 bytes")
	}
	if len(paymentSecret) != sha256.Size {
		return "", errors.New("payment secret must be 32 bytes")
	}
	if amountSats <= 0 {
		return "", errors.New("invoice amount must be greater than zero")
	}
	if uint64(amountSats) > ^uint64(0)/1000 {
		return "", errors.New("invoice amount is too large")
	}
	if issuedAt.Unix() < 0 {
		return "", errors.New("invoice timestamp must be positive")
	}
	if expiry <= 0 {
		return "", errors.New("invoice expiry must be greater than zero")
	}

	var hash [sha256.Size]byte
	copy(hash[:], paymentHash)
	var secret [sha256.Size]byte
	copy(secret[:], paymentSecret)
	invoice, err := zpay32.NewInvoice(
		&chaincfg.RegressionNetParams,
		hash,
		issuedAt,
		zpay32.Amount(lnwire.MilliSatoshi(uint64(amountSats)*1000)),
		zpay32.PaymentAddr(secret),
		zpay32.Description("l402test local playground"),
		zpay32.Expiry(expiry),
		zpay32.CLTVExpiry(18),
	)
	if err != nil {
		return "", fmt.Errorf("create BOLT11 invoice: %w", err)
	}

	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return "", fmt.Errorf("create invoice signing key: %w", err)
	}
	encoded, err := invoice.Encode(zpay32.MessageSigner{
		SignCompact: func(message []byte) ([]byte, error) {
			hash := sha256.Sum256(message)
			return ecdsa.SignCompact(privateKey, hash[:], true), nil
		},
	})
	if err != nil {
		return "", fmt.Errorf("encode BOLT11 invoice: %w", err)
	}
	return encoded, nil
}

func newPaymentSecret() ([]byte, error) {
	secret := make([]byte, sha256.Size)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("create payment secret: %w", err)
	}
	return secret, nil
}
