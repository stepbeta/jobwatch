# REST API

Base path `/api`, JSON. Errors use:

```json
{ "error": { "code": "string", "message": "string" } }
```

Any unknown method/path under `/api/` returns `404` with code `not_found`.

## GET /api/health

Liveness check; pings the database.

Response `200`:

```json
{ "status": "ok", "version": "dev", "db": "ok" }
```

Response `503` when the database ping fails: `status` = `"degraded"`,
`db` = the error message.
