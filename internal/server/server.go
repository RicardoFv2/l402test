// Package server runs deterministic local L402-protected endpoints.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/RicardoFv2/l402test/internal/config"
	"github.com/RicardoFv2/l402test/internal/fixture"
	"github.com/RicardoFv2/l402test/internal/l402"
)

type Scenario string

const (
	Happy                     Scenario = "happy"
	ExpiredInvoice            Scenario = "expired-invoice"
	MalformedChallenge        Scenario = "malformed-challenge"
	CredentialRefresh         Scenario = "credential-refresh"
	Upstream5xx               Scenario = "upstream-5xx"
	defaultCredentialLifetime          = time.Hour
)

// ParseScenario validates a named behavior supplied by the CLI.
func ParseScenario(value string) (Scenario, error) {
	scenario := Scenario(value)
	switch scenario {
	case Happy, ExpiredInvoice, MalformedChallenge, CredentialRefresh, Upstream5xx:
		return scenario, nil
	default:
		return "", fmt.Errorf("unknown scenario %q", value)
	}
}

type Options struct {
	Config   config.Config
	Scenario Scenario
	Clock    func() time.Time
}

type Server struct {
	scenario  Scenario
	issuer    *l402.Issuer
	endpoints map[string]*endpointRuntime
	paths     map[string][]string
	ordered   []*endpointRuntime
}

type endpointRuntime struct {
	definition config.Endpoint
	challenge  l402.Challenge
	refresh    *l402.Challenge
	mu         sync.Mutex
	refreshed  bool
}

func New(options Options) (*Server, error) {
	if err := config.Validate(options.Config); err != nil {
		return nil, err
	}
	if options.Clock == nil {
		options.Clock = time.Now
	}
	if options.Scenario == "" {
		options.Scenario = Happy
	}
	if _, err := ParseScenario(string(options.Scenario)); err != nil {
		return nil, err
	}
	issuer, err := l402.NewIssuer(options.Clock)
	if err != nil {
		return nil, err
	}
	server := &Server{
		scenario:  options.Scenario,
		issuer:    issuer,
		endpoints: make(map[string]*endpointRuntime, len(options.Config.Endpoints)),
		paths:     make(map[string][]string, len(options.Config.Endpoints)),
	}
	for _, endpoint := range options.Config.Endpoints {
		expiresAt := options.Clock().Add(defaultCredentialLifetime)
		if options.Scenario == ExpiredInvoice {
			expiresAt = options.Clock().Add(-time.Minute)
		}
		challenge, err := issuer.Issue(resource(endpoint), endpoint.PriceSats, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("issue challenge for %s: %w", endpoint.Name, err)
		}
		runtime := &endpointRuntime{definition: endpoint, challenge: challenge}
		if options.Scenario == CredentialRefresh {
			refresh, err := issuer.Issue(resource(endpoint), endpoint.PriceSats, options.Clock().Add(defaultCredentialLifetime))
			if err != nil {
				return nil, fmt.Errorf("issue refresh challenge for %s: %w", endpoint.Name, err)
			}
			runtime.refresh = &refresh
		}
		server.endpoints[key(endpoint.Method, endpoint.Path)] = runtime
		server.paths[endpoint.Path] = append(server.paths[endpoint.Path], endpoint.Method)
		server.ordered = append(server.ordered, runtime)
	}
	return server, nil
}

// Handler exposes only the configured premium endpoints. It has no control
// plane, request logging, storage, wallet, or outbound network calls.
func (server *Server) Handler() http.Handler {
	return http.HandlerFunc(server.serveHTTP)
}

func (server *Server) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	runtime, found := server.endpoints[key(request.Method, request.URL.Path)]
	if !found {
		if methods, pathFound := server.paths[request.URL.Path]; pathFound {
			writer.Header().Set("Allow", strings.Join(methods, ", "))
			writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeError(writer, http.StatusNotFound, "not found")
		return
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")

	if server.scenario == MalformedChallenge {
		writer.Header().Set("WWW-Authenticate", `L402 macaroon="unterminated`)
		writeError(writer, http.StatusPaymentRequired, "malformed L402 challenge")
		return
	}

	challenge := runtime.currentChallenge()
	if err := server.issuer.VerifyAuthorization(request.Header.Get("Authorization"), challenge); err != nil {
		server.writeChallenge(writer, challenge)
		return
	}

	if server.scenario == CredentialRefresh && runtime.requireRefresh() {
		server.writeChallenge(writer, *runtime.refresh)
		return
	}
	if server.scenario == Upstream5xx {
		writeError(writer, upstreamStatus(runtime.definition.Response.Status), "simulated upstream failure")
		return
	}
	server.writePremiumResponse(writer, runtime.definition.Response)
}

func (server *Server) writeChallenge(writer http.ResponseWriter, challenge l402.Challenge) {
	writer.Header().Set("WWW-Authenticate", challenge.WWWAuthenticate())
	writeError(writer, http.StatusPaymentRequired, "L402 credential required")
}

func (server *Server) writePremiumResponse(writer http.ResponseWriter, response config.Response) {
	for name, value := range response.Headers {
		writer.Header().Set(name, value)
	}
	writer.Header().Set("Content-Type", response.ContentType)
	writer.WriteHeader(response.Status)
	_, _ = writer.Write([]byte(response.Body))
}

func (server *Server) Fixtures(baseURL string) fixture.File {
	items := make([]fixture.Endpoint, 0, len(server.ordered))
	for _, runtime := range server.ordered {
		preimages := []string{runtime.challenge.Preimage}
		if runtime.refresh != nil {
			preimages = append(preimages, runtime.refresh.Preimage)
		}
		items = append(items, fixture.Endpoint{
			Name:      runtime.definition.Name,
			Method:    runtime.definition.Method,
			Path:      runtime.definition.Path,
			PriceSats: runtime.definition.PriceSats,
			Preimages: preimages,
		})
	}
	return fixture.File{Version: 1, Scenario: string(server.scenario), BaseURL: baseURL, Endpoints: items}
}

func (runtime *endpointRuntime) currentChallenge() l402.Challenge {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.refreshed && runtime.refresh != nil {
		return *runtime.refresh
	}
	return runtime.challenge
}

func (runtime *endpointRuntime) requireRefresh() bool {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	if runtime.refreshed {
		return false
	}
	runtime.refreshed = true
	return true
}

func resource(endpoint config.Endpoint) string {
	return endpoint.Method + " " + endpoint.Path
}

func key(method, path string) string {
	return method + " " + path
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": message})
}

func upstreamStatus(configured int) int {
	if configured >= http.StatusInternalServerError && configured <= 599 {
		return configured
	}
	return http.StatusBadGateway
}
