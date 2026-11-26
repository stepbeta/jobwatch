# Design

## Context

The SPA is a hash-routed React app embedded with `go:embed`. `App.tsx` owns
the top bar and route switch; each page owns its own data loading and hash
query state. The light and dark palettes already exist in `styles.css` behind
`@media (prefers-color-scheme: dark)`, and `index.html` declares
`color-scheme: light dark`. No browser title or favicon is managed today.
See `proposal.md` for motivation and `specs/` for the required behavior.

## Goals / Non-Goals

**Goals:**

- One shell component that owns the header, sidebar, theme control and page
  chrome, without moving page data into it.
- Browser title and favicon driven by the leaf page that knows the entity.
- A theme selection that changes only presentation and never data.
- Settings tabs that share one form state and one Save.

**Non-Goals:**

- Tabbed offer workspaces or side-by-side offer panes.
- A user account, avatar upload or server-side preferences.
- Backend, schema, API or dependency changes.
- Redirecting the old `#/search-profile` route.

## Decisions

### Shell layout

`App.tsx` keeps a single header and a `<nav>` sidebar; the route switch stays
inside `<main>`. The header is full width, with the brand mark on the left and
the theme control on the right. The sidebar sits under the header, fixed at
220px, and marks the active section with `aria-current="page"` plus an accent
bar. Offer and company detail pages map to Offers for highlighting.

At `max-width: 900px`, the sidebar becomes a horizontal, scrollable nav row
below the header rather than a drawer. A drawer needs an overlay, focus trap
and a menu button; a row keeps the existing dense-table breakpoint and costs
much less. The header controls remain visible at every width.

### Theme selection

The theme is stored in `localStorage` as `system`, `light` or `dark`. An inline
script in `index.html` reads it before paint and sets `data-theme` on `<html>`
and the matching `color-scheme`, preventing a flash. `styles.css` keeps the
light variables on `:root`, applies the dark variables for
`:root[data-theme='dark']`, and keeps the `prefers-color-scheme` block scoped to
`:root:not([data-theme='light'])` for the system mode. The header control is a
three-state menu, not a two-state switch, because "system" is a real choice.
The default when nothing is stored is `system`.

### Browser title and favicon

A small `useDocumentTitle` hook sets `document.title` from a route label. List
pages resolve their label synchronously; `OfferPage` and `CompanyPage` start
with `Offer — JobWatch` / `Company — JobWatch` and replace it after the fetch.
The application state is never included. An unmounting detail page does not
write a title; the next route's effect owns the final value.

The favicon is generated in `web/src/favicon.ts`. A base 32×32 SVG geometry
(the same watch mark used in the header) is combined with a state badge: a
circle in the bottom-right corner with a white cut-out ring, a state colour and
a simple glyph. `faviconFor(applicationState)` returns a
`data:image/svg+xml` URL; the module updates the existing
`<link rel="icon">` href. Distinct data URIs avoid browser caching of the
previous state. Only `OfferPage` writes a state favicon and its effect cleanup
restores the default. The static `web/public/favicon.svg` is the no-JS default
and mirrors the base geometry.

Glyph choices are colour-first because a 7px badge cannot carry detail:

| State | Colour | Glyph |
|---|---|---|
| `none` | — | no badge |
| `saved` | amber | bookmark |
| `applied` | blue | stopwatch |
| `interview` | amber | speech dot |
| `offer` | green | star |
| `accepted` | green | check |
| `turned_down` | muted | minus |
| `negative` | red | cross |

The stopped/clock glyph is preferred over an hourglass: it stays readable at
16px and conveys "waiting" without diagonal strokes.

### Settings tabs

`SearchProfilePage` becomes `SettingsPage` at `#/settings`. The form state stays
at page level; tabs only control which field group is rendered. The active tab
is written to the hash query as `tab=<id>` through `replaceRouteQuery`, so a
reload or direct link opens the same tab without adding history entries. The
tab list uses `role="tablist"`, buttons with `aria-selected`, and the existing
focus ring. A single Save button submits the whole profile. Validation maps
each error to a tab; saving switches to the first offending tab before showing
the error. This avoids an error hidden behind an inactive tab.

### Dashboard filters

The listing filter is removed from both sections. The Applications section
keeps one `Hide closed applications` checkbox. When checked, the existing
`HIDE_NEGATIVE_STATES` query is sent; otherwise the query relies only on
`has_application=true`. The URL stores `closed=1`, and the old `listing` /
`application` parameters are ignored. The Saved section loses its filter and
relies on the existing Offer column to show `disappeared`.

## Risks / Trade-offs

- **Favicon glyphs are unreadable at 16px** → rely on the badge colour and keep
  glyphs to one or two strokes; the browser title is not used for state, so the
  favicon is a glance only.
- **SVG favicon support varies** → Chromium and Firefox support it; the static
  `favicon.svg` remains as the default, and the app still works without a
  dynamic favicon.
- **Theme flash on reload** → the inline pre-paint script sets `data-theme`
  before React loads.
- **Hidden Settings validation** → saving switches to the offending tab; tab
  labels carry an error marker.
- **Title/favicon ownership across page switches** → only the mounted leaf page
  writes them, and effects restore defaults on unmount.
- **Sidebar at 900px** → the horizontal nav row can scroll; no overlay or focus
  trap is introduced.

## Migration Plan

No data migration. The frontend is rebuilt and embedded; reverting the commit
restores the previous shell. Users lose the old dashboard filter parameters and
the `#/search-profile` route, both intentionally.
