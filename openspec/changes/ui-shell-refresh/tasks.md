# Tasks

## 1. Application shell

- [x] 1.1 Replace the top-bar links with a full-width header and a left sidebar nav (Dashboard, Offers, Incomplete with count, Sources, Interview prep, Settings); mark the active section, including offer and company detail routes as Offers. Verify with a shell test covering every route and the active marker.
- [x] 1.2 Move the Incomplete count badge into the sidebar and keep it refreshed with the existing count fetch. Verify the badge appears and disappears in tests.
- [x] 1.3 Collapse the sidebar to a horizontal scrollable nav row at `max-width: 900px` while keeping the header controls. Verify via a CSS breakpoint check and a narrow-viewport render test.
- [x] 1.4 Update `docs/ARCHITECTURE.md` (component shell and navigation) and confirm it matches the new `App.tsx` structure.

## 2. Theme selection

- [x] 2.1 Rework the palette selectors so light is the `:root` default, dark applies for `:root[data-theme='dark']`, and the system dark palette is scoped to `:root:not([data-theme='light'])`. Verify both palettes still resolve and no unstyled state appears.
- [x] 2.2 Add the pre-paint inline script in `web/index.html` that reads the stored theme, sets `data-theme` and `color-scheme`, and defaults to `system`. Verify a reload in each stored mode paints correctly with no flash.
- [x] 2.3 Add the header theme control for `system` / `light` / `dark`, persist the choice, and update immediately. Verify with tests for default, explicit persisted choice, and returning to system.
- [x] 2.4 Document the theme control and default in `docs/USAGE.md` and `README.md`; verify the wording matches the control labels.

## 3. Browser identity

- [x] 3.1 Add `web/public/favicon.svg` and the `<link rel="icon">` default, and add the brand mark to the header beside the JobWatch name. Verify the icon is served by the embedded SPA and renders at 16px in both palettes.
- [x] 3.2 Add a document-title hook and route labels: `Dashboard — JobWatch`, `Offers — JobWatch`, `Incomplete — JobWatch`, `Sources — JobWatch`, `Interview prep — JobWatch`, `Settings — JobWatch`, `Page not found — JobWatch`. Verify with tests.
- [x] 3.3 Use the loaded data for detail titles: `<offer title> — <company> · JobWatch` and `<company> — JobWatch`, with loading fallbacks, and never include the application state. Verify with tests.
- [x] 3.4 Add the runtime SVG favicon module with the base mark and the eight application-state badges, and wire it to the open offer; restore the default on unmount and on state change. Verify with tests for the state map, the live update, and the reset.
- [x] 3.5 Update `docs/ARCHITECTURE.md` for the title and favicon ownership; verify the described owner matches the implementation.

## 4. Dashboard filters

- [x] 4.1 Remove the listing filter from both dashboard sections and replace the Applications application filter with a default-off `Hide closed applications` toggle. Verify the Applications query omits the application-state filter when off and sends the positive-state set when on.
- [x] 4.2 Persist the toggle as `closed=1` and ignore the old `listing` and `application` parameters. Verify reload and back/forward restore the toggle.
- [x] 4.3 Keep the refresh scope aligned with the visible applications and update the Dashboard tests for the new query and URL. Verify the whole dashboard test file passes.
- [x] 4.4 Update the dashboard description in `docs/ARCHITECTURE.md`; confirm it no longer mentions a listing filter.

## 5. Settings page

- [ ] 5.1 Rename the route and sidebar label to Settings (`#/settings`) and update the page heading; remove the old `search-profile` route and label. Verify the route test and the sidebar link.
- [ ] 5.2 Split the editor into Profile, Title terms, Location terms, Description and Remote tabs while keeping one form state and one Save. Verify switching tabs preserves edits.
- [ ] 5.3 Write the active tab to the URL as `tab=<id>` and restore it on load. Verify reload and direct-link tests.
- [ ] 5.4 Map validation errors to tabs and switch to the first offending tab on save; mark tabs with errors. Verify hidden-tab validation tests.
- [ ] 5.5 Update `docs/USAGE.md` for the Settings page and tabs; verify the labels match the UI.

## 6. Integration and release notes

- [ ] 6.1 Run `npm test`, `npm run build`, `go build ./...`, `go vet ./...` and `go test ./...`; verify all pass.
- [ ] 6.2 Manually smoke-test the shell, active states, titles, favicon state changes, theme switching, dashboard toggle and Settings tabs against a fresh build.
- [ ] 6.3 Add the `CHANGELOG.md` Unreleased entry for the shell, theme, titles/favicon, dashboard and Settings changes; verify it reads as a user-facing summary.

## Workflow follow-up

- Replace the `docs/SPEC.md` §9 Web UI section with a pointer to `openspec/specs/web-ui-shell/spec.md` when the change is archived.
- Archive `ui-shell-refresh` after all groups are reviewed and completed.
