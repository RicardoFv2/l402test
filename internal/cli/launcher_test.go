package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLauncherItemsReflectConfigurationState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		state        launcherState
		firstAction  launcherAction
		disabled     []launcherAction
		wantContains []string
	}{
		{
			name:        "missing configuration",
			state:       launcherState{configPath: "l402test.yaml", themeName: "nord"},
			firstAction: launcherActionInit,
			disabled:    []launcherAction{launcherActionStart, launcherActionValidate},
			wantContains: []string{
				"Create configuration",
				"create l402test.yaml first",
			},
		},
		{
			name:        "ready configuration",
			state:       launcherState{configPath: "l402test.yaml", configReady: true, themeName: "dracula"},
			firstAction: launcherActionStart,
			wantContains: []string{
				"Start local playground",
				"happy",
				"test fixture",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := launcherItems(test.state)
			if items[firstEnabledLauncherItem(items)].action != test.firstAction {
				t.Fatalf("first enabled action = %v, want %v", items[firstEnabledLauncherItem(items)].action, test.firstAction)
			}
			for _, action := range test.disabled {
				if launcherItemEnabled(items, action) {
					t.Fatalf("action %v should be disabled", action)
				}
			}
			joined := launcherItemText(items)
			for _, want := range test.wantContains {
				if !strings.Contains(joined, want) {
					t.Fatalf("launcher items are missing %q: %q", want, joined)
				}
			}
		})
	}
}

func TestLauncherSelectionSkipsDisabledItems(t *testing.T) {
	t.Parallel()
	items := launcherItems(launcherState{configPath: "l402test.yaml"})
	selected := firstEnabledLauncherItem(items)
	next, confirm, cancel := advanceLauncherSelection(items, selected, themePickerKeyDown)
	if confirm || cancel || items[next].action != launcherActionTheme {
		t.Fatalf("down result = (%v, %t, %t), want theme without confirmation", items[next].action, confirm, cancel)
	}
	next, confirm, cancel = advanceLauncherSelection(items, next, themePickerKeyUp)
	if confirm || cancel || items[next].action != launcherActionInit {
		t.Fatalf("up result = (%v, %t, %t), want init without confirmation", items[next].action, confirm, cancel)
	}
	next, confirm, cancel = advanceLauncherSelection(items, next, themePickerKeyEnter)
	if cancel || !confirm || items[next].action != launcherActionInit {
		t.Fatalf("enter result = (%v, %t, %t), want init confirmation", items[next].action, confirm, cancel)
	}
}

func TestInteractiveLauncherLinesUseSelectedTheme(t *testing.T) {
	t.Parallel()
	view, err := newPresentation(new(bytes.Buffer), "always", "dracula")
	if err != nil {
		t.Fatal(err)
	}
	state := launcherState{configPath: "l402test.yaml", configReady: true, themeName: "dracula"}
	lines := strings.Join(interactiveLauncherLines(view, state, launcherItems(state), 0), "\n")
	for _, want := range []string{
		"What do you want to do?",
		"Start local playground",
		"serve --format json",
		"All Cobra commands remain available for scripts and agents.",
		"\x1b[38;2;189;147;249m",
	} {
		if !strings.Contains(lines, want) {
			t.Fatalf("interactive launcher is missing %q: %q", want, lines)
		}
	}
}

func TestLauncherHandoffShowsEquivalentCommand(t *testing.T) {
	t.Parallel()
	output := new(bytes.Buffer)
	view, err := newPresentation(output, "never", defaultTerminalTheme)
	if err != nil {
		t.Fatal(err)
	}
	if err := view.writeLauncherHandoff(output, "l402test validate --config l402test.yaml"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Selected command", "$ l402test validate --config l402test.yaml"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("launcher handoff is missing %q: %q", want, output.String())
		}
	}
}

func TestLauncherStateForDetectsRegularConfiguration(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "l402test.yaml")
	if state := launcherStateFor(path, "nord"); state.configReady {
		t.Fatal("missing configuration should not be ready")
	}
	if err := os.WriteFile(path, []byte(exampleConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if state := launcherStateFor(path, "nord"); !state.configReady {
		t.Fatal("regular configuration should be ready")
	}
}

func launcherItemEnabled(items []launcherItem, action launcherAction) bool {
	for _, item := range items {
		if item.action == action {
			return item.enabled
		}
	}
	return false
}

func launcherItemText(items []launcherItem) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, item.label+" "+item.description)
	}
	return strings.Join(parts, "\n")
}
