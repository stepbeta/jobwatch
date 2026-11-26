# Proposal

## Why

The archived `ui-shell-refresh` shipped with four presentation defects: the
sidebar background ends after the links instead of filling the page, the theme
control is a select instead of a compact icon, the dashboard hide-closed
control is still a checkbox with an oversized input gap, and the brand mark
reads as a stopwatch rather than a watch/radar mark.

## What Changes

- Make the sidebar column paint the full page height while keeping the links
  sticky below the header.
- Replace the theme select with a single icon button that cycles
  `system → light → dark → system`, with the current mode in its accessible
  label and tooltip.
- Replace the dashboard checkbox with the existing toggle-switch control,
  left-aligned and without the generic filter input width.
- Redraw the brand mark as a radar sweep and blips, and mirror it in the static
  favicon.

## Capabilities

### New Capabilities

None. These are visual and interaction corrections within the existing
`web-ui-shell` and `application-lifecycle` requirements; no requirement text
changes.

### Modified Capabilities

None.

## Impact

- Frontend only: `web/src/components/Shell.tsx`, `ThemeControl.tsx`,
  `icons.tsx`, `web/src/pages/Dashboard.tsx`, `web/src/brand.ts`,
  `web/src/styles.css`, `web/public/favicon.svg`, and their tests.
- No REST API, schema, Go backend or dependency changes.
