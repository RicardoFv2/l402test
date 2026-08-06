/*
Hallmark · terminal component: action launcher · genre: modern-minimal · theme: user-selected editor palette
Audience: agent and SDK developers · use: start local L402 test work without memorizing a command · tone: technical, austere
States: default · navigate · active action · disabled action · running handoff · error handoff · success handoff · exit
Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V5
*/
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

var errInteractiveLauncherUnavailable = errors.New("interactive launcher unavailable")

const (
	defaultLauncherConfigPath  = "l402test.yaml"
	defaultLauncherFixturePath = ".fixtures/fixture.json"
)

type launcherAction uint8

const (
	launcherActionExit launcherAction = iota
	launcherActionStart
	launcherActionInit
	launcherActionValidate
	launcherActionTheme
	launcherActionHelp
)

type launcherItem struct {
	action      launcherAction
	label       string
	description string
	enabled     bool
}

type launcherState struct {
	configPath  string
	configReady bool
	themeName   string
}

func launcherStateFor(configPath, themeName string) launcherState {
	state := launcherState{configPath: configPath, themeName: themeName}
	info, err := os.Stat(configPath)
	state.configReady = err == nil && info.Mode().IsRegular()
	return state
}

func launcherItems(state launcherState) []launcherItem {
	items := make([]launcherItem, 0, 6)
	if state.configReady {
		items = append(items,
			launcherItem{action: launcherActionStart, label: "Start local playground", description: "happy · local port · test fixture", enabled: true},
			launcherItem{action: launcherActionValidate, label: "Validate configuration", description: state.configPath, enabled: true},
		)
	} else {
		items = append(items,
			launcherItem{action: launcherActionInit, label: "Create configuration", description: "write " + state.configPath, enabled: true},
			launcherItem{action: launcherActionStart, label: "Start local playground", description: "create " + state.configPath + " first", enabled: false},
			launcherItem{action: launcherActionValidate, label: "Validate configuration", description: "create " + state.configPath + " first", enabled: false},
		)
	}
	return append(items,
		launcherItem{action: launcherActionTheme, label: "Change terminal theme", description: state.themeName, enabled: true},
		launcherItem{action: launcherActionHelp, label: "Show CLI commands", description: "flags and automation usage", enabled: true},
		launcherItem{action: launcherActionExit, label: "Exit", description: "return to your shell", enabled: true},
	)
}

func runInteractiveLauncher(input *os.File, output io.Writer, view presentation, state launcherState) (launcherAction, error) {
	terminalState, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return launcherActionExit, fmt.Errorf("%w: %v", errInteractiveLauncherUnavailable, err)
	}
	defer func() { _ = term.Restore(int(input.Fd()), terminalState) }()

	if _, err := io.WriteString(output, ansiAlternateScreen+ansiHideCursor); err != nil {
		return launcherActionExit, err
	}
	defer func() { _, _ = io.WriteString(output, ansiShowCursor+ansiMainScreen) }()

	items := launcherItems(state)
	selected := firstEnabledLauncherItem(items)
	reader := bufio.NewReader(input)
	for {
		if err := redrawInteractiveLauncher(output, view, state, items, selected); err != nil {
			return launcherActionExit, err
		}
		key, err := readThemePickerKey(reader)
		if err != nil {
			return launcherActionExit, err
		}
		next, confirm, cancel := advanceLauncherSelection(items, selected, key)
		if cancel {
			return launcherActionExit, nil
		}
		if confirm {
			return items[selected].action, nil
		}
		selected = next
	}
}

func redrawInteractiveLauncher(output io.Writer, view presentation, state launcherState, items []launcherItem, selected int) error {
	if _, err := io.WriteString(output, ansiCursorHome+ansiClearScreen); err != nil {
		return err
	}
	return writeLines(output, interactiveLauncherLines(view, state, items, selected)...)
}

func interactiveLauncherLines(view presentation, state launcherState, items []launcherItem, selected int) []string {
	configState := state.configPath + " · not found"
	if state.configReady {
		configState = state.configPath + " · ready"
	}
	lines := []string{
		view.header("local L402 playground"),
		view.rule(),
		"",
		view.heading("Test paid HTTP flows locally."),
		view.note("402 → BOLT11 → L402 retry. No wallet. No node. No funds."),
		"",
		view.field("config", configState),
		view.field("theme", state.themeName),
		view.field("automation", "serve --format json"),
		"",
		view.heading("What do you want to do?"),
	}
	for index, item := range items {
		lines = append(lines, formatLauncherItem(view, item, index == selected))
	}
	return append(lines,
		"",
		view.rule(),
		view.note("↑ ↓ navigate · Enter select · Esc exit"),
		view.note("All Cobra commands remain available for scripts and agents."),
	)
}

func formatLauncherItem(view presentation, item launcherItem, selected bool) string {
	if !item.enabled {
		return view.muted("    " + item.label + "  ·  " + item.description)
	}
	if selected {
		return "  " + view.accent("›") + " " + view.panel(" run ") + " " + view.bold(item.label) + view.muted("  ·  "+item.description)
	}
	return "    " + view.text(item.label) + view.muted("  ·  "+item.description)
}

func firstEnabledLauncherItem(items []launcherItem) int {
	for index, item := range items {
		if item.enabled {
			return index
		}
	}
	return 0
}

func advanceLauncherSelection(items []launcherItem, selected int, key themePickerKey) (next int, confirm, cancel bool) {
	if len(items) == 0 {
		return 0, false, true
	}
	if selected < 0 || selected >= len(items) || !items[selected].enabled {
		selected = firstEnabledLauncherItem(items)
	}
	switch key {
	case themePickerKeyUp:
		return nextEnabledLauncherItem(items, selected, -1), false, false
	case themePickerKeyDown:
		return nextEnabledLauncherItem(items, selected, 1), false, false
	case themePickerKeyEnter:
		return selected, true, false
	case themePickerKeyCancel:
		return selected, false, true
	default:
		return selected, false, false
	}
}

func nextEnabledLauncherItem(items []launcherItem, selected, direction int) int {
	for offset := 1; offset <= len(items); offset++ {
		candidate := (selected + direction*offset + len(items)) % len(items)
		if items[candidate].enabled {
			return candidate
		}
	}
	return selected
}
