# Product strategy

## Product in one sentence

`l402test` is a local HTTP L402 playground for testing an agent, SDK, or client
before it is connected to a real wallet or Lightning node.

## Positioning statement

For developers building an agent, SDK, middleware, or HTTP client that consumes
L402-protected APIs and needs to verify the `402 -> challenge -> authorization
retry` behavior, `l402test` is a local protocol test harness that serves
configurable premium endpoints, valid test BOLT11 invoices, test macaroons, and
fixture preimages. Unlike a full regtest Lightning environment or a hand-written
mock, it isolates the HTTP L402 contract without requiring a wallet, node,
funds, or outbound service.

## Ideal user and job to be done

### Primary user

An engineer building one of the following:

- An autonomous or assisted agent that calls paid HTTP APIs.
- An L402 SDK, HTTP transport, middleware, or gateway client.
- A client that uses a fake wallet in CI before it uses a real wallet in an
  integration environment.

### Job

> When my client reaches an L402-protected endpoint, help me prove that it
> understands the challenge and retry lifecycle without bringing up payment
> infrastructure that is irrelevant to the HTTP assertion.

The client must be able to make an unauthenticated request, parse
`WWW-Authenticate`, inspect an invoice and price, obtain a preimage through its
own test payment adapter, retry with `Authorization`, and respond correctly
when the credential is expired or refreshed. This flow follows the
[L402 protocol specification](https://docs.lightning.engineering/the-lightning-network/l402/protocol-specification)
and is also reflected in the [L402 agent specification](https://github.com/lightninglabs/L402/blob/master/agent-spec.md).

## Problem framing

Testing an L402 client often entangles two separate concerns:

1. Does the HTTP client implement the protocol lifecycle correctly?
2. Can a local wallet and Lightning environment issue, route, and settle a
   real payment?

The second is essential for full-stack payment integration, but it adds nodes,
channels, funds, Docker, and operational state to a test whose only assertion
may be an HTTP retry. `l402test` is intentionally narrow: it makes the first
concern cheap, local, and repeatable while leaving the second to a dedicated
Lightning environment.

## Product promise

**Test the L402 HTTP handshake locally before connecting a wallet.**

The promise has three parts:

- **Protocol-shaped:** Challenges use L402 HTTP headers, test macaroons, and
  structurally valid regtest BOLT11 invoices rather than a JSON-only mock.
- **Controlled:** YAML declares the endpoints and price signals; scenarios
  expose common client branches on demand.
- **Local-first:** No wallet, node, funds, persistence, telemetry, or outbound
  calls are necessary.

## Competitive and alternative landscape

| Alternative | Strong when | Why `l402test` is different |
| --- | --- | --- |
| Hand-written HTTP mocks | One isolated branch is enough | They can drift from the actual L402 header, macaroon, invoice, and retry shape |
| [Polar](https://lightningpolar.com/) | Node topology, channels, RPC, and local Lightning payments matter | `l402test` does not simulate the network; it isolates the HTTP client contract |
| [Nigiri](https://github.com/vulpemventures/nigiri) | A Docker-based Bitcoin regtest environment and optional Lightning nodes are needed | `l402test` avoids standing up the regtest stack when payment settlement is out of scope |
| A real wallet/provider sandbox | End-to-end production-adjacent behavior is required | `l402test` is safe for fast local and CI protocol tests, not a substitute for that validation |

The relationship is complementary, not competitive: use a full local Lightning
environment when the system under test includes node or payment behavior; use
`l402test` when the system under test is an L402 HTTP consumer.

## Strategic choices

### Narrow protocol scope

The product focuses on HTTP L402 rather than becoming a broad Lightning
sandbox. This keeps setup near zero and makes failures attributable to the
client's protocol logic.

### Self-service developer tool

The primary distribution is a Go binary or container. The first useful outcome
is a reproducible local or CI test, not an account, hosted control plane, or
sales process.

### Local security boundary

Test preimages are only written to an explicit fixture file. The server does
not log macaroons or preimages, and it binds to loopback by default. These
credentials are test material, not production payment credentials. The L402
specification notes that real bearer credentials require protected transport in
production.

### Explicit non-goals

The project does not attempt to replace LND, Core Lightning, Bitcoin Core,
wallet adapters, real invoice settlement, routing, gRPC, MCP, or a production
billing service. Clear non-goals prevent a convenient test harness from being
mistaken for a payment stack.

## Go-to-market hypothesis

This is an open-source, self-service developer tool. Its most credible path to
adoption is a developer who can:

1. Clone or install it.
2. Start a configured endpoint in minutes.
3. Point an existing client test at the JSON readiness address.
4. See a failing L402 branch become a repeatable CI test.

Useful proof points are qualitative and technical rather than invented market
metrics: issue reports from SDK authors, examples in client test suites,
successful use across the supplied scenarios, and feedback that the harness
removed an otherwise-required node or wallet from a test.

## Validation signals

The next product decisions should be driven by evidence such as:

- Which scenarios developers actually use or ask for.
- Whether agents/SDKs need helpers around fixture consumption, rather than a
  wallet implementation inside this project.
- Whether HTTP L402 coverage is sufficient before considering gRPC or MCP.
- Whether teams use the JSON readiness contract in CI without needing a hosted
  service.

Do not infer consumer demand from App Store rankings or fabricate market-size,
adoption, or performance claims. `l402test` is developer infrastructure, so
its relevant evidence comes from protocol users and integration workflows.

## Reference sources

- [L402 overview](https://docs.lightning.engineering/the-lightning-network/l402)
- [L402 HTTP/gRPC protocol specification](https://docs.lightning.engineering/the-lightning-network/l402/protocol-specification)
- [L402 agent specification](https://github.com/lightninglabs/L402/blob/master/agent-spec.md)
- [Aperture, an L402 proxy](https://docs.lightning.engineering/lightning-network-tools/aperture)
- [Polar](https://lightningpolar.com/)
- [Nigiri](https://github.com/vulpemventures/nigiri)
