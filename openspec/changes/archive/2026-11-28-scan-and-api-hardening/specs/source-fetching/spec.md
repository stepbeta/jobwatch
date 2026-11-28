# Spec Delta

## Purpose

Defines the politeness rules the shared HTTP client applies to every source
request, including every step of a redirect chain, so that scanning stays
within robots.txt and rate limits.

## ADDED Requirements

### Requirement: Every redirect hop is checked

The shared client SHALL follow HTTP redirects itself, at most five hops per
request, and SHALL apply `robots.txt` and the per-host request interval to
each hop before it is requested. A redirect to a scheme other than `http` or
`https`, or to a hop disallowed by `robots.txt`, SHALL fail the request
without contacting that hop.

#### Scenario: Redirect to another host re-checks robots

- **WHEN** a source URL redirects to a host whose robots.txt disallows the target path
- **THEN** the request fails as blocked and the target is not fetched

#### Scenario: Redirect chain is rate limited per host

- **WHEN** a request follows a redirect to a second host
- **THEN** the client waits for the second host's interval before fetching it

#### Scenario: Redirect loop is stopped

- **WHEN** a redirect chain exceeds five hops
- **THEN** the request fails with an error naming the original URL

#### Scenario: Non-HTTP redirect is refused

- **WHEN** a redirect points at `ftp://` or another non-HTTP scheme
- **THEN** the request fails without contacting the target
