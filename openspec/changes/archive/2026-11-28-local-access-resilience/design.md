# Design

## Context

See `proposal.md` - Why. Two existing mechanisms shape the approach:

- `internal/api/policy.go` already holds the allowlist decision, but the only
  input it receives is `api.Server.BindAddr`: the literal `--addr` string.
  `cmd/jobwatch/serve.go` owns flag parsing and the final `listenAddr`.
- `web/src/router.ts` reads `window.location.hash` in a `useState`
  initializer and subscribes in `useEffect`, which runs after the first paint.
  `App.tsx` renders per-page errors but no shell-level connection state.

The API has no authentication and must stay unauthenticated; the allowlist is
the only defence against DNS rebinding, so widening it is a security decision,
not a convenience toggle.

## Goals / Non-Goals

**Goals:**

- `--unsafe-bind` makes the UI usable at the addresses the operator actually
  opens, without opening the API to arbitrary `Host` values.
- A refused request is self-explanatory in both the HTTP message and the UI.
- Route changes coming from the address bar are never silently dropped.
- Keep the allowlist logic in `internal/api` testable without network access.

**Non-Goals:**

- Authentication, TLS, CORS response headers or a public deployment mode.
- SSRF filtering of scan/import URLs, retries and failure counters (still
  deferred from `scan-and-api-hardening`).
- Accepting wildcard host patterns such as `*.example.com`.
- Changing the default bind (`127.0.0.1`) or the JSON/Origin/body rules.

## Decisions

### The CLI computes the allowlist, the API enforces it

`serve` resolves the bind address, expands it, and passes a set of allowed
host names to `api.Server` (for example `AllowedHosts map[string]struct{}`),
replacing `BindAddr`. `internal/api` keeps the rules that need no discovery:
loopback IPs, `localhost`, and any name ending in `.localhost`. Comparison is
by host only, lowercased, port ignored - the same shape the policy already
uses.

Alternatives considered: letting the API enumerate interfaces on each request
(hidden I/O in a handler, untestable); keeping `BindAddr` and adding a second
input (two sources of truth for one decision).

### Wildcard binds expand to the machine's own addresses

When the bind address is unspecified (`0.0.0.0` or `::`), `serve` adds every
unicast address from `net.InterfaceAddrs()` and `os.Hostname()`. A specific
non-loopback bind adds just that address. No DNS lookup is performed, so an
FQDN such as `marie-curie.localdomain` is not added automatically; that is
what `--allow-host` is for.

Rationale: a browser sends the address the user typed, never `0.0.0.0`, so
comparing against the bind string can only ever fail. Accepting the machine's
own addresses does not weaken rebinding protection: an attacker's page still
sends its own domain in `Host`.

### `.localhost` names are always accepted

Every name ending in `.localhost` is reserved for loopback by RFC 6761 and is
resolved locally by browsers, so it cannot be rebound to a remote address.
Accepting it lets `jobwatch.localhost` and similar aliases work without a flag.

### `--allow-host` entries are exact, validated, and repeatable

The flag is a repeatable string flag. A value is trimmed and lowercased; an
optional port is stripped with `net.SplitHostPort`; the empty string, a value
containing `/` or `://`, and a value that is only a wildcard are rejected
before the database is opened, with an error that names the flag. Entries are
matched exactly (`internal.example` does not allow `a.internal.example`),
because patterns invite silent over-permission.

### The refusal message is actionable

`403 forbidden_host` keeps its code and names the offending host plus the flag:
`host "proxy.local" is not allowed; restart serve with --allow-host proxy.local`.
The UI banner adds the loopback address to open. The API does not need to know
the final port for this message.

### The shell owns connection state

`useHealthBanner` (in `App.tsx` or a small module) calls a typed
`api.health()` on mount, on `pageshow`, and on `visibilitychange` when the tab
becomes visible. It maps a `403 forbidden_host` to the host/flag hint, any
other failure to "server unreachable", and clears on success. One banner is
rendered above the routed page, reusing the existing `top-error` styling, so
per-page errors and toasts keep their current behavior.

Alternatives considered: intercepting every failed request (noisy, hides which
cause matters); a toast per failure (repeats and expires, wrong for a
persistent cause).

### Router syncs when it subscribes

`useRoute` keeps its state initializer but the effect calls the same handler
immediately after subscribing, and subscribes to `pageshow` as well as
`hashchange`. A hash change that lands between first paint and subscription is
therefore applied on the next tick, and a back/forward-cache restore re-reads
the address bar. `replaceRouteQuery` keeps using `replaceState` without firing
`hashchange`, so in-page filter changes still do not remount the page.

## Risks / Trade-offs

- **Widened allowlist on a wildcard bind.** Any interface IP and the hostname
  become acceptable `Host` values. Mitigation: only when the operator passes
  `--unsafe-bind`, which already warns that the API is unauthenticated.
- **Reverse proxies need one flag.** Anyone fronting the app with a tunnel or a
  proxy domain gets `403 forbidden_host` until `--allow-host` is set.
  Mitigation: the message and the UI banner both name the flag and the host.
- **Hostname coverage is locale-dependent.** `os.Hostname()` may return a short
  name while the browser uses an FQDN or mDNS name. Mitigation: documented, and
  `--allow-host` covers it; no DNS lookups are added.
- **Extra health request.** One request per mount/visibility restore; the
  endpoint is a cheap ping and stays unauthenticated like the rest of the API.
- **Interface enumeration at startup can miss addresses added later** (VPN
  up after serve starts). Mitigation: restart the server, or pass
  `--allow-host` for a stable name.

## Migration Plan

No database or API-contract change. Deploy by rebuilding the binary and
restarting `serve`; loopback users see no difference. Rollback is the previous
binary - the allowlist is computed at startup and stores nothing. Operators who
front the app with a proxy add `--allow-host <name>` on the next restart.

## Open Questions

None blocking.
