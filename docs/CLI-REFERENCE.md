# CLI reference

l402test is a Cobra CLI for starting local L402-protected HTTP endpoints. It is
designed for manual exploration and for test automation. It does not need a
wallet, Lightning node, funds, database, or external service.

## Synopsis

~~~text
l402test [global flags] <command> [command flags]
~~~

Run l402test --help or l402test <command> --help for generated Cobra help.
When launched with no subcommand from a real interactive terminal, the CLI opens
its theme picker and keyboard launcher. When stdin/stdout are not an interactive
terminal, the no-argument invocation prints help instead.

The launcher is a convenience layer for people. Scripts, CI jobs, SDK tests,
and agents should use the explicit commands below.

## Global flags

| Flag | Values | Behavior |
| --- | --- | --- |
| --color | auto (default), always, never | Controls ANSI color in human-readable output. auto disables color for non-terminal output, NO_COLOR, or TERM=dumb. |
| --theme | tokyo-night, dracula, catppuccin-mocha, nord | Applies a terminal palette to the current invocation. catppuccin is accepted as an alias for catppuccin-mocha. |
| -h, --help | - | Prints Cobra help. |

--theme is an invocation override for normal commands. To save an explicit theme
preference, use l402test --theme nord theme. Saved preferences live in the
operating system user configuration directory in l402test/theme.json.

## Commands

### init

~~~text
l402test init [--output <path>]
~~~

Writes a minimal valid YAML configuration. The default path is l402test.yaml;
-o is short for --output.

The command creates parent directories when necessary, requests mode 0600 for
the new file, and refuses to overwrite an existing file.

~~~sh
l402test init
l402test init --output config/premium.yaml
~~~

### validate

~~~text
l402test validate [--config <path>]
~~~

Loads and validates a YAML configuration without opening a listener. The default
path is l402test.yaml; -c is short for --config.

~~~sh
l402test validate --config examples/premium.yaml
~~~

Validation requires one YAML document, version: 1, at least one endpoint, and
no unknown YAML fields. Each endpoint needs a unique name and unique method/path
pair; supported methods are GET, POST, PUT, PATCH, DELETE, and HEAD. Paths must
be absolute clean paths, prices must be positive, response statuses must be
between 200 and 599, and a content type is required.

### serve

~~~text
l402test serve --fixture-out <path> [flags]
~~~

Starts configured endpoints and blocks until Ctrl+C or SIGTERM shuts it down.
--fixture-out is required because clients need its test preimages to complete
the retry.

| Flag | Default | Behavior |
| --- | --- | --- |
| -c, --config <path> | l402test.yaml | YAML configuration to load. |
| --scenario <name> | happy | Behavior to emulate; see [Scenarios](#scenarios). |
| --listen <address> | 127.0.0.1:0 | TCP address. Port 0 selects an available local port. |
| --fixture-out <path> | required | Fixture JSON destination. Its parent directory must exist. |
| --format <text\|json> | text | Readiness output format. Any value other than text or json is rejected. |

Example with a known fixture directory:

~~~sh
mkdir .fixtures
l402test serve \
  --config examples/premium.yaml \
  --fixture-out .fixtures/fixture.json \
  --format json
~~~

With the default listener, the process prints one readiness record like this
before serving requests:

~~~json
{"event":"ready","base_url":"http://127.0.0.1:53142","scenario":"happy","fixture_file":".fixtures/fixture.json"}
~~~

base_url contains the assigned port, so automation should read it from this
record rather than assuming a port when --listen ends in :0.

### theme

~~~text
l402test theme
l402test --theme dracula theme
~~~

Opens an interactive selector when a terminal is available. Arrow keys preview
the palette, Enter saves it, and Esc cancels. If raw terminal input is not
available, the selector uses a numbered prompt instead.

l402test --theme <name> theme saves the named theme without opening the
selector. The palette changes only l402test output; it does not change the
terminal, shell, or editor theme globally.

### version

~~~text
l402test version
~~~

Prints the build version, commit, and build date. Development builds use the
defaults embedded in cmd/l402test/main.go; release builds set them through
GoReleaser linker flags.

### completion

~~~text
l402test completion <bash|fish|powershell|zsh>
~~~

Generates a completion script for the selected shell. Use the generated
subcommand help for shell-specific installation instructions.

## Scenarios

| Name | What a client should expect |
| --- | --- |
| happy | Receive a valid challenge, then receive the configured premium response after a valid retry. |
| expired-invoice | Receive a structurally valid but expired invoice/credential; the retry remains a 402. |
| malformed-challenge | Receive a 402 with an invalid WWW-Authenticate value; exercise challenge parser errors. |
| credential-refresh | After the first valid retry, receive another 402; parse the new macaroon and retry with the second fixture preimage. |
| upstream-5xx | Complete authorization, then handle a simulated JSON upstream error. The status is a configured 5xx status or 502. |

## YAML schema

~~~yaml
version: 1
endpoints:
  - name: premium-weather
    method: GET
    path: /v1/weather
    price_sats: 21
    response:
      status: 200
      content_type: application/json
      headers:
        X-Plan: premium
      body: '{"forecast":"clear"}'
~~~

body is a literal string: l402test writes it unchanged. price_sats becomes the
amount in the test BOLT11 invoice; it does not cause a real debit. For a
two-endpoint example, see examples/premium.yaml.

## HTTP contract and retry

An unauthenticated request to a configured endpoint receives HTTP 402:

~~~text
WWW-Authenticate: L402 macaroon="<base64>", invoice="<bolt11>"
~~~

The fixture contains the matching hexadecimal preimage. A test wallet or client
should parse the challenge, associate its invoice with the fixture preimage,
then repeat the same HTTP request with:

~~~text
Authorization: L402 <macaroon>:<hex-preimage>
~~~

The macaroon is scoped to the request method and path. A valid credential can
be reused until it expires, except where the selected scenario deliberately
changes the flow.

The repository includes a minimal client demonstration for the normal retry:

~~~sh
go run ./examples/client \
  --base-url http://127.0.0.1:53142 \
  --fixture .fixtures/fixture.json
~~~

Pass --method POST --path /v1/summary to demonstrate the second endpoint in
examples/premium.yaml. The example performs one challenge and one retry; it is
not a production wallet or a complete scenario runner.

## JSON automation

Use serve --format json whenever another program must discover the listener.
The readiness value is one JSON object on stdout with exactly these fields:

| Field | Meaning |
| --- | --- |
| event | Always ready for the readiness event. |
| base_url | Final HTTP origin, including the selected port. |
| scenario | The active scenario name. |
| fixture_file | The fixture path supplied by --fixture-out. |

JSON readiness contains no terminal escape sequences or preimages. The server
continues running after emitting the record, so a harness can wait for that
line, parse base_url, load fixture_file, run its test, and terminate the
process with an interrupt or termination signal.

For a shell-agnostic automation shape:

~~~text
create fixture directory
start l402test serve ... --format json in the background
wait for and parse the single readiness line
issue the unauthenticated request and inspect the 402 challenge
load the matching preimage from the fixture
retry with Authorization: L402 <macaroon>:<preimage>
stop the server
~~~

Use text mode only for a human-facing terminal. --format json avoids the
interactive launcher and terminal color presentation.

## Fixture file

--fixture-out writes JSON with the following shape:

~~~json
{
  "version": 1,
  "scenario": "happy",
  "base_url": "http://127.0.0.1:53142",
  "endpoints": [
    {
      "name": "premium-weather",
      "method": "GET",
      "path": "/v1/weather",
      "price_sats": 21,
      "preimages": ["hex-preimage"]
    }
  ]
}
~~~

Most endpoint entries have one preimage. In credential-refresh, each entry has
two preimages in challenge order. Treat fixture contents as local test data:
they are intentionally not printed by the CLI. The repository ignores the
standard `fixture.json`, names matching `*.fixture.json`, and the `.fixtures/`
directory.

## Manual workflow

~~~sh
l402test init
l402test validate --config l402test.yaml
mkdir .fixtures
l402test serve --config l402test.yaml --fixture-out .fixtures/fixture.json
~~~

The text readiness output prints a copyable first curl command. The initial
request should return 402; that is the expected starting state, not a server
error.

## Development checks

Run from the repository root:

~~~sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/l402test
~~~

The repository CI additionally checks formatting, runs staticcheck and
govulncheck, and smoke-tests the Docker image with l402test version.
