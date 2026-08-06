package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RicardoFv2/l402test/internal/config"
	"github.com/RicardoFv2/l402test/internal/l402"
)

func TestHappyPathAndMethodHandling(t *testing.T) {
	t.Parallel()
	playground := newPlayground(t, Happy)
	httpServer := httptest.NewServer(playground.Handler())
	defer httpServer.Close()

	response, err := http.Get(httpServer.URL + "/v1/weather")
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("first status = %d", response.StatusCode)
	}
	macaroon := macaroonFromHeader(t, response.Header.Get("WWW-Authenticate"))
	_ = response.Body.Close()

	fixtures := playground.Fixtures(httpServer.URL)
	request, err := http.NewRequest(http.MethodGet, httpServer.URL+"/v1/weather", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "L402 "+macaroon+":"+fixtures.Endpoints[0].Preimages[0])
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authorized status = %d", response.StatusCode)
	}

	wrongMethod, err := http.Post(httpServer.URL+"/v1/weather", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer wrongMethod.Body.Close()
	if wrongMethod.StatusCode != http.StatusMethodNotAllowed || wrongMethod.Header.Get("Allow") != http.MethodGet {
		t.Fatalf("wrong method response = %d, Allow = %q", wrongMethod.StatusCode, wrongMethod.Header.Get("Allow"))
	}
}

func TestDefaultCredentialLifetime(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	cfg := config.Config{Version: 1, Endpoints: []config.Endpoint{{
		Name: "weather", Method: http.MethodGet, Path: "/v1/weather", PriceSats: 21,
		Response: config.Response{Status: http.StatusOK, ContentType: "application/json", Body: `{}`},
	}}}
	playground, err := New(Options{Config: cfg, Scenario: Happy, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if got := playground.ordered[0].challenge.ExpiresAt.Sub(now); got != defaultCredentialLifetime {
		t.Fatalf("credential lifetime = %s, want %s", got, defaultCredentialLifetime)
	}
}

func TestScenarios(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name             string
		scenario         Scenario
		initialHeaderBad bool
		finalStatus      int
		refresh          bool
	}{
		{name: "expired invoice", scenario: ExpiredInvoice, finalStatus: http.StatusPaymentRequired},
		{name: "malformed challenge", scenario: MalformedChallenge, initialHeaderBad: true},
		{name: "credential refresh", scenario: CredentialRefresh, finalStatus: http.StatusOK, refresh: true},
		{name: "upstream error", scenario: Upstream5xx, finalStatus: http.StatusBadGateway},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			playground := newPlayground(t, testCase.scenario)
			httpServer := httptest.NewServer(playground.Handler())
			defer httpServer.Close()
			response, err := http.Get(httpServer.URL + "/v1/weather")
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusPaymentRequired {
				t.Fatalf("initial status = %d", response.StatusCode)
			}
			if testCase.initialHeaderBad {
				if !strings.Contains(response.Header.Get("WWW-Authenticate"), "unterminated") {
					t.Fatalf("challenge = %q", response.Header.Get("WWW-Authenticate"))
				}
				response.Body.Close()
				return
			}
			macaroon := macaroonFromHeader(t, response.Header.Get("WWW-Authenticate"))
			response.Body.Close()
			fixtures := playground.Fixtures(httpServer.URL)
			response = authorizedRequest(t, httpServer.URL, macaroon, fixtures.Endpoints[0].Preimages[0])
			if testCase.scenario == ExpiredInvoice {
				if response.StatusCode != http.StatusPaymentRequired {
					t.Fatalf("expired status = %d", response.StatusCode)
				}
				response.Body.Close()
				return
			}
			if testCase.refresh {
				if response.StatusCode != http.StatusPaymentRequired {
					t.Fatalf("refresh status = %d", response.StatusCode)
				}
				macaroon = macaroonFromHeader(t, response.Header.Get("WWW-Authenticate"))
				response.Body.Close()
				response = authorizedRequest(t, httpServer.URL, macaroon, fixtures.Endpoints[0].Preimages[1])
			}
			defer response.Body.Close()
			if response.StatusCode != testCase.finalStatus {
				t.Fatalf("final status = %d, want %d", response.StatusCode, testCase.finalStatus)
			}
		})
	}
}

func TestUpstream5xxUsesConfiguredServerError(t *testing.T) {
	t.Parallel()
	playground := newPlayground(t, Upstream5xx)
	playground.ordered[0].definition.Response.Status = http.StatusServiceUnavailable
	httpServer := httptest.NewServer(playground.Handler())
	defer httpServer.Close()

	response, err := http.Get(httpServer.URL + "/v1/weather")
	if err != nil {
		t.Fatal(err)
	}
	macaroon := macaroonFromHeader(t, response.Header.Get("WWW-Authenticate"))
	response.Body.Close()
	fixtures := playground.Fixtures(httpServer.URL)
	response = authorizedRequest(t, httpServer.URL, macaroon, fixtures.Endpoints[0].Preimages[0])
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("configured upstream status = %d, want %d", response.StatusCode, http.StatusServiceUnavailable)
	}
}

func newPlayground(t *testing.T, scenario Scenario) *Server {
	t.Helper()
	cfg := config.Config{Version: 1, Endpoints: []config.Endpoint{{
		Name: "weather", Method: http.MethodGet, Path: "/v1/weather", PriceSats: 21,
		Response: config.Response{Status: http.StatusOK, ContentType: "application/json", Body: `{"forecast":"clear"}`},
	}}}
	playground, err := New(Options{Config: cfg, Scenario: scenario, Clock: func() time.Time {
		return time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return playground
}

func authorizedRequest(t *testing.T, baseURL, macaroon, preimage string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, baseURL+"/v1/weather", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "L402 "+macaroon+":"+preimage)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func macaroonFromHeader(t *testing.T, header string) string {
	t.Helper()
	macaroon, _, err := l402.ParseWWWAuthenticate(header)
	if err != nil {
		t.Fatalf("could not read macaroon from challenge: %v", err)
	}
	return macaroon
}
