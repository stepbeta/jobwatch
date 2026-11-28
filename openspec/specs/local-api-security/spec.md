# local-api-security Specification

## Purpose

Defines which requests the local HTTP API accepts from the browser and the
network, so that a page the user visits cannot drive the API and a rebound
DNS name cannot read personal data.

## Requirements

### Requirement: Requests must target the bound host

The server SHALL reject every request whose `Host` header does not name a
loopback address (`127.0.0.0/8`, `[::1]`) or the exact address the server
was started on, with any port. When the server was started with an explicit
non-loopback address, that address SHALL also be accepted. The rejection
SHALL return `403 forbidden_host` and MUST happen before the request is
routed.

#### Scenario: DNS rebinding attempt is rejected

- **WHEN** a request arrives with `Host: attacker.example`
- **THEN** the server returns `403 forbidden_host` and no API handler runs

#### Scenario: Loopback host with a development port is accepted

- **WHEN** the development proxy forwards a request with `Host: localhost:5173` to a server bound to `127.0.0.1:8080`
- **THEN** the request is routed normally

#### Scenario: Explicitly bound address is accepted

- **WHEN** the server was started with an explicit non-loopback address and a request uses that address in `Host`
- **THEN** the request is routed normally

### Requirement: Cross-origin writes are rejected

The server SHALL reject any state-changing request that carries a non-empty
`Origin` header whose host is not a loopback address or the server's
explicitly bound address. The rejection SHALL return `403 forbidden_origin`.
A request with no `Origin` header SHALL remain acceptable so that local CLIs
and tests keep working.

#### Scenario: Foreign origin cannot create a source

- **WHEN** a page on `https://example.com` sends a `POST /api/sources` with `Origin: https://example.com`
- **THEN** the server returns `403 forbidden_origin` and no source is created

#### Scenario: Local development origin is accepted

- **WHEN** the browser at `http://localhost:5173` sends `PATCH /api/offers/1` through the development proxy
- **THEN** the request is routed normally

#### Scenario: Missing origin is accepted

- **WHEN** a local `curl` call sends `POST /api/scan` with no `Origin` header
- **THEN** the request is routed normally

### Requirement: Bodies are JSON, apart from attachment uploads

A state-changing request whose body is decoded as JSON SHALL declare
`Content-Type: application/json`. The multipart attachment upload SHALL be
the only exception. A bodyless request, or a route that ignores its body
(for example `POST /api/sources/{id}/scan`), MUST remain acceptable
without a content type. A request that violates this requirement SHALL
return `415 unsupported_media_type`, and it MUST be rejected before the
body is decoded.

#### Scenario: Plain-text body is rejected

- **WHEN** a request sends `POST /api/notes` with `Content-Type: text/plain`
- **THEN** the server returns `415 unsupported_media_type` and no note is created

#### Scenario: Attachment upload keeps working

- **WHEN** the UI sends `POST /api/offers/1/attachments` with `multipart/form-data`
- **THEN** the upload is processed normally

#### Scenario: Bodyless write needs no content type

- **WHEN** a request sends `POST /api/sources/1/scan` with no body
- **THEN** the request is routed normally

### Requirement: Non-loopback binds are explicit

`serve` SHALL refuse to bind a non-loopback address unless the
`--unsafe-bind` flag is passed, and SHALL print a warning when it binds with
that flag. The default address MUST stay `127.0.0.1`.

#### Scenario: Public bind is refused

- **WHEN** the user runs `jobwatch serve --addr 0.0.0.0` without `--unsafe-bind`
- **THEN** the command exits with an error and the server does not listen

#### Scenario: Public bind is allowed with a warning

- **WHEN** the user runs `jobwatch serve --addr 0.0.0.0 --unsafe-bind`
- **THEN** the server listens and prints a warning that the API has no authentication
