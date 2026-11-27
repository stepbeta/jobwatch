# Tasks

## 1. CI workflow

- [x] 1.1 Add `.github/workflows/ci.yml` with the `go`, `web`, and `security` jobs described in design.md, triggered on pull requests and pushes to `main`; verify the YAML parses (`python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml'))"`)
- [x] 1.2 Verify the `go` job commands pass locally exactly as written: `go build ./...`, `go vet ./...`, `go test -count=1 ./...`
- [x] 1.3 Verify the `web` job commands pass locally exactly as written: `npm ci`, `npm test`, `npm run build`, and `go build -tags ui` with the built `dist/`
- [x] 1.4 Document the CI checks and the `-tags ui` build in `README.md` and `CHANGELOG.md`; verify the documented commands match the workflow

## 2. Local test and hook targets

- [x] 2.1 Add `make test` and `make test-go` to the `Makefile`; verify both pass and that `make test` runs the frontend suite
- [x] 2.2 Add `.githooks/pre-commit` with gofmt, `go vet`, `tsc --noEmit` when `web/node_modules` exists, and `gitleaks protect --staged` when installed; verify it passes on the clean tree and fails when a file is intentionally unformatted
- [x] 2.3 Add `.githooks/pre-push` running the full Go suite and `npm test`; verify it fails on a deliberately broken test and passes on the clean tree
- [x] 2.4 Add `make hooks` to install `core.hooksPath` and executable bits; verify `git config core.hooksPath` reports `.githooks` after running it
- [x] 2.5 Document hook installation and optional tools in `README.md`, `docs/ARCHITECTURE.md` and `CHANGELOG.md`; verify a fresh clone can follow the instructions as written

## 3. Repository bookkeeping

- [x] 3.1 Add `.github/` and `.githooks/` to the repository layout in `AGENTS.md`; verify the tree matches the documented listing
- [x] 3.2 Mark F-03 done in `docs/MILESTONES.md`, close the related known gaps, and note that `golangci-lint`, coverage and Dependabot remain open; verify no fixed item is still listed as a gap
- [x] 3.3 In the untracked `REVIEW.md`, mark F-03 as fixed and F-16/F-20 as partially addressed by CI; verify the wording names what remains deferred

## 4. Integration

- [x] 4.1 Run `make test` and a clean `go build -tags ui -o /tmp/jobwatch ./cmd/jobwatch`; verify both pass with the hooks installed
- [x] 4.2 Verify the workflow file uses only actions available on public GitHub and no secret; confirm the expected first-run result is green by running `govulncheck ./...` and `npm audit` locally

## Workflow follow-up

- Create `chore/ci-and-hooks` off `main` and land it before `scan-and-api-hardening`, so the rest is gated by CI.
- Commit each group with a conventional-commit message; never stage `REVIEW.md`.
- Archive the change after review; no `docs/SPEC.md` pointer is needed because no capability changed.
