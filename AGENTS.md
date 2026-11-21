# AGENTS.md

Instructions for AI coding agents working on this repository. Read this
file in full before doing anything else, and re-read it if instructed
by the user.

## What this project is

**JobWatch** — a local, single-user web application that tracks job
offers for a person looking for a full-remote Platform Engineer / SRE
role in France. It scans configured sources (ATS boards such as
Greenhouse, Lever, Ashby; career sites including Welcome to the Jungle
company pages; RSS feeds), detects new / changed / disappeared offers,
classifies them with a deterministic rule-based engine, and tracks the
application lifecycle (timeline, notes, states) per offer and per
company.

## Hard constraints (never violate)

1. **Single static binary, zero external dependencies.** Go backend,
   SQLite (pure-Go driver: `modernc.org/sqlite`), frontend embedded
   with `go:embed`. No Docker, no external services, no config files
   required for basic operation.
2. **Localhost only.** The server binds to `127.0.0.1` by default. No
   authentication, no telemetry, no outbound requests other than
   source scanning.
3. **Respectful crawling.** Honor `robots.txt`, rate-limit per source
   (~1 req/2s default, per-source configurable), realistic User-Agent,
   never bypass anti-bot systems. A failing source is marked `error`,
   never crashes the scan loop.
4. **Deterministic classification.** No LLM in the scan loop. The
   classifier is keyword/score based and lives in `internal/classify`.
5. **No silent merging.** Duplicate companies or offers may only be
   merged after explicit user confirmation in the UI.
6. **Data safety.** All scan writes happen in transactions. Scans never
   delete offers (disappearing offers become `disappeared`, never
   removed). The only offer deletion is the absorbed row of an explicit,
   user-confirmed offer merge, and it happens only after every event,
   note, snapshot and provenance row has moved to the kept offer. Thus
   the "never delete offers" rule for automated behavior is absolute.
   Every state-changing migration must be reversible-safe for a single
   SQLite file (i.e. additive migrations preferred).

## Tech stack

- Go >= 1.27, standard library first. Router: net/http ServeMux.
- DB access via `database/sql` + `modernc.org/sqlite`. WAL mode.
- Migrations: plain SQL files applied in order, tracked in a
  `schema_migrations` table.
- Frontend: React + TypeScript (Vite), plain CSS or a minimal utility
  setup, no heavyweight framework ceremony. Built output is embedded
  into the binary via `go:embed` under `web/dist`.
- CLI: Cobra with subcommands (`serve`, `scan`, `add-source`,
  `add-sources`, `list`, `import`, `backup`, `reclassify`).

## Repository layout (keep it tidy)

```
cmd/jobwatch/        main + subcommands
internal/db/         sqlite setup, migrations runner, queries
internal/sources/    Fetcher interface + per-type adapters
internal/scan/       scan pipeline + manual import + classifier backfill (uses db + sources + classify)
internal/importmeta/ URL job-page metadata extraction for manual import
internal/classify/   scoring engine + keyword tables
internal/diff/       line diff for changed-offer snapshots
internal/api/        REST handlers
internal/model/      types shared across packages
web/                 React SPA source (src/) + Go embed (embed.go, noui.go, spa.go)
migrations/          numbered .sql files + embed.FS (migrations.go)
docs/                living documentation (see below)
openspec/            spec-driven change planning (see "OpenSpec" below)
```

## Documentation duty (MANDATORY)

The project's documentation must stay synchronized with the code.
Whenever an agent session changes behavior, data model, or API, the
same session MUST update:

1. `docs/ARCHITECTURE.md` — components, data flow, adapter interface,
   classification pipeline. Update when structure or behavior changes.
2. `docs/DATA-MODEL.md` — every table, column meaning, and state
   machine (offer_state, application_state, event types). Update on
   any schema or enum change.
3. `docs/API.md` — every REST endpoint: method, path, params, response
   shape. Update on any endpoint change.
4. `CHANGELOG.md` — one entry per session under a "Unreleased" heading:
   Added / Changed / Fixed bullets, written for a human reader.
5. `docs/MILESTONES.md` — mark completed milestones and their done
   criteria; add deviations from the original plan with a short reason,
   and keep the known-gaps table current.
6. `README.md` — build/run instructions and command reference. Update
   when commands, flags, or source configuration change.
7. `docs/SPEC.md` — only for permanent decisions that change the
   specification; record each revision in its "Spec revisions" section.
   Once a behavior has been captured as a capability under
   `openspec/specs/`, replace its section here with a one-line pointer
   instead of editing it (see "OpenSpec" below).
8. `AGENTS.md` — repository layout, when packages are added or moved.

Rules:
- Documentation changes belong in the SAME commit/code batch as the
  code that made them necessary. Never leave docs stale.
- If the agent discovers the docs contradict the code, it must fix the
  docs and flag the contradiction to the user in its summary.
- Keep docs terse and factual. No marketing language, no speculation.

### OpenSpec

The project adopts OpenSpec incrementally. Until every behavior in
`docs/SPEC.md` has a capability spec, both systems exist and ownership
moves one capability at a time.

- **One owner per capability.** Until a behavior is captured,
  `docs/SPEC.md` is authoritative. Once
  `openspec/specs/<capability>/spec.md` exists, that spec is
  authoritative and the `SPEC.md` section becomes a pointer. Never
  maintain both descriptions of the same behavior.
- **Capture on touch, not by bulk migration.** When a change modifies a
  subsystem, its delta becomes a capability spec when the change is
  archived, and the matching `SPEC.md` section is replaced by a pointer
  in the same batch. Do not backfill specs for behavior that is not
  changing.
- **OpenSpec owns obligations; docs keep the rest.** Requirements and
  scenarios (SHALL/MUST behavior) belong in `openspec/specs/`. Schemas,
  endpoints, components and procedures stay in `DATA-MODEL.md`,
  `API.md`, `ARCHITECTURE.md` and `README.md`.
- **Never hand-write a change.** Scaffold with
  `npx -y openspec new change "<name>"` (the CLI is not installed
  globally) and follow `openspec instructions` for each artifact. Work
  is planned in `proposal.md`, `design.md`, `specs/` and `tasks.md`,
  and implemented from `tasks.md`.
- **Planning is not implementation.** An explore session may create
  change artifacts after explicit confirmation, never code. Code changes
  start from an approved change, and archiving happens only when its
  tasks are done.

## Definition of done (every session)

- `go build ./...` passes; `go vet ./...` clean.
- `go test ./...` passes. New business logic (dedup, classification,
  adapters, migrations) ships with table-driven unit tests.
- Schema changes ship as a new numbered migration + updated
  DATA-MODEL.md.
- New endpoints documented in API.md.
- CHANGELOG.md entry written.
- MILESTONES.md status updated if the session closes or advances a
  milestone.
- When the session implements an OpenSpec change: tick its `tasks.md`
  items, keep the affected capability spec in step, and replace the
  matching `docs/SPEC.md` section with a pointer if the change is
  archived.
- The agent ends its session with a short summary: what was built,
  what tests were added, what docs were touched, known gaps.
- Unless the user says otherwise the session must conclude with a
  git commit, but not a push.

## Conventions

- Go: standard layout, `internal/` for non-exported packages, errors
  wrapped with `%w`, context propagated everywhere, no global mutable
  state except the DB handle wired at startup.
- SQL: snake_case, timestamps as RFC3339 TEXT (UTC).
- IDs of offers/companies come from SQLite autoincrement; fingerprints
  are SHA-256 hex over normalized strings.
- Normalization helpers (names, titles, keywords) live in
  `internal/model/normalize.go` and are reused everywhere; never inline
  ad-hoc normalization.
- Keyword tables in the classifier are constants with comments; tweaks
  there must be accompanied by updated classifier tests.
- Frontend: English UI text, no i18n layer. API errors returned as
  `{ "error": { "code": string, "message": string } }`.
- Git: always use the conventional commits standard for commit messages.

## Working style

- Work in the order given by the user's current milestone prompt. Do
  not jump ahead to later milestones.
- When a milestone prompt conflicts with this file, stop and ask the
  user; do not improvise.
- Prefer many small, verifiable commits over one giant batch.
- Do not add dependencies without stating why in the session summary.
- **Apply one task group at a time.** When implementing an OpenSpec
  change, complete a numbered group from `tasks.md`, tick it, and stop
  for review before starting the next group.
- **Back up the database before touching it.** Before applying a
  migration or any other change to the SQLite file, make a copy:
  `jobwatch backup jobwatch.db.<timestamp>.bkp` (consistent copy, refuses
  to overwrite, runs an integrity check). When no process has the
  database open, `cp jobwatch.db jobwatch.db.<timestamp>.bkp` is
  acceptable — never copy while `serve` holds it.
