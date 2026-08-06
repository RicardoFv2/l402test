// Command client demonstrates the minimum client-side L402 retry flow against
// l402test. It is intentionally stdlib-only so it can be copied into an SDK or
// agent integration test.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/RicardoFv2/l402test/internal/l402"
)

type fixtureFile struct {
	Endpoints []fixtureEndpoint `json:"endpoints"`
}

type fixtureEndpoint struct {
	Method    string   `json:"method"`
	Path      string   `json:"path"`
	Preimages []string `json:"preimages"`
}

func main() {
	baseURL := flag.String("base-url", "", "l402test base URL")
	fixturePath := flag.String("fixture", "", "fixture JSON written by l402test")
	method := flag.String("method", http.MethodGet, "HTTP method for the premium endpoint")
	path := flag.String("path", "/v1/weather", "premium endpoint path")
	flag.Parse()
	if *baseURL == "" || *fixturePath == "" {
		fmt.Fprintln(os.Stderr, "usage: client --base-url http://127.0.0.1:8080 --fixture fixture.json")
		os.Exit(2)
	}
	requestMethod := strings.ToUpper(strings.TrimSpace(*method))
	if requestMethod == "" {
		fatal(fmt.Errorf("method is required"))
	}

	fixtures, err := readFixture(*fixturePath)
	if err != nil {
		fatal(err)
	}
	preimage, err := findPreimage(fixtures, requestMethod, *path)
	if err != nil {
		fatal(err)
	}
	url := strings.TrimRight(*baseURL, "/") + *path
	challengeRequest, err := http.NewRequest(requestMethod, url, nil)
	if err != nil {
		fatal(err)
	}
	challenge, err := http.DefaultClient.Do(challengeRequest)
	if err != nil {
		fatal(err)
	}
	defer challenge.Body.Close()
	if challenge.StatusCode != http.StatusPaymentRequired {
		fatal(fmt.Errorf("expected 402, received %s", challenge.Status))
	}
	macaroon, _, err := l402.ParseWWWAuthenticate(challenge.Header.Get("WWW-Authenticate"))
	if err != nil {
		fatal(err)
	}

	request, err := http.NewRequest(requestMethod, url, nil)
	if err != nil {
		fatal(err)
	}
	request.Header.Set("Authorization", "L402 "+macaroon+":"+preimage)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		fatal(err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		fatal(err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		fatal(fmt.Errorf("premium request failed: %s: %s", response.Status, contents))
	}
	fmt.Println(string(contents))
}

func readFixture(filename string) (fixtureFile, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		return fixtureFile{}, fmt.Errorf("read fixture: %w", err)
	}
	var value fixtureFile
	if err := json.Unmarshal(contents, &value); err != nil {
		return fixtureFile{}, fmt.Errorf("decode fixture: %w", err)
	}
	return value, nil
}

func findPreimage(fixtures fixtureFile, method, path string) (string, error) {
	for _, endpoint := range fixtures.Endpoints {
		if endpoint.Method == method && endpoint.Path == path && len(endpoint.Preimages) > 0 {
			return endpoint.Preimages[0], nil
		}
	}
	return "", fmt.Errorf("no fixture preimage for %s %s", method, path)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "client:", err)
	os.Exit(1)
}
