# Tasks

## 1. API allowlist input

- [x] 1.1 Replace `api.Server.BindAddr` with an explicit allowlist set and keep `localhost`, loopback IPs and `*.localhost` as built-in rules; verify `go test ./internal/api/...` passes with the updated constructor call sites (`serve.go`, API tests)
- [x] 1.2 Make `403 forbidden_host` name the rejected host and `--allow-host`; verify the new table test asserts the message contains both
- [x] 1.3 Extend `internal/api/policy_test.go` with cases for `jobwatch.localhost`, an allowed entry matching by host with any port and case-insensitively, and a foreign `Host` still rejected; verify those tests fail against the old policy and pass now
- [x] 1.4 Update the API docs: `docs/API.md` error table and host rules, and the `REST API` allowlist paragraph in `docs/ARCHITECTURE.md`; verify both describe the new rules and mention `--allow-host`

## 2. Serve flags and address expansion

- [x] 2.1 Add the repeatable `serve --allow-host` flag with validation (empty, URL or path aborts before the database opens) and verify with table tests for valid, empty, scheme and path values
- [x] 2.2 Expand the bind address into allowlist entries: a specific address as-is, a wildcard (`0.0.0.0`, `::`) into every unicast interface address plus `os.Hostname()`; verify unit tests with a fake interface list and hostname cover wildcard, specific and loopback binds
- [x] 2.3 Update `README.md` and `docs/USAGE.md` command tables and the `serve` paragraph: `--allow-host`, which address to open with `--unsafe-bind`, and the tunnel case; verify the documented commands match `serve --help`

## 3. Shell connection banner

- [x] 3.1 Add a typed `api.health()` call and a shell hook that checks on mount, `pageshow` and visibility restore, mapping `403 forbidden_host` to the host/flag hint and other failures to "server unreachable"; verify with App tests for refused host, unreachable server, recovery and the success case
- [x] 3.2 Render one banner above the routed page with cause-specific text and the address to open; verify an App test finds a single banner element and that no banner appears on success
- [x] 3.3 Describe the shell startup check in the `Web UI` section of `docs/ARCHITECTURE.md`; verify the section names the health endpoint and the banner states

## 4. Router startup sync

- [x] 4.1 Apply the current hash when the listener is attached and on `pageshow`, keeping `replaceRouteQuery` non-navigating; verify `router.test.ts` covers a hash changed before the effect, a `pageshow` restore and an unchanged route on query rewrite
- [x] 4.2 Note the startup/restore behavior in the `router.ts` row of `docs/ARCHITECTURE.md`; verify the row still matches the module's exports

## 5. Integration and close-out

- [ ] 5.1 Run `make test` and verify `go build ./...`, `go vet ./...` and the frontend suite pass
- [ ] 5.2 Verify end to end against a rebuilt binary: loopback still works, `--addr 0.0.0.0 --unsafe-bind` serves the UI at the machine's LAN IP and hostname, a mapped foreign host shows the banner and is accepted after `--allow-host`; record the observed results in the session summary
- [ ] 5.3 Write the `CHANGELOG.md` Unreleased entry (fixed allowlist derivation, `--allow-host`, router sync, health banner) and update `docs/MILESTONES.md` known gaps; verify `openspec validate local-access-resilience` still passes

## Workflow follow-up

- Replace the matching `docs/SPEC.md` sections with pointers to `local-api-security` and `web-ui-shell` when this change is archived.
- Archive the change after the project's review requirements are satisfied.
