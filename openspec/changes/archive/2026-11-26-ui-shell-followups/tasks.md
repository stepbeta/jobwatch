# Tasks

## 1. Shell and control corrections

- [x] 1.1 Wrap the sidebar nav in `.sidebar-inner`, stretch `.app-body`, and move the sticky positioning and scroll cap to the inner wrapper. Verify with the Shell test (wrapper exists) and a manual tall-page check that the column paints to the page bottom while the links stay sticky.
- [x] 1.2 Replace the theme select with a single icon button that cycles `system → light → dark → system`, using monitor, sun and moon icons and an accessible label that names the next mode. Verify with updated ThemeControl tests for default, persistence and cycling.
- [x] 1.3 Replace the dashboard checkbox with the existing toggle-switch, left-aligned outside `.filters`, and keep the `closed=1` URL state. Verify with updated Dashboard tests using the switch role.
- [x] 1.4 Redraw `brandMarkShapes` as a radar (body, face, two rings, sweep and blip) and mirror it in `web/public/favicon.svg`; verify the header mark and default favicon render at 16px and the state badges still overlay correctly.
- [x] 1.5 Update `docs/ARCHITECTURE.md` (sidebar wrapper, theme control, radar mark), `docs/USAGE.md` (theme icon cycle) and the `CHANGELOG.md` Unreleased entry; verify the wording matches the controls.
- [x] 1.6 Run `npm test`, `npm run build`, `go build ./...`, `go vet ./...` and `go test ./...`; verify all pass.
