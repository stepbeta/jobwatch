# Proposal

## Why

The top bar currently carries every section link, the browser tab is always
titled "JobWatch", and the search profile editor is one long form. With offers
opened in separate browser tabs there is no at-a-glance identity, the dashboard
spends three controls on filters that reduce to one meaningful choice, and the
densest settings screen is hard to navigate.

## What Changes

- Move section navigation from the top bar into a left sidebar. Keep a
  full-width header above it for the brand and right-side controls.
- Add a working **system / light / dark** theme control in the header. The light
  and dark variable sets already exist behind `prefers-color-scheme`; this adds
  the selection mechanics and persistence.
- Add a brand mark used in the header and as the default favicon.
- Set `document.title` per route. Offer pages use
  `<title> — <company> · JobWatch`; company pages use `<name> — JobWatch`.
  The application state MUST NOT appear in the title.
- Add a runtime-generated SVG favicon badge for the offer's application state:
  the base mark for `none`, a coloured corner badge for `saved`, `applied`,
  `interview`, `offer`, `accepted`, `turned_down` and `negative`. The default
  favicon returns when the offer page is left.
- Rename **Search profile** to **Settings** at `#/settings` and split the editor
  into tabs: Profile, Title terms, Location terms, Description and Remote. Keep
  one Save for the whole profile; switching tabs preserves unsaved edits; the
  active tab lives in the URL.
- Dashboard: remove the listing (`all` / `active only`) filter from both
  sections and replace the Applications application filter with a default-off
  **Hide closed applications** toggle (`?closed=1`). Saved keeps no filter.
- Collapse the sidebar at the existing 900px breakpoint; the header controls
  remain available.

## Capabilities

### New Capabilities

- `web-ui-shell`: the persistent header and sidebar frame, browser page titles,
  the offer-state favicon, theme selection and the brand mark.

### Modified Capabilities

- `application-lifecycle`: the dashboard no longer filters on listing state and
  exposes a single hide-closed application toggle.
- `search-profiles`: the profile editor becomes a tabbed Settings page, with
  description terms moved from an advanced disclosure into a tab.

## Impact

- Frontend: `web/src/App.tsx`, `web/src/router.ts`, `web/src/styles.css`,
  `Dashboard.tsx`, `SearchProfilePage.tsx`, `OfferPage.tsx`, new shell and
  browser-chrome components, `web/index.html`, and a `web/public/favicon.svg`.
- No REST API, database schema, Go backend or new dependency changes.
- Tests: new tests for title/favicon/theme/shell behaviour and Settings tabs;
  update the dashboard filter tests.
