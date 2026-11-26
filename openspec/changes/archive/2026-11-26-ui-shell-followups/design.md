# Design

## Context

The shell is a full-width header above a flex row containing `.sidebar` and
`<main>`. The sidebar is `position: sticky` with a `max-height`, and the row
uses `align-items: flex-start`, so the column height follows the nav content
instead of the page. The theme control is a `<select>`; the dashboard uses a
plain checkbox inside `.filters`, whose `input { min-width: 110px }` rule also
applies to checkboxes. The brand geometry is shared by `brand.ts` and the
static favicon. See `proposal.md` for motivation.

## Goals / Non-Goals

**Goals:**

- Full-height sidebar column with sticky navigation.
- A compact, discoverable theme control without a menu.
- A native-feeling toggle for the dashboard filter.
- A radar mark that stays legible at 16px and keeps the state badge overlay.

**Non-Goals:**

- Changing any requirement, API, schema or data.
- Adding a mobile drawer; the 900px horizontal nav row stays.

## Decisions

### Sidebar wrapper for full-height paint

Wrap the nav links in `.sidebar-inner`:

```
<div class="app-body">
  <nav class="sidebar"><div class="sidebar-inner">...</div></nav>
  <main>...</main>
</div>
```

`.app-body` becomes `align-items: stretch`. `.sidebar` paints the full column
(background and right border) and loses `max-height`; `.sidebar-inner` keeps
`position: sticky; top: 43px; max-height: calc(100vh - 43px); overflow-y:
auto` and the padding. This separates "paint the column" from "keep the links
visible", which a single sticky element cannot do once the page is taller than
the viewport. The 900px breakpoint resets the inner wrapper to a static,
horizontal row.

Alternative: set a background on `.app-body` with a gradient. Rejected because
the border and hover surfaces would need duplicating and the column would not
be a real box.

### Theme icon cycle

The header renders one `.icon-button` with a theme glyph: a monitor for
`system`, a sun for `light`, a moon for `dark`. Clicking advances
`system → light → dark → system`. The button's `aria-label` is
`Theme: <current>. Switch to <next>` and its `title` repeats that text, so the
cycle is explainable without a menu. The existing `useTheme` persistence and
pre-paint script stay unchanged.

Alternative: a popover menu with three options. Rejected: more chrome, more
focus management, and the user asked for one icon.

### Dashboard toggle

Reuse the existing `.toggle-switch` / `.toggle-track` markup, with
`role="switch"` and an accessible label. Place it in a dedicated
`.filter-toggle` row with `display: flex; gap: 8px; align-items: center`, not
inside `.filters`, so the generic `min-width: 110px` input rule cannot apply.
The label text remains "Hide closed applications".

### Radar mark

Replace the stopwatch crown and hands with:

- an accent body circle;
- a face circle;
- two concentric accent rings;
- one sweep line from the centre;
- one blip dot on the sweep.

The geometry stays in `brandMarkShapes` so the header and generated favicon
share it. `web/public/favicon.svg` is updated to the same shapes. The state
badge still overlays the bottom-right corner, and the base stays two-tone for
16px legibility.

## Risks / Trade-offs

- **Sticky inner nav inside a stretched column** → the wrapper carries the
  paint and the inner element carries `sticky`; test a page taller than the
  viewport.
- **Three-state icon is less discoverable than a labelled select** → the
  tooltip and accessible label state the current and next mode; the icon still
  changes visibly.
- **Radar detail at 16px** → keep two rings and one blip, no thin spokes.
- **Checkbox-to-switch changes test queries** → update the Dashboard test to
  use the `switch` role and the label.

## Migration Plan

No data migration. Rebuild and re-embed the frontend; reverting the commit
restores the previous presentation.
