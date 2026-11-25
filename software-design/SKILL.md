---
name: repository-software-design
description: Create evidence-based, detailed software architecture and design documentation for an existing software repository, split into logically organized Markdown files and committed to the requested documentation directory. Use when the user asks to analyze a codebase and produce or update software design documents.
---

# Repository Software Design Skill

## Purpose

Produce a coherent, implementation-aware software design package for an existing repository. Explain what the software actually does, how its components work together, what constraints exist, and which improvements are recommendations rather than implemented facts.

The default deliverable is a set of linked Markdown files committed to the repository path requested by the user. Follow the Jetway/MediMind-style approach: one concise metadata/index file plus focused, detailed design documents.

## Operating principles

1. **Evidence before design.** Inspect the repository before writing. Base statements about current behavior on source code, configuration, tests, migrations, workflows, and existing docs.
2. **Separate fact from proposal.** Clearly label content as:
   - **Implemented** — directly supported by current source/configuration/tests.
   - **Partially implemented** — present but incomplete, stubbed, or constrained.
   - **Proposed** — recommended future architecture or behavior, not confirmed in code.
   - **Unknown / needs verification** — repository evidence is insufficient.
3. **Never invent implementation details.** Do not claim a database, queue, protocol, API, deployment topology, security control, or runtime behavior exists unless repository evidence supports it.
4. **Trace evidence.** Reference relevant repository paths, symbols, configuration keys, migrations, and tests inline. Prefer precise paths and symbol names over vague claims.
5. **Preserve repository conventions.** Follow existing naming, language, architecture, and documentation style where practical.
6. **Design for maintainability.** Split documents by concern, avoid duplicated explanations, and link related documents.
7. **Be explicit about uncertainty.** If code is missing or ambiguous, state the assumption and the verification needed.
8. **Write in the user's requested language.** If unspecified, use the language of the request. Keep code identifiers and standard technical terms unchanged where useful.
9. **Do the repository work.** When the user asks to write files into GitHub, create/update those files in the specified repository and directory; do not merely return suggested content in chat.
10. **Do not overstate verification.** Report exactly which paths were read/written and which were subsequently fetched or otherwise verified.

## Workflow

### Phase 1 — Resolve the target

Identify:
- Repository URL and owner/name.
- Target branch (use the requested branch; otherwise repository default branch).
- Target documentation directory.
- Requested language and depth.
- Constraints on filenames, overview contents, or whether existing files must be preserved.

If one essential detail is missing and cannot be safely inferred, ask one concise question. Otherwise proceed with sensible defaults.

### Phase 2 — Repository reconnaissance

Inspect the repository in a deliberate order:

1. Root directory and project documentation (README, architecture docs, contribution guides).
2. Build and dependency manifests (go.mod, package.json, pyproject.toml, Cargo.toml, pom.xml, .csproj).
3. Main entry points and executable commands.
4. Source tree and package/module boundaries.
5. Domain models, core services, application/use-case layer.
6. API, CLI, event, protocol, and integration boundaries.
7. Persistence layer, schemas, migrations, repositories, caches, object storage.
8. Configuration, secrets handling, feature flags, environment variables.
9. Tests, fixtures, mocks, and test infrastructure.
10. Dockerfiles, compose files, Kubernetes/Helm, CI/CD, deployment and operational scripts.
11. Existing design documents and relevant ADR records if available.

Use repository search to locate symbols and references, then fetch full files for important implementation paths. Do not rely on README alone.

Build an evidence map before drafting:

| Concern | Evidence inspected | Confidence | Notes |
|---|---|---|---|
| Runtime / entry points | paths and symbols | High/Medium/Low | observed behavior |
| Architecture / modules | paths and imports | ... | ... |
| Domain model | paths and types | ... | ... |
| Data / persistence | paths and migrations | ... | ... |
| Interfaces / integrations | paths and handlers | ... | ... |
| Security | paths and controls | ... | ... |
| Deployment / operations | paths and workflows | ... | ... |
| Testing / quality | paths and test suites | ... | ... |

Do not include unsupported rows as facts in the final design.

### Phase 3 — Establish the system model

Before writing, determine:

- Product purpose and primary users/actors.
- Main use cases and business capabilities.
- Runtime components and their responsibilities.
- Important internal and external boundaries.
- Main synchronous and asynchronous flows.
- Domain entities, identifiers, lifecycle/state transitions, and invariants.
- Data ownership and persistence behavior.
- External dependencies and failure boundaries.
- Security/privacy-sensitive data and trust boundaries.
- Deployment units and operational dependencies.
- Known limitations, technical debt, and evidence gaps.

Create diagrams only when they clarify a real relationship. Use Mermaid in Markdown where supported. Label proposed components and flows explicitly.

### Phase 4 — Plan the document set

Use this as a default template and adapt it to the actual product:

    software-design/
    ├── Overview-plan.md
    ├── 01-system-context.md
    ├── 02-architecture.md
    ├── 03-domain-model.md
    ├── 04-core-workflows.md
    ├── 05-api-and-interface-design.md
    ├── 06-data-and-persistence.md
    ├── 07-integration-and-messaging.md
    ├── 08-security-and-privacy.md
    ├── 09-deployment-and-operations.md
    └── 10-quality-risks-and-roadmap.md

For protocol-heavy, event-driven, AI, healthcare, financial, or other specialized systems, split or rename documents to give the domain its own focused treatment. Do not force every project into the same file list.

#### Overview-plan.md — strict metadata/index only

This file must remain concise. Include only:
- Project/system name.
- Repository URL.
- Documentation version or revision.
- Last reviewed date.
- Source-of-truth note (repository branch/commit if known).
- A linked list/table of design documents with one-line descriptions.
- Optional status or scope metadata.

Do **not** put architecture explanations, executive summaries, design narratives, diagrams, or duplicated technical content in this file. Detailed explanations belong in dedicated documents.

#### Detailed document expectations

**System context**
- Purpose, scope, actors, capabilities, external systems, context diagram.
- In-scope and out-of-scope boundaries.

**Architecture**
- Architectural style as evidenced, module/component responsibilities, dependency direction, runtime topology, important design decisions.
- Distinguish current architecture from target/proposed architecture.

**Domain model**
- Entities/value objects/aggregates or equivalent structures.
- Relationships, identifiers, lifecycle/state machines, invariants, ownership.
- Map domain concepts to actual code types and storage structures.

**Core workflows**
- End-to-end flows with sequence/activity diagrams where useful.
- Entry points, validation, orchestration, side effects, persistence, events, retries, error paths, idempotency.
- Cover the most important user/business workflows.

**API and interface design**
- HTTP/gRPC/GraphQL/CLI/events/protocol contracts that actually exist.
- Routes, request/response schemas, authentication, validation, error semantics, versioning.
- Mark proposed contracts as proposed.

**Data and persistence**
- Databases, schemas, migrations, repositories, caches, files/object storage.
- Data lifecycle, consistency, transactions, retention, backup/restore, indexing.
- Never infer a database from a dependency alone; verify actual usage.

**Integration and messaging**
- External systems, adapters, protocols, message formats, routing, timeouts, retries, dead-letter behavior, correlation, idempotency.
- Identify which integrations are implemented, mocked, stubbed, or only planned.

**Security and privacy**
- Trust boundaries, authentication, authorization, secret management, encryption, validation, audit, sensitive data, retention, threat scenarios.
- Describe controls actually present; list gaps and recommended controls separately.
- Avoid claiming compliance or certification without explicit evidence.

**Deployment and operations**
- Build/runtime requirements, deployment artifacts, environments, configuration, health checks, logs, metrics, traces, scaling, recovery, migrations, runbooks.
- State what is verified from deployment files and what is a recommended production target.

**Quality, risks, and roadmap**
- Testing strategy and observed test coverage shape (do not invent coverage percentages).
- Reliability, performance, scalability, maintainability, accessibility or domain-specific quality attributes.
- Risks, assumptions, technical debt, prioritized recommendations and staged roadmap.

### Phase 5 — Draft consistently

For every detailed file:

- Start with a clear title and a short purpose/scope statement.
- Use consistent terminology and define domain-specific acronyms.
- Explain the current implementation first; put target-state proposals in a separate section.
- Include relevant repository evidence using inline paths, e.g. internal/server/router.go, and symbol names.
- Include diagrams only when accurate and useful.
- Explain normal flow and important failure/edge cases.
- Add cross-links to related design files using relative Markdown links.
- Avoid repeating full sections; summarize and link to the authoritative document.
- Record assumptions, open questions, and evidence gaps where applicable.
- Prefer precise, actionable statements over generic architecture boilerplate.

Recommended labels:
- [Implemented]
- [Partial]
- [Proposed]
- [Needs verification]

Use labels consistently, especially in architecture, security, integration, and deployment sections.

### Phase 6 — Quality review

Before committing, verify:

**Factual quality**
- Every claim about current behavior is traceable to repository evidence.
- Dependencies are not mistaken for active integrations.
- Tests/mocks/stubs are not presented as production implementations.
- Protocol support and standards conformance are not overstated.
- Security/compliance claims are evidence-based.
- Proposed designs are unmistakably marked.

**Document quality**
- Overview-plan.md contains metadata and links only.
- All links and filenames resolve.
- No major topic is duplicated inconsistently.
- Terms, component names, and lifecycle states are consistent.
- Diagrams match written descriptions and code.
- Scope, assumptions, limitations, and open questions are visible.
- The document set is detailed enough to guide implementation and review.

**Repository quality**
- Do not overwrite unrelated user files.
- If a target file already exists, fetch it first and update it deliberately; preserve useful material unless obsolete or contradictory.
- Write files sequentially when the GitHub API requires current blob SHAs.
- Use clear commit messages.
- Fetch created/updated files or the directory after writing to verify persisted content.
- If a write or verification fails, report it honestly and do not claim full completion.

### Phase 7 — Final response

Respond in the user's language. Include:
1. Confirmation of what was created/updated.
2. Link to the documentation directory.
3. Links to each important document (or a compact directory link if many).
4. A concise description of design coverage.
5. Important limitations/uncertainties discovered.
6. Explicit verification status: which files were fetched after writing and any files not verified.

Do not dump the entire documentation into chat when files are already committed. Offer one useful next step, such as deeper review, ADR extraction, implementation backlog, or sequence diagrams.

## Adaptation rules

- **Small repository:** keep the document set compact; do not manufacture architecture layers that do not exist.
- **Monolith:** describe actual modules and boundaries; do not label it microservices without evidence.
- **Microservices:** document service ownership, contracts, data boundaries, discovery, resilience, and deployment independently.
- **AI/agent system:** include model/provider boundaries, prompts, tools, orchestration, memory/state, evaluation, safety, observability, and cost controls only where relevant.
- **Healthcare or regulated domain:** explicitly document sensitive data flows, consent/authorization, audit, retention, human oversight, and applicable compliance questions; do not claim compliance without evidence.
- **Protocol/gateway system:** document framing, parsing, codecs, validation, routing, correlation, retries, session/state management, interoperability profiles, and conformance limits where present.
- **Event-driven system:** document event schemas, ordering, delivery semantics, replay, idempotency, outbox/inbox, and dead-letter handling where applicable.
- **User asks for a specific structure:** honor that structure unless it conflicts with factual accuracy or repository safety.

## Definition of done

The task is complete only when:
- Repository reconnaissance has covered relevant implementation and operations surfaces.
- A coherent, appropriately sized set of Markdown design files exists at the requested path.
- Overview-plan.md is metadata/index only when that constraint is requested or adopted as project convention.
- Current facts and future proposals are clearly separated.
- Internal links and diagrams have been reviewed.
- Files have been written to the requested branch and post-write verification has been attempted.
- The final response provides accurate links and an honest account of verification.
