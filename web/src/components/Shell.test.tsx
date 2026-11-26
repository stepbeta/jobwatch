import { afterEach, describe, expect, it } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import styles from '../styles.css?raw'
import { Shell } from './Shell'
import type { Route } from '../router'

const cases: { route: Route; active: string | null }[] = [
  { route: { name: 'dashboard' }, active: 'Dashboard' },
  { route: { name: 'offers' }, active: 'Offers' },
  { route: { name: 'offer', id: 7 }, active: 'Offers' },
  { route: { name: 'company', id: 3 }, active: 'Offers' },
  { route: { name: 'incomplete' }, active: 'Incomplete' },
  { route: { name: 'sources' }, active: 'Sources' },
  { route: { name: 'interviewPrep' }, active: 'Interview prep' },
  { route: { name: 'settings' }, active: 'Settings' },
  { route: { name: 'notfound' }, active: null },
]

function renderShell(route: Route, incompleteCount = 0) {
  return render(
    <Shell route={route} incompleteCount={incompleteCount}>
      <div>page content</div>
    </Shell>,
  )
}

afterEach(() => {
  Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
})

describe('Shell', () => {
  it('shows every sidebar link and marks the active section for every route', () => {
    for (const { route, active } of cases) {
      const view = renderShell(route)
      const nav = screen.getByRole('navigation', { name: 'Sections' })
      const links = within(nav).getAllByRole('link')
      expect(links.map((link) => link.textContent?.replace(/\d+/g, '').trim())).toEqual([
        'Dashboard',
        'Offers',
        'Incomplete',
        'Sources',
        'Interview prep',
        'Settings',
      ])
      const current = within(nav)
        .queryAllByRole('link')
        .filter((link) => link.getAttribute('aria-current') === 'page')
      if (active === null) {
        expect(current).toHaveLength(0)
      } else {
        expect(current).toHaveLength(1)
        expect(current[0].textContent).toContain(active)
      }
      view.unmount()
    }
  })

  it('links every section to its route', () => {
    renderShell({ name: 'dashboard' })
    const nav = screen.getByRole('navigation', { name: 'Sections' })
    const hrefs = within(nav)
      .getAllByRole('link')
      .map((link) => link.getAttribute('href'))
    expect(hrefs).toEqual([
      '#/',
      '#/offers',
      '#/incomplete',
      '#/sources',
      '#/interview-prep',
      '#/settings',
    ])
  })

  it('shows the incomplete count badge only while offers are pending', () => {
    const view = renderShell({ name: 'incomplete' }, 3)
    const nav = screen.getByRole('navigation', { name: 'Sections' })
    const incomplete = within(nav).getByRole('link', { name: /Incomplete/ })
    expect(within(incomplete).getByText('3')).toBeTruthy()

    view.unmount()
    renderShell({ name: 'incomplete' }, 0)
    const navAgain = screen.getByRole('navigation', { name: 'Sections' })
    const incompleteAgain = within(navAgain).getByRole('link', { name: /Incomplete/ })
    expect(within(incompleteAgain).queryByText('3')).toBeNull()
  })

  it('keeps the header controls and every link at a narrow viewport', () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 800 })
    const { container } = renderShell({ name: 'dashboard' })

    expect(container.querySelector('.app-header .header-controls')).toBeTruthy()
    expect(container.querySelector('.app-header .brand')?.textContent).toBe('JobWatch')
    expect(container.querySelector('.app-header .brand svg')).toBeTruthy()
    expect(within(screen.getByRole('navigation')).getAllByRole('link')).toHaveLength(6)
  })

  it('collapses the sidebar through the shared 900px breakpoint', () => {
    expect(styles).toMatch(
      /@media \(max-width: 900px\) \{[\s\S]*?\.sidebar-inner\s*\{[\s\S]*?flex-direction: row;/,
    )
    expect(styles).toMatch(/\.sidebar\s*\{[\s\S]*?width: 220px;/)
  })

  it('paints the sidebar column separately from the sticky links', () => {
    const { container } = renderShell({ name: 'dashboard' })
    const nav = screen.getByRole('navigation', { name: 'Sections' })
    expect(nav.querySelector('.sidebar-inner')).toBeTruthy()
    expect(container.querySelector('.sidebar > .sidebar-inner')).toBeTruthy()
    expect(styles).toMatch(/\.app-body\s*\{[\s\S]*?align-items: stretch;/)
    expect(styles).toMatch(/\.sidebar-inner\s*\{[\s\S]*?position: sticky;/)
  })
})
