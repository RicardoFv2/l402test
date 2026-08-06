/*
Hallmark · terminal component: palette renderer · genre: modern-minimal · theme: editor-inspired
Audience: agent and SDK developers · use: inspect local L402 test state · tone: technical, dark, austere
Hallmark · pre-emit critique: P5 H5 E5 S5 R5 V5
*/
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/RicardoFv2/l402test/internal/server"
)

const (
	terminalRuleWidth = 58

	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
)

type colorMode string

const (
	colorAuto   colorMode = "auto"
	colorAlways colorMode = "always"
	colorNever  colorMode = "never"
)

type presentation struct {
	color bool
	theme terminalTheme
}

type outputOptions struct {
	color       string
	theme       string
	preferences themePreferencesStore
}

func (o *outputOptions) presentation(input io.Reader, output io.Writer, allowThemePicker bool) (presentation, error) {
	themeName, err := o.resolveTheme(input, output, allowThemePicker)
	if err != nil {
		return presentation{}, err
	}
	return newPresentation(output, o.color, themeName)
}

func newPresentation(writer io.Writer, rawMode, rawTheme string) (presentation, error) {
	theme, err := resolveTerminalTheme(rawTheme)
	if err != nil {
		return presentation{}, err
	}
	color, err := resolveColorMode(writer, rawMode)
	if err != nil {
		return presentation{}, err
	}
	return presentation{color: color, theme: theme}, nil
}

func resolveColorMode(writer io.Writer, rawMode string) (bool, error) {
	mode := colorMode(strings.ToLower(strings.TrimSpace(rawMode)))
	switch mode {
	case colorAlways:
		return true, nil
	case colorNever:
		return false, nil
	case colorAuto:
		if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
			return false, nil
		}
		return isInteractiveTerminal(writer), nil
	default:
		return false, fmt.Errorf("color must be auto, always, or never")
	}
}

func isInteractiveTerminal(value any) bool {
	file, ok := value.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func (p presentation) header(subtitle string) string {
	if !p.color {
		return "l402test / " + subtitle
	}
	return p.panel(" l402test ") + " " + p.accent("◆") + p.muted("  "+subtitle)
}

func (p presentation) rule() string {
	rule := strings.Repeat("-", terminalRuleWidth)
	if p.color {
		rule = strings.Repeat("─", terminalRuleWidth)
	}
	return p.muted(rule)
}

func rootLong() string {
	return `Start local L402-protected HTTP endpoints without a wallet, node, or funds.
Use --format json when another program needs readiness output.`
}

func (p presentation) writeInitSuccess(writer io.Writer, configPath string) error {
	return writeLines(writer,
		p.header("configuration"),
		p.rule(),
		"",
		p.status("WRITTEN"),
		p.field("config", configPath),
		p.field("next", "l402test validate --config "+configPath),
	)
}

func (p presentation) writeValidationSuccess(writer io.Writer, configPath string, endpointCount int) error {
	return writeLines(writer,
		p.header("configuration check"),
		p.rule(),
		"",
		p.status("VALID"),
		p.field("config", configPath),
		p.field("endpoints", fmt.Sprintf("%d", endpointCount)),
	)
}

func (p presentation) writeReady(writer io.Writer, format, baseURL string, scenario server.Scenario, fixturePath, firstRequest string) error {
	if format == "json" {
		return json.NewEncoder(writer).Encode(map[string]string{
			"event":        "ready",
			"base_url":     baseURL,
			"scenario":     string(scenario),
			"fixture_file": fixturePath,
		})
	}
	return writeLines(writer,
		p.header("local L402 HTTP playground"),
		p.rule(),
		"",
		p.status("READY"),
		p.field("address", baseURL),
		p.field("scenario", string(scenario)),
		p.field("fixture", fixturePath),
		"",
		p.heading("Try the L402 challenge"),
		p.command(firstRequest),
		"",
		p.note("Press Ctrl+C to stop."),
	)
}

func (p presentation) writeVersion(writer io.Writer, build BuildInfo) error {
	return writeLines(writer,
		p.header("build information"),
		p.rule(),
		"",
		p.field("version", build.Version),
		p.field("commit", build.Commit),
		p.field("date", build.Date),
	)
}

func (p presentation) writeThemeSaved(writer io.Writer) error {
	return writeLines(writer,
		p.header("terminal theme"),
		p.rule(),
		"",
		p.status("SAVED"),
		p.field("theme", p.theme.name),
		p.note("Run l402test theme to choose another palette."),
	)
}

func (p presentation) status(value string) string {
	if !p.color {
		return "[" + value + "]"
	}
	return p.success("●") + " " + p.bold(value)
}

func (p presentation) field(label, value string) string {
	prefix := fmt.Sprintf("  %-11s ", strings.ToUpper(label))
	if !p.color {
		return prefix + value
	}
	return p.muted(prefix) + p.strong(value)
}

func (p presentation) heading(value string) string {
	if !p.color {
		return value
	}
	return p.decorate(ansiBold+p.theme.accent.foreground(), value)
}

func (p presentation) command(value string) string {
	if !p.color {
		return "$ " + value
	}
	return p.accent("$") + " " + p.code(value)
}

func (p presentation) note(value string) string {
	return p.muted(value)
}

func (p presentation) accent(value string) string {
	return p.decorate(p.theme.accent.foreground(), value)
}

func (p presentation) success(value string) string {
	return p.decorate(p.theme.success.foreground(), value)
}

func (p presentation) muted(value string) string {
	return p.decorate(p.theme.muted.foreground(), value)
}

func (p presentation) strong(value string) string {
	return p.decorate(ansiBold, value)
}

func (p presentation) text(value string) string {
	return p.decorate(p.theme.foreground.foreground(), value)
}

func (p presentation) bold(value string) string {
	return p.decorate(ansiBold, value)
}

func (p presentation) code(value string) string {
	return p.decorate(p.theme.code.foreground(), value)
}

func (p presentation) panel(value string) string {
	return p.decorate(ansiBold+p.theme.surface.background()+p.theme.foreground.foreground(), value)
}

func (p presentation) writeLauncherHandoff(writer io.Writer, command string) error {
	return writeLines(writer,
		p.header("launcher"),
		p.rule(),
		"",
		p.note("Selected command"),
		p.command(command),
		"",
	)
}

func (p presentation) decorate(style, value string) string {
	if !p.color || value == "" {
		return value
	}
	return style + value + ansiReset
}

func writeLines(writer io.Writer, lines ...string) error {
	_, err := io.WriteString(writer, strings.Join(lines, "\n")+"\n")
	return err
}
