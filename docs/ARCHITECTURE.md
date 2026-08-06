# Architecture

l402test is a local HTTP playground for clients, SDKs, and agents that need to
exercise an L402-protected endpoint. It models the HTTP exchange only:
402 -> BOLT11 invoice -> L402 retry. It does not settle a Lightning payment.

## Scope and boundaries

- The server reads a versioned YAML file and exposes only the configured HTTP
  method/path pairs.
- Every process creates its own macaroon issuer, challenges, BOLT11 invoices,
  and test preimages.
- Invoices are signed regtest BOLT11 values with an ephemeral key. They are
  structurally decodable, but have no route to a Lightning node and cannot be
  paid.
- The process has no wallet, persistent database, telemetry, request log, or
  outbound network client.
- Default listening is 127.0.0.1:0; a non-loopback address is possible only
  when the caller explicitly passes --listen.

This makes the project a protocol test harness, not a payment processor or a
production authorization service.

## Package map

~~~
cmd/l402test
  -> internal/cli       Cobra commands, human output, JSON readiness, signals
  -> internal/config    YAML decoding and validation
  -> internal/server    HTTP routing, scenarios, response behavior
  -> internal/l402      BOLT11 construction, macaroon issue/verification
  -> internal/fixture   Atomic test-preimage fixture writer
~~~

| Package | Responsibility |
| --- | --- |
| cmd/l402test | Injects build version, commit, and date into the Cobra root command; prints command errors to stderr. |
| internal/cli | Implements init, validate, serve, theme, and version; owns the keyboard launcher and terminal presentation. |
| internal/config | Loads exactly one YAML document, rejects unknown fields, and validates endpoint definitions. |
| internal/l402 | Generates test-only BOLT11 invoices and macaroons; parses L402 challenge and authorization headers. |
| internal/server | Builds per-endpoint runtime state, returns HTTP errors/challenges, verifies credentials, and applies scenarios. |
| internal/fixture | Writes fixture JSON through a temporary file and rename, with mode 0600 requested for the temporary file. |

## Startup lifecycle

serve performs the following work in order:

1. Loads and validates the YAML configuration.
2. Validates the named scenario and creates an in-memory server.
3. For each endpoint, creates an L402 challenge. The normal credential lifetime
   is one hour.
4. Opens the requested TCP listener and derives base_url from its actual
   address; with port 0, this is the assigned ephemeral port.
5. Writes the fixture file before emitting readiness.
6. Prints readiness in text or JSON, then serves HTTP until SIGINT or
   SIGTERM. Shutdown has a 10-second deadline.

The HTTP server uses a 5-second read-header timeout, 15-second read and write
timeouts, and a 60-second idle timeout.

## L402 exchange

For a configured resource such as GET /v1/weather, the normal path is:

~~~
client                         l402test
  | GET /v1/weather               |
  |------------------------------>|
  | 402 + WWW-Authenticate:       |
  |   L402 macaroon="...",        |
  |   invoice="lnbcrt..."         |
  |<------------------------------|
  |                                |
  | Authorization: L402           |
  |   <macaroon>:<preimage>       |
  |------------------------------>|
  | configured premium response   |
  |<------------------------------|
~~~

Issuer.Issue generates a random 32-byte preimage and hashes it for the invoice
and macaroon. The macaroon is base64 encoded and contains first-party caveats
for:

- payment_hash: the SHA-256 hash of the expected preimage;
- resource: the exact METHOD path string; and
- expires_at: the Unix expiration time.

The server accepts Authorization: L402 <base64-macaroon>:<hex-preimage> only
when the preimage exactly matches the challenge and macaroon verification
succeeds with the process-local root key. Missing headers, malformed values,
unknown/invalid macaroons, incorrect preimages, resource mismatches, and
expired credentials receive a new 402 challenge in normal scenarios.

Credentials are bearer credentials and may be reused until expiration. The test
preimage is deliberately made available only through the local fixture, so a
fake wallet or test client can model invoice-to-preimage lookup.

## HTTP routing and responses

internal/server keys endpoints by exact METHOD path values.

- An unconfigured path returns 404 with {"error":"not found"}.
- A configured path with the wrong method returns 405, an Allow header, and
  {"error":"method not allowed"}.
- A configured endpoint sets Cache-Control: no-store and
  X-Content-Type-Options: nosniff before L402 processing.
- A successful request copies configured response headers, sets the configured
  content type and status, and writes the configured body unchanged.

## Scenario behavior

| Scenario | Behavior |
| --- | --- |
| happy | An unauthenticated request receives a valid challenge; an authorized retry receives the configured premium response. |
| expired-invoice | The generated BOLT11 invoice and credential are already expired. Authorization remains rejected with 402. |
| malformed-challenge | Requests receive 402 with an intentionally unterminated WWW-Authenticate header. |
| credential-refresh | The first valid original credential receives a replacement challenge. Retrying with the replacement macaroon and its second fixture preimage succeeds. |
| upstream-5xx | Authorization succeeds, then the server returns {"error":"simulated upstream failure"} with the configured 5xx response status, or 502 if the configured status is not in the 500-599 range. |

## YAML configuration contract

The loader requires one YAML document, a top-level version: 1, and at least one
endpoint. Unknown YAML fields are rejected.

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

| Field | Type | Validation |
| --- | --- | --- |
| version | integer | Must be 1. |
| endpoints | list | Must contain at least one item. |
| endpoints[].name | string | Required and unique. |
| endpoints[].method | string | One of GET, POST, PUT, PATCH, DELETE, or HEAD. |
| endpoints[].path | string | Required absolute, clean URL path; duplicate METHOD path pairs are rejected. |
| endpoints[].price_sats | integer | Must be greater than zero. It controls the BOLT11 amount and is not a payment charge. |
| endpoints[].response.status | integer | Must be in the inclusive range 200-599. |
| endpoints[].response.content_type | string | Required and nonblank. |
| endpoints[].response.headers | string map | Optional; names and values cannot contain CR or LF, and names cannot be blank. |
| endpoints[].response.body | string | Written unchanged after successful authorization; it may be empty. |

examples/premium.yaml contains a GET /v1/weather endpoint and a POST
/v1/summary endpoint.

## Fixture contract

The fixture is intentionally local test data, not a wallet secret. Its JSON
contains the selected scenario, the final base URL, and one item per configured
endpoint:

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

Most scenarios write one preimage per endpoint. credential-refresh writes two,
in original-challenge then replacement-challenge order. The fixture parent
directory must already exist for direct serve calls. The interactive launcher
creates its own .fixtures directory before using its default fixture path.

## Security model

- The server's macaroon root key is random and process-local; it is not saved
  or logged.
- Preimages are random test data. They are not printed in text readiness, JSON
  readiness, or server logs.
- The fixture writer creates a temporary file in the destination directory,
  requests restrictive 0600 permissions, and renames it into place.
- Protected endpoint responses are marked no-store to discourage caching of
  challenge-related responses.
- Exposing the listener with --listen 0.0.0.0:... is an explicit caller
  choice. It does not add authentication beyond the intentionally test-only
  L402 behavior.
- The project does not implement real invoice settlement, wallet adapters,
  gRPC, MCP, persistent credentials, or remote deployment controls.

Do not use generated invoices, macaroons, or fixture preimages as production
credentials.

## Testing and delivery

| Area | Verified by repository tests/workflows |
| --- | --- |
| Configuration | Table-driven validation covers version, missing endpoints, unsafe paths, non-positive prices, duplicate route, and duplicate name cases. |
| L402 primitives | BOLT11 decoding is checked with LND's zpay32 decoder; issuer verification, expiry rejection, challenge parsing, and fuzzing of both header parsers are covered. |
| HTTP server | httptest covers the happy path, method handling, credential lifetime, all scenarios, and configured upstream 5xx statuses. |
| Fixtures | Tests verify JSON content and replacement of an existing fixture. |
| CLI | Tests cover init/validate/version output, text and JSON readiness, color isolation, theme behavior, launcher state, fixture creation, and context-driven shutdown. |

The CI workflow runs unit tests on Ubuntu and Windows. Its Ubuntu quality job
also checks gofmt, runs go vet, go test -race, staticcheck, and govulncheck,
then builds a Docker image and invokes version in it.

Tag builds run GoReleaser for Darwin, Linux, and Windows on amd64 and arm64,
with checksums and archive SBOMs. The release workflow also builds and pushes
Linux amd64/arm64 images to ghcr.io/ricardofv2/l402test using the tag name and
latest.
