// Package fixture writes the intentionally non-production credentials consumed
// by test wallets and client harnesses.
package fixture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// File is written by `l402test serve`. It deliberately includes only test
// preimages: clients receive the macaroon and invoice from the HTTP challenge.
type File struct {
	Version   int        `json:"version"`
	Scenario  string     `json:"scenario"`
	BaseURL   string     `json:"base_url"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	Name      string   `json:"name"`
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	PriceSats int64    `json:"price_sats"`
	Preimages []string `json:"preimages"`
}

// Write atomically creates a private fixture file. The caller controls its
// location so CI can use a temporary directory and keep it outside Git.
func Write(filename string, value File) error {
	if filename == "" {
		return fmt.Errorf("fixture output path is required")
	}
	directory := filepath.Dir(filename)
	if _, err := os.Stat(directory); err != nil {
		return fmt.Errorf("fixture output directory: %w", err)
	}
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode fixture: %w", err)
	}
	contents = append(contents, '\n')

	temporary, err := os.CreateTemp(directory, ".l402test-*")
	if err != nil {
		return fmt.Errorf("create fixture: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect fixture: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write fixture: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close fixture: %w", err)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return fmt.Errorf("publish fixture: %w", err)
	}
	return nil
}
