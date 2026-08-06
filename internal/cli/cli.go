// Package cli adapts the l402test domain packages to Cobra commands.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/RicardoFv2/l402test/internal/config"
	"github.com/RicardoFv2/l402test/internal/fixture"
	"github.com/RicardoFv2/l402test/internal/server"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func NewRoot(build BuildInfo) *cobra.Command {
	return newRoot(build, &outputOptions{preferences: defaultThemePreferencesStore()})
}

func newRoot(build BuildInfo, options *outputOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "l402test",
		Short:         "A local playground for testing L402-protected HTTP endpoints",
		Long:          rootLong(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runRootLauncher(command, options)
		},
	}
	root.PersistentFlags().StringVar(&options.color, "color", "auto", "color output: auto, always, or never")
	root.PersistentFlags().StringVar(&options.theme, "theme", "", "terminal palette override: "+supportedTerminalThemes)
	root.AddCommand(newInitCommand(options), newValidateCommand(options), newServeCommand(options), newThemeCommand(options), newVersionCommand(build, options))
	return root
}

func runRootLauncher(command *cobra.Command, options *outputOptions) error {
	input := command.InOrStdin()
	output := command.OutOrStdout()
	terminalInput, _, interactive := interactiveThemePickerFiles(input, output)
	if !interactive {
		return command.Help()
	}

	view, err := selectLauncherPresentation(options, input, output)
	if err != nil {
		return err
	}
	for {
		state := launcherStateFor(defaultLauncherConfigPath, view.theme.name)
		action, err := runInteractiveLauncher(terminalInput, output, view, state)
		if err != nil {
			if errors.Is(err, errInteractiveLauncherUnavailable) {
				return command.Help()
			}
			return err
		}

		switch action {
		case launcherActionExit:
			return nil
		case launcherActionTheme:
			view, err = selectLauncherPresentation(options, input, output)
			if err != nil {
				return err
			}
			continue
		case launcherActionInit:
			if err := view.writeLauncherHandoff(output, "l402test init --output "+defaultLauncherConfigPath); err != nil {
				return err
			}
			if err := writeExampleConfig(defaultLauncherConfigPath); err != nil {
				return err
			}
			return view.writeInitSuccess(output, defaultLauncherConfigPath)
		case launcherActionValidate:
			if err := view.writeLauncherHandoff(output, "l402test validate --config "+defaultLauncherConfigPath); err != nil {
				return err
			}
			endpointCount, err := validateConfiguration(defaultLauncherConfigPath)
			if err != nil {
				return err
			}
			return view.writeValidationSuccess(output, defaultLauncherConfigPath, endpointCount)
		case launcherActionStart:
			if err := os.MkdirAll(filepath.Dir(defaultLauncherFixturePath), 0o700); err != nil {
				return fmt.Errorf("create launcher fixture directory: %w", err)
			}
			if err := view.writeLauncherHandoff(output, "l402test serve --config "+defaultLauncherConfigPath+" --scenario happy --fixture-out "+defaultLauncherFixturePath); err != nil {
				return err
			}
			return servePlayground(command.Context(), output, view, serveOptions{
				configPath:   defaultLauncherConfigPath,
				scenarioName: string(server.Happy),
				listen:       "127.0.0.1:0",
				fixturePath:  defaultLauncherFixturePath,
				format:       "text",
			})
		case launcherActionHelp:
			return command.Help()
		default:
			return nil
		}
	}
}

func selectLauncherPresentation(options *outputOptions, input io.Reader, output io.Writer) (presentation, error) {
	selected, canceled, err := options.selectAndSaveTheme(input, output)
	if err != nil {
		return presentation{}, err
	}
	if canceled {
		return options.presentation(input, output, false)
	}
	return newPresentation(output, options.color, selected)
}

func newInitCommand(options *outputOptions) *cobra.Command {
	var configOutput string
	command := &cobra.Command{
		Use:   "init",
		Short: "Write a minimal l402test YAML configuration",
		RunE: func(command *cobra.Command, _ []string) error {
			view, err := options.presentation(command.InOrStdin(), command.OutOrStdout(), true)
			if err != nil {
				return err
			}
			if err := writeExampleConfig(configOutput); err != nil {
				return err
			}
			return view.writeInitSuccess(command.OutOrStdout(), configOutput)
		},
	}
	command.Flags().StringVarP(&configOutput, "output", "o", "l402test.yaml", "path for the new configuration")
	return command
}

func writeExampleConfig(configOutput string) error {
	if err := os.MkdirAll(filepath.Dir(configOutput), 0o755); err != nil {
		return fmt.Errorf("create configuration directory: %w", err)
	}
	file, err := os.OpenFile(configOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create configuration: %w", err)
	}
	defer file.Close()
	if _, err := io.WriteString(file, exampleConfig); err != nil {
		return err
	}
	return nil
}

func newValidateCommand(options *outputOptions) *cobra.Command {
	var configPath string
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate an l402test YAML configuration",
		RunE: func(command *cobra.Command, _ []string) error {
			view, err := options.presentation(command.InOrStdin(), command.OutOrStdout(), true)
			if err != nil {
				return err
			}
			endpointCount, err := validateConfiguration(configPath)
			if err != nil {
				return err
			}
			return view.writeValidationSuccess(command.OutOrStdout(), configPath, endpointCount)
		},
	}
	command.Flags().StringVarP(&configPath, "config", "c", "l402test.yaml", "path to the YAML configuration")
	return command
}

func validateConfiguration(configPath string) (int, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return 0, err
	}
	return len(cfg.Endpoints), nil
}

type serveOptions struct {
	configPath   string
	scenarioName string
	listen       string
	fixturePath  string
	format       string
}

func newServeCommand(options *outputOptions) *cobra.Command {
	var configPath, scenarioName, listen, fixturePath, format string
	command := &cobra.Command{
		Use:   "serve",
		Short: "Start local L402-protected endpoints",
		RunE: func(command *cobra.Command, _ []string) error {
			var view presentation
			if format == "text" {
				var err error
				view, err = options.presentation(command.InOrStdin(), command.OutOrStdout(), true)
				if err != nil {
					return err
				}
			}
			return servePlayground(command.Context(), command.OutOrStdout(), view, serveOptions{
				configPath:   configPath,
				scenarioName: scenarioName,
				listen:       listen,
				fixturePath:  fixturePath,
				format:       format,
			})
		},
	}
	command.Flags().StringVarP(&configPath, "config", "c", "l402test.yaml", "path to the YAML configuration")
	command.Flags().StringVar(&scenarioName, "scenario", string(server.Happy), "scenario to emulate")
	command.Flags().StringVar(&listen, "listen", "127.0.0.1:0", "TCP address to listen on")
	command.Flags().StringVar(&fixturePath, "fixture-out", "", "required output path for test preimages")
	command.Flags().StringVar(&format, "format", "text", "readiness output format: text or json")
	_ = command.MarkFlagRequired("fixture-out")
	return command
}

func servePlayground(parent context.Context, output io.Writer, view presentation, options serveOptions) error {
	if options.format != "text" && options.format != "json" {
		return errors.New("format must be text or json")
	}
	cfg, err := config.Load(options.configPath)
	if err != nil {
		return err
	}
	scenario, err := server.ParseScenario(options.scenarioName)
	if err != nil {
		return err
	}
	playground, err := server.New(server.Options{Config: cfg, Scenario: scenario})
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", options.listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", options.listen, err)
	}
	defer listener.Close()
	baseURL := "http://" + listener.Addr().String()
	if err := fixture.Write(options.fixturePath, playground.Fixtures(baseURL)); err != nil {
		return err
	}
	if err := view.writeReady(output, options.format, baseURL, scenario, options.fixturePath, firstRequest(baseURL, cfg.Endpoints[0].Method, cfg.Endpoints[0].Path)); err != nil {
		return err
	}

	httpServer := &http.Server{
		Handler:           playground.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.Serve(listener) }()
	signalContext, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalContext.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdown)
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve playground: %w", err)
	}
}

func firstRequest(baseURL, method, path string) string {
	url := baseURL + path
	if method == http.MethodGet {
		return "curl -i " + url
	}
	return fmt.Sprintf("curl -i -X %s %s", method, url)
}

func newVersionCommand(build BuildInfo, options *outputOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		RunE: func(command *cobra.Command, _ []string) error {
			view, err := options.presentation(command.InOrStdin(), command.OutOrStdout(), true)
			if err != nil {
				return err
			}
			return view.writeVersion(command.OutOrStdout(), build)
		},
	}
}

func newThemeCommand(options *outputOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "theme",
		Short: "Choose and save a terminal color theme",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			selected, canceled, err := options.selectAndSaveTheme(command.InOrStdin(), command.OutOrStdout())
			if err != nil {
				return err
			}
			if canceled {
				return nil
			}
			view, err := newPresentation(command.OutOrStdout(), options.color, selected)
			if err != nil {
				return err
			}
			return view.writeThemeSaved(command.OutOrStdout())
		},
	}
}

const exampleConfig = `version: 1
endpoints:
  - name: premium-weather
    method: GET
    path: /v1/weather
    price_sats: 21
    response:
      status: 200
      content_type: application/json
      body: '{"forecast":"clear"}'
`
