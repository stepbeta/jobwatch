import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import {
  companyDocumentTitle,
  offerDocumentTitle,
  routeTitle,
  useDocumentTitle,
} from './title'
import type { Route } from './router'

describe('routeTitle', () => {
  const cases: [Route, string | null][] = [
    [{ name: 'dashboard' }, 'Dashboard — JobWatch'],
    [{ name: 'offers' }, 'Offers — JobWatch'],
    [{ name: 'incomplete' }, 'Incomplete — JobWatch'],
    [{ name: 'sources' }, 'Sources — JobWatch'],
    [{ name: 'interviewPrep' }, 'Interview prep — JobWatch'],
    [{ name: 'profile' }, 'Settings — JobWatch'],
    [{ name: 'notfound' }, 'Page not found — JobWatch'],
    [{ name: 'offer', id: 7 }, null],
    [{ name: 'company', id: 3 }, null],
  ]

  it('labels every list route and leaves detail routes to the page', () => {
    for (const [route, title] of cases) {
      expect(routeTitle(route)).toBe(title)
    }
  })
})

describe('detail titles', () => {
  it('uses the offer title and company and never the application state', () => {
    expect(offerDocumentTitle('Site Reliability Engineer', 'Acme')).toBe(
      'Site Reliability Engineer — Acme · JobWatch',
    )
    expect(offerDocumentTitle('SRE', 'Acme')).not.toContain('Applied')
    expect(companyDocumentTitle('Acme')).toBe('Acme — JobWatch')
  })
})

describe('useDocumentTitle', () => {
  it('writes the title and ignores null while a detail page loads', () => {
    const { rerender } = renderHook(
      ({ title }: { title: string | null }) => useDocumentTitle(title),
      { initialProps: { title: 'Offers — JobWatch' as string | null } },
    )
    expect(document.title).toBe('Offers — JobWatch')

    rerender({ title: 'Dashboard — JobWatch' })
    expect(document.title).toBe('Dashboard — JobWatch')

    rerender({ title: null })
    expect(document.title).toBe('Dashboard — JobWatch')
  })
})
