# 10 — Quality Strategy, Constraints and Roadmap

## 1. Quality strategy

The repository's verification approach is built around the same node assembly and scenarios used in both integration tests and load generation. Avoid test-only wiring that differs from production composition.

## 2. Test layers

| Layer | Purpose | Expected coverage |
|---|---|---|
| Codec unit tests | Syntax and grammar correctness | Valid, invalid, boundary and diagnostic cases |
| Fuzz tests | Parser robustness and round-trip properties | Every wire/file parser; package-local fuzz harnesses |
| Store conformance | Backend semantic equivalence | Same assertions against memory and PostgreSQL |
| Gateway tests | Pipeline, PNR transitions, dedupe and response | Message lifecycle and concurrency |
| Integration scenarios | Real node wiring and transports | Simulated carriers over actual listeners |
| Load tests | Concurrency and throughput | Reuse integration scenarios with multiple workers |
| Security tests | Trust boundaries and data handling | TLS, peer identity, auth, limits, redaction and relay |

Useful repository commands:

```sh
make check
make test-pg
make fuzz
go test ./internal/scenario
go run ./cmd/jetwayload -list
go run ./cmd/jetwayload -workers 16 -for 30s
```

## 3. Critical regression properties

- Capture is durable before acknowledgement.
- Unknown/malformed payload is retained with diagnostics.
- Duplicate messages do not create duplicate business effects.
- PNR optimistic concurrency prevents lost updates.
- Exactly one concurrent writer wins for a given expected version.
- Date resolution is deterministic on replay and rejects invalid leap dates.
- Locator allocation is unique under a stable key.
- Queue placement is idempotent under concurrent sweepers.
- Address fan-out preserves original bytes and avoids sending back to ingress peer.
- Full indexed lookups do not silently scan only recent PNRs.
- Parser fuzzing does not panic, allocate without bounds or corrupt round trips.

A regression test should be observed failing against the old behaviour before it is accepted as evidence that the bug is covered.

## 4. Performance baseline

The repository's scaling document reports measurements from a 2026 laptop and local PostgreSQL 17. Representative single-threaded figures:

| Operation | Observed cost |
|---|---:|
| Type B parse | ~1.0 µs |
| EDIFACT parse | ~4.7 µs |
| Full inbound pipeline, memory store | ~14 µs |
| AppendMessage | ~73 µs |
| GetPNR | ~39 µs |
| UpdatePNR with events | ~221 µs |
| CreatePNR with events | ~167 µs |

An inbound message that changes a record was measured at roughly 450–500 µs of database time in the described setup. Numbers vary with hardware, PostgreSQL configuration, data distribution and concurrency; reproduce benchmarks before capacity decisions.

## 5. Known implementation limitations

- No encryption at rest or field-level encryption.
- No individual passenger-data erasure workflow.
- API authorization is a single bearer token, not per-user RBAC.
- Spool is unbounded and drains serially.
- Availability cache and channel sequence baselines are in-process state.
- MATIP channel sequence baseline is not preserved across process restart.
- AIRIMP profile is incomplete; divide advice is a notable missing message capability.
- Some PADIS compositions are inferred profiles because paid directories are unavailable.
- Sweeper due-date predicates can be pushed further into SQL.
- Insights aggregation is computed per request and is not appropriate at large scale.
- Dedicated btree columns for document number and carrier locator remain a possible optimization.
- Demo fleet is included in the shipped binary and should be separated for a hardened production build.

## 6. Prioritized roadmap

| Priority | Work item | Reason / acceptance direction |
|---|---|---|
| P0 | Encryption at rest and key management | Required before production passenger data |
| P0 | Individual erasure workflow across PNR, raw messages, events, spool and exports | Privacy and regulatory requirement |
| P0 | Bounded spool with backpressure and disk safety | Prevent disk exhaustion during prolonged outage |
| P1 | AIRIMP divide message/profile completion | Allows carriers to be advised of split PNRs |
| P1 | Shared availability state or deterministic peer pinning strategy | Avoid inconsistent free-sale decisions across processes |
| P1 | Persist channel sequence baselines / explicit failover semantics | Reduce false sequence-gap alarms |
| P1 | Parallel spool draining with ordering guarantees | Improve recovery throughput |
| P1 | Per-user identity and role-based authorization | Reduce blast radius of shared admin token |
| P2 | SQL-driven sweeper due-date selection | Avoid full-table work per sweep |
| P2 | Precomputed Insights counters | Avoid per-request full-store aggregation |
| P2 | Dedicated indexed document/partner locator columns | Reduce reliance on JSON GIN containment |
| P2 | Certificate hot reload | Reduce restart requirement for rotation |
| P2 | Separate demo fleet from production binary | Reduce production attack surface |

Priorities are design recommendations based on documented gaps, not commitments or scheduled releases.

## 7. Acceptance criteria for changes

A change is ready when:
1. its behaviour and source-of-truth package are documented;
2. relevant unit and integration tests pass;
3. parser changes include fuzz/fixture updates;
4. store changes pass both backend conformance suites;
5. concurrency and failure behaviour are tested;
6. metrics/logging avoid passenger PII;
7. security and operational impacts are stated;
8. benchmarks are supplied when a hot path or query plan changes.

## 8. Design maintenance

Update the relevant design file in the same pull request as a material architecture or data-contract change. Keep `Overview-plan.md` metadata-only and update its document map when files are added, renamed or removed.
