package cli

import (
	"fmt"
	"strings"
)

const (
	defaultTerminalTheme    = "tokyo-night"
	supportedTerminalThemes = "tokyo-night, dracula, catppuccin-mocha, nord"
)

type terminalColor struct {
	red   uint8
	green uint8
	blue  uint8
}

func (c terminalColor) foreground() string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.red, c.green, c.blue)
}

func (c terminalColor) background() string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.red, c.green, c.blue)
}

type terminalTheme struct {
	name       string
	surface    terminalColor
	foreground terminalColor
	accent     terminalColor
	success    terminalColor
	muted      terminalColor
	code       terminalColor
}

type terminalThemeChoice struct {
	name        string
	label       string
	description string
}

var terminalThemeChoices = []terminalThemeChoice{
	{name: "tokyo-night", label: "Tokyo Night", description: "midnight blue with sky accents"},
	{name: "dracula", label: "Dracula", description: "violet with cyan accents"},
	{name: "catppuccin-mocha", label: "Catppuccin Mocha", description: "soft mauve with teal accents"},
	{name: "nord", label: "Nord", description: "arctic blue with muted contrast"},
}

var terminalThemes = map[string]terminalTheme{
	"tokyo-night": {
		name:       "tokyo-night",
		surface:    terminalColor{26, 27, 38},
		foreground: terminalColor{192, 202, 245},
		accent:     terminalColor{122, 162, 247},
		success:    terminalColor{158, 206, 106},
		muted:      terminalColor{86, 95, 137},
		code:       terminalColor{125, 207, 255},
	},
	"dracula": {
		name:       "dracula",
		surface:    terminalColor{40, 42, 54},
		foreground: terminalColor{248, 248, 242},
		accent:     terminalColor{189, 147, 249},
		success:    terminalColor{80, 250, 123},
		muted:      terminalColor{98, 114, 164},
		code:       terminalColor{139, 233, 253},
	},
	"catppuccin-mocha": {
		name:       "catppuccin-mocha",
		surface:    terminalColor{30, 30, 46},
		foreground: terminalColor{205, 214, 244},
		accent:     terminalColor{203, 166, 247},
		success:    terminalColor{166, 227, 161},
		muted:      terminalColor{147, 153, 178},
		code:       terminalColor{137, 220, 235},
	},
	"nord": {
		name:       "nord",
		surface:    terminalColor{46, 52, 64},
		foreground: terminalColor{216, 222, 233},
		accent:     terminalColor{136, 192, 208},
		success:    terminalColor{163, 190, 140},
		muted:      terminalColor{76, 86, 106},
		code:       terminalColor{129, 161, 193},
	},
}

func resolveTerminalTheme(rawTheme string) (terminalTheme, error) {
	name := strings.ToLower(strings.TrimSpace(rawTheme))
	if name == "" {
		name = defaultTerminalTheme
	}
	if name == "catppuccin" {
		name = "catppuccin-mocha"
	}
	theme, ok := terminalThemes[name]
	if !ok {
		return terminalTheme{}, fmt.Errorf("theme must be one of: %s", supportedTerminalThemes)
	}
	return theme, nil
}
