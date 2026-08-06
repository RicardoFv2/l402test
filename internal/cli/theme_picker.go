/*
Hallmark · terminal component: interactive palette picker · genre: modern-minimal · theme: editor-inspired
Audience: agent and SDK developers · use: choose a readable local CLI palette · tone: technical, dark, austere
States: default · navigate · active preview · confirm · cancel · line-input fallback
Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V5
*/
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

var (
	errThemeSelectionCanceled            = errors.New("theme selection canceled")
	errInteractiveThemePickerUnavailable = errors.New("interactive theme picker unavailable")
)

const (
	ansiAlternateScreen = "\x1b[?1049h"
	ansiMainScreen      = "\x1b[?1049l"
	ansiCursorHome      = "\x1b[H"
	ansiClearScreen     = "\x1b[2J"
	ansiHideCursor      = "\x1b[?25l"
	ansiShowCursor      = "\x1b[?25h"
)

type themePickerKey uint8

const (
	themePickerKeyNone themePickerKey = iota
	themePickerKeyUp
	themePickerKeyDown
	themePickerKeyEnter
	themePickerKeyCancel
)

func chooseTerminalTheme(input io.Reader, output io.Writer, color bool) (string, error) {
	if terminalInput, terminalOutput, ok := interactiveThemePickerFiles(input, output); ok {
		selected, err := chooseTerminalThemeWithArrows(terminalInput, terminalOutput, color)
		if err == nil || errors.Is(err, errThemeSelectionCanceled) {
			return selected, err
		}
		if !errors.Is(err, errInteractiveThemePickerUnavailable) {
			return "", err
		}
	}
	return chooseTerminalThemeByLine(input, output, color)
}

func interactiveThemePickerFiles(input io.Reader, output io.Writer) (*os.File, *os.File, bool) {
	terminalInput, inputOK := input.(*os.File)
	terminalOutput, outputOK := output.(*os.File)
	if !inputOK || !outputOK || !isInteractiveTerminal(terminalInput) || !isInteractiveTerminal(terminalOutput) {
		return nil, nil, false
	}
	if !term.IsTerminal(int(terminalInput.Fd())) || !term.IsTerminal(int(terminalOutput.Fd())) {
		return nil, nil, false
	}
	return terminalInput, terminalOutput, true
}

func chooseTerminalThemeWithArrows(input *os.File, output io.Writer, color bool) (string, error) {
	state, err := term.MakeRaw(int(input.Fd()))
	if err != nil {
		return "", fmt.Errorf("%w: %v", errInteractiveThemePickerUnavailable, err)
	}
	defer func() { _ = term.Restore(int(input.Fd()), state) }()

	if _, err := io.WriteString(output, ansiAlternateScreen+ansiHideCursor); err != nil {
		return "", err
	}
	defer func() { _, _ = io.WriteString(output, ansiShowCursor+ansiMainScreen) }()

	reader := bufio.NewReader(input)
	selected := 0
	for {
		if err := redrawThemePicker(output, selected, color); err != nil {
			return "", err
		}
		key, err := readThemePickerKey(reader)
		if err != nil {
			return "", err
		}
		next, confirm, cancel := advanceThemePickerSelection(selected, key)
		if confirm {
			return terminalThemeChoices[selected].name, nil
		}
		if cancel {
			return "", errThemeSelectionCanceled
		}
		selected = next
	}
}

func redrawThemePicker(output io.Writer, selected int, color bool) error {
	if _, err := io.WriteString(output, ansiCursorHome+ansiClearScreen); err != nil {
		return err
	}
	return writeLines(output, interactiveThemePickerLines(selected, color)...)
}

func interactiveThemePickerLines(selected int, color bool) []string {
	if selected < 0 || selected >= len(terminalThemeChoices) {
		selected = 0
	}
	choice := terminalThemeChoices[selected]
	view := presentation{color: color, theme: terminalThemes[choice.name]}
	lines := []string{
		view.heading("Choose a terminal theme"),
		view.note("Use ↑ / ↓ to preview · Enter to save · Esc to cancel"),
		"",
	}
	for index, option := range terminalThemeChoices {
		if !color {
			marker := " "
			if index == selected {
				marker = "›"
			}
			lines = append(lines, fmt.Sprintf("  %s %d. %-18s %s", marker, index+1, option.label, option.description))
			continue
		}
		if index == selected {
			lines = append(lines,
				"  "+view.accent("›")+" "+view.panel(fmt.Sprintf(" %d ", index+1))+" "+view.strong(option.label)+view.muted("  "+option.description),
			)
			continue
		}
		lines = append(lines, view.muted(fmt.Sprintf("    %d. %-18s %s", index+1, option.label, option.description)))
	}
	return append(lines,
		"",
		view.heading("Preview · "+choice.label),
		view.rule(),
		view.header("local L402 HTTP playground"),
		"",
		view.status("READY"),
		view.field("address", "http://127.0.0.1:53142"),
		view.field("scenario", "happy"),
		"",
		view.command("curl -i http://127.0.0.1:53142/v1/weather"),
	)
}

func advanceThemePickerSelection(selected int, key themePickerKey) (next int, confirm, cancel bool) {
	if len(terminalThemeChoices) == 0 {
		return 0, false, true
	}
	if selected < 0 || selected >= len(terminalThemeChoices) {
		selected = 0
	}
	switch key {
	case themePickerKeyUp:
		return (selected + len(terminalThemeChoices) - 1) % len(terminalThemeChoices), false, false
	case themePickerKeyDown:
		return (selected + 1) % len(terminalThemeChoices), false, false
	case themePickerKeyEnter:
		return selected, true, false
	case themePickerKeyCancel:
		return selected, false, true
	default:
		return selected, false, false
	}
}

func readThemePickerKey(reader *bufio.Reader) (themePickerKey, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return themePickerKeyNone, fmt.Errorf("read theme selection: %w", err)
	}
	switch first {
	case '\r', '\n':
		return themePickerKeyEnter, nil
	case 3:
		return themePickerKeyCancel, nil
	case 0, 0xe0:
		return readWindowsThemePickerKey(reader)
	case '\x1b':
		return readEscapeThemePickerKey(reader)
	default:
		return themePickerKeyNone, nil
	}
}

func readWindowsThemePickerKey(reader *bufio.Reader) (themePickerKey, error) {
	second, err := reader.ReadByte()
	if err != nil {
		return themePickerKeyNone, fmt.Errorf("read Windows theme selection: %w", err)
	}
	switch second {
	case 72:
		return themePickerKeyUp, nil
	case 80:
		return themePickerKeyDown, nil
	default:
		return themePickerKeyNone, nil
	}
}

func readEscapeThemePickerKey(reader *bufio.Reader) (themePickerKey, error) {
	if reader.Buffered() == 0 {
		return themePickerKeyCancel, nil
	}
	second, err := reader.ReadByte()
	if err != nil {
		return themePickerKeyNone, fmt.Errorf("read escape theme selection: %w", err)
	}
	if second != '[' && second != 'O' {
		return themePickerKeyNone, nil
	}
	third, err := reader.ReadByte()
	if err != nil {
		return themePickerKeyNone, fmt.Errorf("read arrow theme selection: %w", err)
	}
	switch third {
	case 'A':
		return themePickerKeyUp, nil
	case 'B':
		return themePickerKeyDown, nil
	default:
		return themePickerKeyNone, nil
	}
}

func chooseTerminalThemeByLine(input io.Reader, output io.Writer, color bool) (string, error) {
	if err := writeThemePicker(output, color); err != nil {
		return "", err
	}
	reader := bufio.NewReader(input)
	for {
		if _, err := io.WriteString(output, "\nTheme [1]: "); err != nil {
			return "", err
		}
		choice, err := readThemeChoice(reader)
		if err != nil {
			return "", err
		}
		if name, ok := parseThemeChoice(choice); ok {
			return name, nil
		}
		if _, err := io.WriteString(output, "Choose 1-4, or type a theme name.\n"); err != nil {
			return "", err
		}
	}
}

func writeThemePicker(output io.Writer, color bool) error {
	lines := []string{
		"",
		"Choose a terminal theme",
		"This is saved locally. Run l402test theme to change it.",
	}
	for index, choice := range terminalThemeChoices {
		theme := terminalThemes[choice.name]
		view := presentation{color: color, theme: theme}
		if !color {
			lines = append(lines, fmt.Sprintf("  %d. %s - %s", index+1, choice.label, choice.description))
			continue
		}
		lines = append(lines,
			fmt.Sprintf("  %d. %s %s %s%s", index+1, view.panel(" l402test "), view.accent("◆"), view.code(choice.label), view.muted("  "+choice.description)),
		)
	}
	return writeLines(output, lines...)
}

func readThemeChoice(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err == nil {
		return line, nil
	}
	if errors.Is(err, io.EOF) && line != "" {
		return line, nil
	}
	return "", fmt.Errorf("read theme selection: %w", err)
}

func parseThemeChoice(raw string) (string, bool) {
	choice := strings.ToLower(strings.TrimSpace(raw))
	if choice == "" {
		return defaultTerminalTheme, true
	}
	for index, theme := range terminalThemeChoices {
		if choice == strconv.Itoa(index+1) {
			return theme.name, true
		}
	}
	resolved, err := resolveTerminalTheme(choice)
	if err != nil {
		return "", false
	}
	return resolved.name, true
}
