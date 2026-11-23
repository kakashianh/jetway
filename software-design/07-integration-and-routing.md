# 07 — Integration, Transport and Routing

## 1. Integration model

A carrier connection is configured as a peer plus a listener/transport and, when required, a protocol profile. Most onboarding work should be configuration. Code is added only when framing, message recognition or segment interpretation differs from shipped behaviour.

## 2. Ingress adapters

Ingress adapters accept a byte stream or request, establish trusted peer context, frame the payload and pass it to the gateway.

| Ingress | Typical use | Identity / operational notes |
|---|---|---|
| HTTPS | Request/response integrations and NDC | mTLS can identify a peer; ordinary HTTP load balancing |
| TCP / framed TCP | Persistent partner links | Frame boundaries and peer mapping are configuration-driven |
| MATIP | Stateful airline Type B session | Long-lived session; pin peer link to one process |
| File drop | Batch exchange | One consumer/lock per directory to avoid competing readers |

## 3. Peer identity

Supported identification sources include:
- client certificate common name mapped to a configured peer;
- source CIDR on a private network;
- dedicated listener bound to one peer;
- token-protected hello where configured.

An unmapped certificate must be rejected. Do not trust a name asserted by a message payload. CIDR identity is weaker than certificate identity and should be restricted to private circuits.

## 4. Framing and sessions

`transport.Framer` handles configured framing such as length-prefix or sentinel. Frame sizes must be bounded. MATIP implements RFC 2351 session open, open confirm, session close and Type B data packets. Session state and channel sequence tracking make the link stateful.

## 5. Egress and delivery

Egress adapters send via HTTP post, outbound dial, reply-in-session or file drop. Outbound delivery uses retry/backoff and restart recovery. A failed send remains pending/retryable; it must not be recorded as delivered.

Partner-specific transport requirements (TLS, idle timeout, connection limits, rate limits and framing) belong in configuration and the partner's interface control document.

## 6. Routing

There are two distinct routing keys:

- **Peer route:** return a response to the peer that sent the request.
- **Address route:** deliver a message to every addressee encoded in the Type B address block.

The gateway builds an address table from each peer's `TTYAddress` and `Addresses`. Fan-out sends bytes unchanged and records the outcome for each addressee. Possible outcomes include delivered, terminates here, or no configured link.

Relay mode forwards messages addressed to other peers. It is disabled by default. A relay must skip the ingress link to prevent loops. Enabling relay without strict peer access controls creates an open-relay risk.

## 7. Carrier onboarding checklist

1. Obtain partner interface control document and agree message scope.
2. Choose transport and framing; confirm max frame and timeout.
3. Define peer identity using certificate mapping where possible.
4. Configure TLS/client CA and per-peer credentials/tokens.
5. Set peer address aliases and outbound destination.
6. Select Type B/AIRIMP or EDIFACT/PADIS profile.
7. Add profile extension only for proven dialect differences.
8. Configure retry, rate limits, connection limits and idle timeout.
9. Test inbound, response, duplicate, reconnect, malformed input and fan-out.
10. Validate with partner test traffic before production activation.

## 8. Queue integration

The internal queue is the durable worklist. The optional `queue.Publisher` notifies an external work system after placement. Persist first, publish second. If publish fails, log the error and retain the work item for later readers/reconciliation.

Queue items are idempotent by queue, PNR, reason code and segment. The gateway creates work from partner-driven transitions; the sweeper creates work for elapsed deadlines and missing responses.

## 9. External domain integrations

- Inventory is accessed through `gateway.Responder`; the default implementation is a simulator.
- Airport ground/DCS integration is a replaceable seam; the DCS package provides domain behaviour and message formats.
- External carriers may supply authoritative schedule, ticket, baggage, crew or settlement systems; Jetway modules do not automatically replace those systems.
- NDC is exposed over HTTP for order create/retrieve/cancel and order view operations.

## 10. Multi-node routing constraints

MATIP and framed TCP links must be assigned to a stable process or use an L4 affinity strategy that preserves whole sessions and channel ownership. HTTP requests can be distributed normally when their state is in shared persistence. File-drop paths require a single consumer or explicit locking.

The current availability cache and channel sequence baselines are in-process state. Multiple gateway processes therefore require deliberate peer pinning or a future shared-state design; do not assume that sharing PostgreSQL alone makes every in-memory decision consistent.
