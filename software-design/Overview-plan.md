---
document: Jetway Software Design Index
project: Jetway
repository: https://github.com/kakashianh/jetway
branch: main
status: living-design
version: 1.0
last-reviewed: 2026-11-23
language: en
source-of-truth: implementation on main
---

# Jetway — Software Design

## Project metadata

| Field | Value |
|---|---|
| Product | Jetway airline and GDS reservation messaging gateway |
| Primary language | Go 1.27 |
| Module | `github.com/adamf/jetway` |
| Persistence | PostgreSQL; in-memory backend for tests/demo |
| Wire protocols | Type B/AIRIMP, UN/EDIFACT/PADIS, plus aviation operational formats |
| License | MIT |
| Repository | [kakashianh/jetway](https://github.com/kakashianh/jetway) |
| Design baseline | `main` branch, reviewed 2026-11-23 |

## Design document map

| File | Scope |
|---|---|
| [01-system-context.md](01-system-context.md) | Purpose, actors, boundaries, capabilities and exclusions |
| [02-architecture.md](02-architecture.md) | Runtime topology, components, layering and architecture decisions |
| [03-domain-model.md](03-domain-model.md) | PNR, segments, events, locators, dates and business invariants |
| [04-message-lifecycle.md](04-message-lifecycle.md) | Ingress-to-egress pipeline, idempotency, retries and failure handling |
| [05-protocol-design.md](05-protocol-design.md) | Type B, AIRIMP, EDIFACT, PADIS and other protocol families |
| [06-data-and-persistence.md](06-data-and-persistence.md) | Store contracts, event sourcing, PostgreSQL, spool and retention |
| [07-integration-and-routing.md](07-integration-and-routing.md) | Carrier onboarding, transports, peer identity, routing and queues |
| [08-security-and-privacy.md](08-security-and-privacy.md) | Trust boundaries, authentication, passenger data and known gaps |
| [09-deployment-and-operations.md](09-deployment-and-operations.md) | Configuration, deployment topology, observability and runbooks |
| [10-quality-and-roadmap.md](10-quality-and-roadmap.md) | Testing strategy, performance baseline, constraints and open work |

## Reading order

Start with 01–02 for system understanding, then 03–06 for domain and data correctness, 07–09 for integration and operations, and 10 for verification and planned work.

## Design authority

These documents describe the implementation visible in the repository at the review date. They distinguish implemented behaviour from limitations and recommendations. When a document and code disagree, the code and executable tests on the target commit are authoritative; update the design document with the implementation change.

## Important constraints

- Jetway is a gateway, record store and collection of replaceable airline-domain services; it is not a passenger booking website or agent desktop.
- AIRIMP and several PADIS message compositions are extensible inferred profiles, not a claim of full paid-publication conformance.
- Production use with passenger personal data is blocked by missing encryption at rest and individual erasure workflow.
- MATIP sessions are stateful; distribute whole carrier links, not individual messages.
- Capture must be durable before acknowledging an inbound message.
