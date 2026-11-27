# Design

## Context

See `proposal.md` - Why. Measured on this machine: uncached
`go test -count=1 ./...` takes about 13 s, `npm test` about 38 s, and
`tsc --noEmit` is a few seconds. `govulncheck ./...` and `npm audit` are
currently clean. The repo remote is `github.com/stepbeta/jobwatch`.

## Goals / Non-Goals

**Goals:**

- Every push and pull request fails on a compile error, vet finding, test
  failure, type error, known Go vulnerability, or committed secret.
- A local contributor can get the same signal before pushing with one
  command.
- Committed hooks are opt-in and fast enough to run on every commit.

**Non-Goals:**

- No coverage threshold, no `golangci-lint`, no Dependabot/Renovate, no
  release workflow. These can be added once the baseline is green.
- No change to how the binary is built or released.

## Decisions

### Two CI jobs plus a security job

- `go`: `setup-go` with `go-version-file: go.mod`, then `go build ./...`,
  `go vet ./...`, `go test -count=1 ./...`.
- `web`: `setup-node` 24 with npm cache keyed on `web/package-lock.json`,
  then `npm ci`, `npm test`, `npm run build`, and finally
  `go build -tags ui -o /tmp/jobwatch ./cmd/jobwatch` so the embedded-asset
  path is compiled too.
- `security`: `govulncheck-action` and `gitleaks-action` with
  `fetch-depth: 0`.

Rationale: separate jobs run in parallel and name the failing area. The
`-tags ui` build is the only check that exercises `go:embed` with a real
`dist/`, so it belongs in the job that produced it.

Alternative considered: one job doing everything sequentially. Simpler, but
slower and less legible. Alternative considered: pinning every action to a
commit SHA. Stronger supply-chain hygiene, but with no Dependabot the pins
go stale; the next change can add both together.

### `make test` is the local contract

`make test` runs `go vet`, `go test` and the frontend suite; `make test-go`
exists for a quick loop. CI uses the same commands so a green `make test`
is meaningful.

### Hooks are committed but opt-in

`.githooks/pre-commit` runs `gofmt -l`, `go vet`, `tsc --noEmit` when
`web/node_modules` exists, and `gitleaks protect --staged` when `gitleaks`
is installed. `.githooks/pre-push` runs the full Go suite and `npm test`.
`make hooks` sets `core.hooksPath=.githooks` and marks the scripts
executable.

Rationale: 38 s of frontend tests on every commit is enough friction to
make people bypass hooks. Splitting fast checks (commit) from the full
suite (push) keeps them useful. Optional tools are skipped with a message
rather than failing a commit on a machine without them; CI is the actual
gate.

Alternative considered: a Git hook manager dependency (husky/lefthook).
Rejected: no new dependency for three shell scripts.

## Risks / Trade-offs

- [Hooks are bypassable with `--no-verify`] → CI is authoritative and runs
  on every push and PR.
- [A contributor without `gitleaks` gets no local secret scan] → CI still
  scans; the hook prints which check was skipped.
- [Major-tag actions can change behavior] → accept for now and add
  Dependabot in a later change.
- [gitleaks flags a test fixture] → add an allowlist entry only when a real
  false positive appears; no speculative configuration.
