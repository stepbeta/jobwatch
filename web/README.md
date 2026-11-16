# web

React + TypeScript (Vite) single-page app for JobWatch.

```sh
npm install
npm run dev     # http://127.0.0.1:5173, proxies /api to 127.0.0.1:8080
npm run build   # type-check + bundle into dist/
```

`dist/` is gitignored. The Go binary embeds it when built with
`-tags ui` (see `embed.go`); without the tag, `noui.go` serves a
placeholder page. `spa.go` holds the shared static handler with the
`index.html` fallback for client-side routes.

Routing is hash-based (`#/`, `#/sources`, `#/offers/:id`,
`#/companies/:id`) so no server rewrite rules are needed.
