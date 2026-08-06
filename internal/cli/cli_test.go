package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RicardoFv2/l402test/internal/fixture"
	"github.com/spf13/cobra"
)

func TestInitValidateAndVersion(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	configPath := filepath.Join(directory, "l402test.yaml")
	root := newTestRoot(t, BuildInfo{Version: "v0.1.0", Commit: "abc", Date: "2026-08-05"})
	output := new(bytes.Buffer)
	root.SetOut(output)
	root.SetArgs([]string{"init", "--output", configPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("init error = %v", err)
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("configuration not written: %v", err)
	}
	if !strings.Contains(output.String(), "[WRITTEN]") {
		t.Fatalf("init output = %q", output.String())
	}

	output.Reset()
	root = newTestRoot(t, BuildInfo{})
	root.SetOut(output)
	root.SetArgs([]string{"validate", "--config", configPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("validate error = %v", err)
	}
	if !strings.Contains(output.String(), "[VALID]") || !strings.Contains(output.String(), "ENDPOINTS   1") {
		t.Fatalf("validate output = %q", output.String())
	}

	output.Reset()
	root = newTestRoot(t, BuildInfo{Version: "v0.1.0", Commit: "abc", Date: "2026-08-05"})
	root.SetOut(output)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version error = %v", err)
	}
	if !strings.Contains(output.String(), "l402test / build information") || !strings.Contains(output.String(), "v0.1.0") {
		t.Fatalf("version output = %q", output.String())
	}
}

func TestWriteReadyJSON(t *testing.T) {
	t.Parallel()
	output := new(bytes.Buffer)
	view, err := newPresentation(output, "always", defaultTerminalTheme)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.writeReady(output, "json", "http://127.0.0.1:9000", "happy", "fixture.json", "curl -i http://127.0.0.1:9000/v1/weather"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"event":"ready"`) || strings.Contains(output.String(), "preimage") || strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("readiness JSON = %q", output.String())
	}
}

func TestWriteReadyText(t *testing.T) {
	t.Parallel()
	output := new(bytes.Buffer)
	request := "curl -i http://127.0.0.1:9000/v1/weather"
	view, err := newPresentation(output, "never", defaultTerminalTheme)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.writeReady(output, "text", "http://127.0.0.1:9000", "happy", "fixture.json", request); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"l402test / local L402 HTTP playground", "[READY]", "SCENARIO    happy", request, "Press Ctrl+C to stop."} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("text readiness is missing %q: %q", want, output.String())
		}
	}
}

func TestTextPresentationCanUseColorWithoutAffectingJSON(t *testing.T) {
	t.Parallel()
	output := new(bytes.Buffer)
	view, err := newPresentation(output, "always", defaultTerminalTheme)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.writeReady(output, "text", "http://127.0.0.1:9000", "happy", "fixture.json", "curl -i http://127.0.0.1:9000/v1/weather"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\x1b[") || !strings.Contains(output.String(), "READY") {
		t.Fatalf("color text readiness = %q", output.String())
	}

	output.Reset()
	if err := view.writeReady(output, "json", "http://127.0.0.1:9000", "happy", "fixture.json", "curl -i http://127.0.0.1:9000/v1/weather"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("color JSON readiness = %q", output.String())
	}
}

func TestAutoColorIsDisabledForNonTerminalWriter(t *testing.T) {
	t.Parallel()
	view, err := newPresentation(new(bytes.Buffer), "auto", defaultTerminalTheme)
	if err != nil {
		t.Fatal(err)
	}
	if view.color {
		t.Fatal("auto color should be disabled for a non-terminal writer")
	}
}

func TestTerminalThemes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		accentCode string
	}{
		{name: "tokyo-night", accentCode: "\x1b[38;2;122;162;247m"},
		{name: "dracula", accentCode: "\x1b[38;2;189;147;249m"},
		{name: "catppuccin-mocha", accentCode: "\x1b[38;2;203;166;247m"},
		{name: "nord", accentCode: "\x1b[38;2;136;192;208m"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			view, err := newPresentation(new(bytes.Buffer), "always", test.name)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(view.header("local L402 HTTP playground"), test.accentCode) {
				t.Fatalf("theme %q did not render its accent", test.name)
			}
		})
	}
}

func TestTerminalThemeAliasAndValidation(t *testing.T) {
	t.Parallel()
	view, err := newPresentation(new(bytes.Buffer), "always", "catppuccin")
	if err != nil {
		t.Fatal(err)
	}
	if view.theme.name != "catppuccin-mocha" {
		t.Fatalf("alias resolved to %q", view.theme.name)
	}
	if _, err := newPresentation(new(bytes.Buffer), "always", "unknown"); err == nil {
		t.Fatal("unknown theme did not return an error")
	}
}

func TestThemeFlagChangesCLIOutput(t *testing.T) {
	t.Parallel()
	root := newTestRoot(t, BuildInfo{})
	output := new(bytes.Buffer)
	root.SetOut(output)
	root.SetArgs([]string{"--color", "always", "--theme", "nord", "version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\x1b[38;2;136;192;208m") {
		t.Fatalf("Nord accent was not rendered: %q", output.String())
	}
}

func TestRootWithoutCommandShowsHelp(t *testing.T) {
	t.Parallel()
	root := newTestRoot(t, BuildInfo{})
	output := new(bytes.Buffer)
	root.SetOut(output)
	root.SetArgs(nil)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Start local L402-protected HTTP endpoints without a wallet, node, or funds.") {
		t.Fatalf("root help = %q", output.String())
	}
}

func TestServeWritesFixtureAndShutsDown(t *testing.T) {
	directory := t.TempDir()
	configPath := filepath.Join(directory, "l402test.yaml")
	fixturePath := filepath.Join(directory, "fixture.json")
	if err := os.WriteFile(configPath, []byte(exampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	context, cancel := context.WithCancel(context.Background())
	defer cancel()
	root := newRoot(BuildInfo{}, &outputOptions{
		preferences: themePreferencesStore{pathErr: errors.New("JSON mode must not read theme preferences")},
	})
	root.SetContext(context)
	root.SetOut(new(bytes.Buffer))
	root.SetArgs([]string{"serve", "--config", configPath, "--fixture-out", fixturePath, "--format", "json"})
	done := make(chan error, 1)
	go func() { done <- root.Execute() }()

	var result fixture.File
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		contents, err := os.ReadFile(fixturePath)
		if err == nil && json.Unmarshal(contents, &result) == nil && result.BaseURL != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if result.BaseURL == "" {
		t.Fatal("serve did not write a usable fixture")
	}
	response, err := http.Get(result.BaseURL + "/v1/weather")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("challenge status = %d", response.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not shut down")
	}
}

func newTestRoot(t *testing.T, build BuildInfo) *cobra.Command {
	t.Helper()
	return newRoot(build, &outputOptions{
		preferences: themePreferencesStore{path: filepath.Join(t.TempDir(), "theme.json")},
	})
}
