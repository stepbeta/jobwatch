# Proposal

## Why

The local-API hardening from `scan-and-api-hardening` makes the API
unreachable through any `Host` that is not loopback or the literal `--addr`
string. `serve --addr 0.0.0.0 --unsafe-bind` therefore accepts `Host:
0.0.0.0` (which no browser sends) and rejects the machine's own LAN address,
so the flag its operator was told to use produces a UI whose every request
fails with `403 forbidden_host`. The SPA still loads, so the failure looks
like a broken interface instead of a refused request. A second, narrower
defect hides startup races: the app subscribes to `hashchange` after the
first paint, so a hash change that lands in that window is dropped and the
address bar moves while the view stays put.

## What Changes

- Derive the API host allowlist from what the server actually serves instead
  of the literal `--addr` value: loopback addresses, `*.localhost`, the bound
  address, every local interface address and the machine hostname when the
  bind is a wildcard, plus explicit exclusions for nothing.
- Add a repeatable `serve --allow-host <host>` flag for reverse proxies and
  tunnels, and make the `403 forbidden_host` message name the host that was
  rejected and the flag that would allow it.
- Make `hashchange` handling authoritative from the first frame: sync the
  route when the listener is attached and on `pageshow` (back/forward cache
  restore), not only on later `hashchange` events.
- Probe `GET /api/health` from the shell on mount and when the tab becomes
  visible, and show one top-level banner when the API is unreachable or
  refuses the host, with the address to open and the flag to pass. Per-panel
  errors keep working; the banner explains the shared cause.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `local-api-security`: the host/Origin allowlist is derived from the served
  addresses and explicit `--allow-host` entries, and a wildcard bind accepts
  the machine's own interface addresses and hostname.
- `web-ui-shell`: hash navigation is applied even when the change lands
  before the listener attaches or while the page is restored from cache, and
  a refused or unreachable API is reported by one shell-level banner.

## Impact

- `internal/api/policy.go`, `internal/api/policy_test.go`: allowlist input,
  `.localhost` handling, rejection message.
- `cmd/jobwatch/serve.go`, `cmd/jobwatch/serve_test.go`: `--allow-host`
  parsing and validation, allowlist construction from the bind address and
  interfaces.
- `web/src/router.ts`, `web/src/router.test.ts`, `web/src/App.tsx`,
  `web/src/App.test.tsx`: route sync on subscribe/`pageshow`, health probe and
  banner.
- `web/src/api.ts`: typed health call reused by the shell.
- Documentation: `docs/API.md` (403 body/message), `docs/ARCHITECTURE.md`
  (allowlist derivation, shell startup), `README.md` and `docs/USAGE.md`
  (`--allow-host`, which address to open), `CHANGELOG.md`,
  `docs/MILESTONES.md`, and the `docs/SPEC.md` pointers when archived.
- No schema change, no new dependency, no change to the default bind or to
  the JSON/Origin rules.
