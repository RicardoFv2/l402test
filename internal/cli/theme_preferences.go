package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type themePreference struct {
	Theme string `json:"theme"`
}

type themePreferencesStore struct {
	path    string
	pathErr error
}

func defaultThemePreferencesStore() themePreferencesStore {
	directory, err := os.UserConfigDir()
	if err != nil {
		return themePreferencesStore{pathErr: fmt.Errorf("locate user configuration directory: %w", err)}
	}
	return themePreferencesStore{path: filepath.Join(directory, "l402test", "theme.json")}
}

func (s themePreferencesStore) Load() (string, bool, error) {
	if s.pathErr != nil {
		return "", false, s.pathErr
	}
	if s.path == "" {
		return "", false, errors.New("theme preference path is empty")
	}
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read theme preference: %w", err)
	}
	var preference themePreference
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&preference); err != nil {
		return "", false, fmt.Errorf("decode theme preference: %w", err)
	}
	name := strings.TrimSpace(preference.Theme)
	if name == "" {
		return "", false, nil
	}
	return name, true, nil
}

func (s themePreferencesStore) Save(themeName string) error {
	if s.pathErr != nil {
		return s.pathErr
	}
	if s.path == "" {
		return errors.New("theme preference path is empty")
	}
	theme, err := resolveTerminalTheme(themeName)
	if err != nil {
		return err
	}
	contents, err := json.Marshal(themePreference{Theme: theme.name})
	if err != nil {
		return fmt.Errorf("encode theme preference: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create theme preference directory: %w", err)
	}
	if err := os.WriteFile(s.path, append(contents, '\n'), 0o600); err != nil {
		return fmt.Errorf("write theme preference: %w", err)
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return fmt.Errorf("restrict theme preference permissions: %w", err)
	}
	return nil
}

func (o *outputOptions) resolveTheme(input io.Reader, output io.Writer, allowThemePicker bool) (string, error) {
	if explicit := strings.TrimSpace(o.theme); explicit != "" {
		theme, err := resolveTerminalTheme(explicit)
		if err != nil {
			return "", err
		}
		return theme.name, nil
	}
	stored, found, err := o.preferences.Load()
	if err != nil {
		return "", fmt.Errorf("load terminal theme preference: %w", err)
	}
	if found {
		theme, err := resolveTerminalTheme(stored)
		if err != nil {
			return "", fmt.Errorf("stored terminal theme is invalid: %w; run l402test theme to choose another", err)
		}
		return theme.name, nil
	}
	color, err := resolveColorMode(output, o.color)
	if err != nil {
		return "", err
	}
	if allowThemePicker && color && isInteractiveTerminal(input) && isInteractiveTerminal(output) {
		selected, canceled, err := o.promptAndSaveTheme(input, output, true)
		if err != nil {
			return "", err
		}
		if canceled {
			return defaultTerminalTheme, nil
		}
		return selected, nil
	}
	return defaultTerminalTheme, nil
}

func (o *outputOptions) selectAndSaveTheme(input io.Reader, output io.Writer) (string, bool, error) {
	if explicit := strings.TrimSpace(o.theme); explicit != "" {
		theme, err := resolveTerminalTheme(explicit)
		if err != nil {
			return "", false, err
		}
		if err := o.preferences.Save(theme.name); err != nil {
			return "", false, err
		}
		return theme.name, false, nil
	}
	if !isInteractiveTerminal(input) || !isInteractiveTerminal(output) {
		return "", false, errors.New("theme selection requires an interactive terminal; use --theme <name> to save a palette non-interactively")
	}
	color, err := resolveColorMode(output, o.color)
	if err != nil {
		return "", false, err
	}
	return o.promptAndSaveTheme(input, output, color)
}

func (o *outputOptions) promptAndSaveTheme(input io.Reader, output io.Writer, color bool) (string, bool, error) {
	selected, err := chooseTerminalTheme(input, output, color)
	if err != nil {
		if errors.Is(err, errThemeSelectionCanceled) {
			return defaultTerminalTheme, true, nil
		}
		return "", false, err
	}
	if err := o.preferences.Save(selected); err != nil {
		fmt.Fprintf(output, "\nTheme %q will be used for this run, but it could not be saved: %v\n\n", selected, err)
	}
	return selected, false, nil
}
