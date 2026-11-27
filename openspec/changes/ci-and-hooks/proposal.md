# Proposal

## Why

The repository has no automated verification: `go build`, `go vet`, the Go
test suite, the TypeScript compiler and the 121 frontend tests only run when
someone remembers to run them. Dependencies were never checked for known
vulnerabilities and nothing stops a secret from being committed. A single
workflow file and an opt-in hook close that gap without adding a runtime
dependency.

## What Changes

- Add a GitHub Actions workflow that runs on push to `main` and on pull
  requests: Go build/vet/test, frontend `npm ci`, tests and production
  build, `govulncheck`, and `gitleaks`.
- Add a `make test` target that runs the Go checks and the frontend suite
  locally.
- Add committed, opt-in git hooks: a fast pre-commit check (gofmt, `go vet`,
  `tsc --noEmit`, staged-secret scan when `gitleaks` is installed) and a
  pre-push full test run.
- Document how to install the hooks and what CI enforces.
- No runtime behavior, dependency or schema changes.

## Capabilities

### New Capabilities

None. This is tooling: it changes neither user-visible behavior nor an
existing requirement, so the change sets `skip_specs: true`.

### Modified Capabilities

None.

## Impact

- `.github/workflows/ci.yml` (new), `.githooks/` (new, opt-in).
- `Makefile`: `test` and `hooks` targets.
- `README.md`, `docs/ARCHITECTURE.md` (verification section),
  `docs/MILESTONES.md` (F-03 done), `CHANGELOG.md`.
- `AGENTS.md`: repository layout gains `.github/` and `.githooks/`.
- CI needs no secret: `gitleaks-action` is free for personal repositories.
  `govulncheck` and `npm audit` are currently clean, so the first run is
  expected to pass.
