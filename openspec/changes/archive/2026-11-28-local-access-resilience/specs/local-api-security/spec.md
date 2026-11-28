# Spec Delta

## MODIFIED Requirements

### Requirement: Requests must target the bound host

The server SHALL reject, before routing, every request whose `Host` header
does not name an allowed host, returning `403 forbidden_host`. Allowed hosts
are: `localhost`, any name ending in `.localhost`, a loopback address, the
address the server was started on, an explicit `--allow-host` entry, and, only
for a wildcard bind, every local interface address and the machine hostname.
Any port SHALL be accepted.

#### Scenario: DNS rebinding attempt is rejected

- **WHEN** a request arrives with `Host: attacker.example`
- **THEN** the server returns `403 forbidden_host` and no API handler runs

#### Scenario: Loopback host with a development port is accepted

- **WHEN** the development proxy forwards a request with `Host: localhost:5173` to a server bound to `127.0.0.1:8080`
- **THEN** the request is routed normally

#### Scenario: Explicitly bound address is accepted

- **WHEN** the server was started with an explicit non-loopback address and a request uses that address in `Host`
- **THEN** the request is routed normally

#### Scenario: Wildcard bind accepts the machine's own addresses

- **WHEN** the server was started with `--addr 0.0.0.0 --unsafe-bind` and a request arrives with `Host` set to one of the machine's interface addresses or its hostname
- **THEN** the request is routed normally

#### Scenario: Wildcard bind rejects a foreign address

- **WHEN** the server was started with `--addr 0.0.0.0 --unsafe-bind` and a request arrives with `Host: 203.0.113.9`
- **THEN** the server returns `403 forbidden_host`

#### Scenario: Localhost subdomain is accepted

- **WHEN** a request arrives with `Host: jobwatch.localhost:8080`
- **THEN** the request is routed normally

#### Scenario: Tunnel host is accepted only when allowed

- **WHEN** the server was started with `--allow-host jobwatch.devtunnels.ms` and a request arrives with `Host: jobwatch.devtunnels.ms`
- **THEN** the request is routed normally
- **AND** the same request is rejected with `403 forbidden_host` when the flag is absent

#### Scenario: Rejected host names the remedy

- **WHEN** a request is rejected for its `Host`
- **THEN** the `403 forbidden_host` message names the rejected host and the `--allow-host` flag that would allow it

### Requirement: Cross-origin writes are rejected

The server SHALL reject any state-changing request that carries a non-empty
`Origin` header whose host is not in the same allowlist used for `Host`:
loopback, a `.localhost` name, the bound address, an explicit `--allow-host`
entry or, for a wildcard bind, a local interface address or the machine
hostname. The rejection SHALL return `403 forbidden_origin`. A request with no
`Origin` header SHALL remain acceptable so that local CLIs and tests keep
working.

#### Scenario: Foreign origin cannot create a source

- **WHEN** a page on `https://example.com` sends a `POST /api/sources` with `Origin: https://example.com`
- **THEN** the server returns `403 forbidden_origin` and no source is created

#### Scenario: Local development origin is accepted

- **WHEN** the browser at `http://localhost:5173` sends `PATCH /api/offers/1` through the development proxy
- **THEN** the request is routed normally

#### Scenario: Wildcard bind accepts the LAN origin

- **WHEN** the server was started with `--addr 0.0.0.0 --unsafe-bind` and the browser at `http://192.168.1.73:8080` sends `PATCH /api/offers/1`
- **THEN** the request is routed normally

#### Scenario: Missing origin is accepted

- **WHEN** a local `curl` call sends `POST /api/scan` with no `Origin` header
- **THEN** the request is routed normally

## ADDED Requirements

### Requirement: Explicit host entries are validated

`serve` SHALL accept `--allow-host` zero or more times. Each value MUST be a
host name or IP address with an optional port; an empty value, a URL or a value
carrying a path SHALL abort startup with an error that names the flag. Entries
SHALL be compared case-insensitively by host, ignoring the port.

#### Scenario: Several entries extend the allowlist

- **WHEN** the user passes `--allow-host internal.example --allow-host proxy.local`
- **THEN** both names are accepted as `Host` values

#### Scenario: A URL is refused

- **WHEN** the user passes `--allow-host https://internal.example/app`
- **THEN** the command exits with an error naming `--allow-host` and the server does not listen

#### Scenario: An empty entry is refused

- **WHEN** the user passes `--allow-host ""`
- **THEN** the command exits with an error naming `--allow-host` and the server does not listen
