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

- No coverage threshold, no `golangci-lint`, no Dependabot/Renovate.
- No code signing, notarization, Homebrew formula or package-manager
  publishing.
- No change to how the binary is built for local development.

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

### Releases are tag-triggered and built in one job

`.github/workflows/release.yml` runs on `push: tags: ['v*']` with
`permissions: contents: write`. It builds the frontend once, runs
`go test -count=1 ./...` as a gate, then loops over five
`GOOS/GOARCH` pairs in a single `ubuntu-latest` job:

| OS | architectures |
|---|---|
| linux | amd64, arm64 |
| darwin | amd64, arm64 |
| windows | amd64 |

`modernc.org/sqlite` is pure Go, so `CGO_ENABLED=0` cross-compiles all of
them from Linux; this was verified locally for all five pairs (~19-20 MB
each). Each build uses `-tags ui -trimpath` and
`-ldflags "-s -w -X main.version=<tag without v>"`.

A single job with a shell loop avoids repeating the 30 s frontend build in
a matrix and keeps the archive naming in one place. If the target list
grows, a matrix fed by an uploaded `dist` artifact is the next step.

### Raw binaries and archives are both attached

Each target is uploaded twice: as a raw, stably named binary
(`jobwatch-linux-amd64`, `jobwatch-darwin-arm64`,
`jobwatch-windows-amd64.exe`, ...) and as an archive with the README
(`jobwatch_<version>_<os>_<arch>.tar.gz`, `.zip` for Windows). A
`sha256sums.txt` covers both forms.

Rationale: the raw name makes
`https://github.com/stepbeta/jobwatch/releases/latest/download/jobwatch-linux-amd64`
stable across releases, so one `curl`/`wget` command works forever.
Archives preserve the executable bit, which HTTP loses, and carry the
README. Keeping versioned archive names avoids ambiguity when several
releases are downloaded side by side.

Publishing uses the preinstalled `gh release create "$GITHUB_REF_NAME"
... --generate-notes`; no additional action is added. A tag containing a
hyphen (for example `v0.3.0-rc.1`) is published with `--prerelease`, so a
test or preview tag never takes over the `releases/latest` download URL. A
re-run uploads with `gh release upload --clobber` rather than failing on the
existing release.

### Release caveats are documented, not solved

- macOS and Windows binaries are unsigned. `curl`/`wget` downloads do not
  set the macOS quarantine attribute, but browser downloads may require
  right-click → Open or `xattr -d com.apple.quarantine`. Fixing this needs
  paid signing identities and is out of scope.
- The repository has no `LICENSE`; publishing does not require one, but the
  README will not claim usage rights beyond what the repository grants.
- Assets are about 20 MB, well under GitHub's 100 MB per-file limit.


## Risks / Trade-offs

- [Hooks are bypassable with `--no-verify`] → CI is authoritative and runs
  on every push and PR.
- [A contributor without `gitleaks` gets no local secret scan] → CI still
  scans; the hook prints which check was skipped.
- [Major-tag actions can change behavior] → accept for now and add
  Dependabot in a later change.
- [gitleaks flags a test fixture] → add an allowlist entry only when a real
  false positive appears; no speculative configuration.
- [A tag is pushed on a broken commit] → the release job runs the Go suite
  before building and publishes only at the end; a failed step leaves no
  release.
- [An unsigned macOS/Windows binary is refused or warned about] →
  documented workaround (right-click Open or `xattr`); signing is a separate
  decision with a recurring cost.
