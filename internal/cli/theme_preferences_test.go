package cli

import (
	"bufio"
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestThemePickerSelectsByNumber(t *testing.T) {
	t.Parallel()
	output := new(bytes.Buffer)
	selected, err := chooseTerminalTheme(strings.NewReader("2\n"), output, true)
	if err != nil {
		t.Fatal(err)
	}
	if selected != "dracula" {
		t.Fatalf("selected theme = %q, want dracula", selected)
	}
	if !strings.Contains(output.String(), "Tokyo Night") || !strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("theme picker output = %q", output.String())
	}
}

func TestThemePickerArrowKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  themePickerKey
	}{
		{name: "ANSI up", input: "\x1b[A", want: themePickerKeyUp},
		{name: "ANSI down", input: "\x1b[B", want: themePickerKeyDown},
		{name: "Windows up", input: "\xe0H", want: themePickerKeyUp},
		{name: "Windows down", input: "\x00P", want: themePickerKeyDown},
		{name: "enter", input: "\r", want: themePickerKeyEnter},
		{name: "escape", input: "\x1b", want: themePickerKeyCancel},
		{name: "control c", input: "\x03", want: themePickerKeyCancel},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, err := readThemePickerKey(bufio.NewReader(strings.NewReader(test.input)))
			if err != nil {
				t.Fatal(err)
			}
			if key != test.want {
				t.Fatalf("key = %v, want %v", key, test.want)
			}
		})
	}
}

func TestThemePickerSelectionWrapsAndRendersPreview(t *testing.T) {
	t.Parallel()
	next, confirm, canceled := advanceThemePickerSelection(0, themePickerKeyUp)
	if next != len(terminalThemeChoices)-1 || confirm || canceled {
		t.Fatalf("up result = (%d, %t, %t)", next, confirm, canceled)
	}
	next, confirm, canceled = advanceThemePickerSelection(1, themePickerKeyEnter)
	if next != 1 || !confirm || canceled {
		t.Fatalf("enter result = (%d, %t, %t)", next, confirm, canceled)
	}
	preview := strings.Join(interactiveThemePickerLines(1, true), "\n")
	if !strings.Contains(preview, "Preview · Dracula") || !strings.Contains(preview, "curl -i http://127.0.0.1:53142/v1/weather") {
		t.Fatalf("interactive preview = %q", preview)
	}
	if !strings.Contains(preview, "\x1b[38;2;189;147;249m") {
		t.Fatalf("Dracula preview did not use its accent: %q", preview)
	}
}

func TestThemePreferencesStore(t *testing.T) {
	t.Parallel()
	store := themePreferencesStore{path: filepath.Join(t.TempDir(), "theme.json")}
	if err := store.Save("catppuccin"); err != nil {
		t.Fatal(err)
	}
	stored, found, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found || stored != "catppuccin-mocha" {
		t.Fatalf("stored theme = %q, found = %t", stored, found)
	}
}

func TestThemeCommandSavesExplicitTheme(t *testing.T) {
	t.Parallel()
	store := themePreferencesStore{path: filepath.Join(t.TempDir(), "theme.json")}
	root := newRoot(BuildInfo{}, &outputOptions{preferences: store})
	output := new(bytes.Buffer)
	root.SetOut(output)
	root.SetArgs([]string{"--theme", "nord", "theme"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	stored, found, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !found || stored != "nord" {
		t.Fatalf("stored theme = %q, found = %t", stored, found)
	}
	if !strings.Contains(output.String(), "[SAVED]") {
		t.Fatalf("theme command output = %q", output.String())
	}
}
