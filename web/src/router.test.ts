import { beforeEach, describe, expect, it } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { parseRoute, replaceRouteQuery, useRoute } from './router'

describe('parseRoute', () => {
  it('maps the dashboard, offers list and offer detail routes', () => {
    expect(parseRoute('#/')).toEqual({ name: 'dashboard' })
    expect(parseRoute('#/offers')).toEqual({ name: 'offers' })
    expect(parseRoute('#/offers?application_state=saved')).toEqual({ name: 'offers' })
    expect(parseRoute('#/offers/7')).toEqual({ name: 'offer', id: 7 })
    expect(parseRoute('#/incomplete')).toEqual({ name: 'incomplete' })
    expect(parseRoute('#/incomplete/7')).toEqual({ name: 'notfound' })
    expect(parseRoute('#/documents')).toEqual({ name: 'documents' })
    expect(parseRoute('#/documents/7')).toEqual({ name: 'notfound' })
    expect(parseRoute('#/interview-prep')).toEqual({ name: 'interviewPrep' })
    expect(parseRoute('#/interview-prep/7')).toEqual({ name: 'notfound' })
    expect(parseRoute('#/settings')).toEqual({ name: 'settings' })
    expect(parseRoute('#/settings?tab=remote')).toEqual({ name: 'settings' })
    expect(parseRoute('#/search-profile')).toEqual({ name: 'notfound' })
  })
})

beforeEach(() => {
  window.location.hash = '#/'
})

describe('useRoute', () => {
  it('applies the address bar hash on mount', () => {
    window.location.hash = '#/sources'

    const { result } = renderHook(() => useRoute())

    expect(result.current).toEqual({ name: 'sources' })
  })

  it('applies a hash change that lands before the listener attaches', () => {
    const { result } = renderHook(() => useRoute())
    expect(result.current).toEqual({ name: 'dashboard' })

    // The address bar moves during startup, before hashchange is handled.
    window.location.hash = '#/offers'
    act(() => {
      window.dispatchEvent(new Event('pageshow'))
    })

    expect(result.current).toEqual({ name: 'offers' })
  })

  it('applies the hash when the page is restored from the cache', () => {
    const { result } = renderHook(() => useRoute())

    window.location.hash = '#/companies/12'
    act(() => {
      window.dispatchEvent(new Event('pageshow'))
    })

    expect(result.current).toEqual({ name: 'company', id: 12 })
  })

  it('keeps the page when only the query is rewritten', () => {
    window.location.hash = '#/offers'
    const { result } = renderHook(() => useRoute())

    act(() => {
      replaceRouteQuery(new URLSearchParams({ per_page: '20' }))
    })

    expect(window.location.hash).toBe('#/offers?per_page=20')
    expect(result.current).toEqual({ name: 'offers' })
  })
})
