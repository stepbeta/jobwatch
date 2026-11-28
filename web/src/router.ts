import { useEffect, useState } from 'react'

export type Route =
  | { name: 'dashboard' }
  | { name: 'offers' }
  | { name: 'incomplete' }
  | { name: 'sources' }
  | { name: 'documents' }
  | { name: 'interviewPrep' }
  | { name: 'settings' }
  | { name: 'offer'; id: number }
  | { name: 'company'; id: number }
  | { name: 'notfound' }

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, '') || '/'
  const [pathname] = path.split('?')
  const parts = pathname.split('/').filter(Boolean)
  if (parts.length === 0) return { name: 'dashboard' }
  if (parts[0] === 'offers' && parts.length === 1) return { name: 'offers' }
  if (parts[0] === 'sources' && parts.length === 1) return { name: 'sources' }
  if (parts[0] === 'documents' && parts.length === 1) return { name: 'documents' }
  if (parts[0] === 'incomplete' && parts.length === 1) return { name: 'incomplete' }
  if (parts[0] === 'interview-prep' && parts.length === 1) return { name: 'interviewPrep' }
  if (parts[0] === 'settings' && parts.length === 1) return { name: 'settings' }
  if (parts[0] === 'offers' && parts[1]) {
    const id = Number(parts[1])
    return Number.isInteger(id) && id > 0 ? { name: 'offer', id } : { name: 'notfound' }
  }
  if (parts[0] === 'companies' && parts[1]) {
    const id = Number(parts[1])
    return Number.isInteger(id) && id > 0 ? { name: 'company', id } : { name: 'notfound' }
  }
  return { name: 'notfound' }
}

export function useRoute(): Route {
  const [hash, setHash] = useState(() => window.location.hash || '#/')
  useEffect(() => {
    const sync = () => setHash(window.location.hash || '#/')
    // Apply whatever the address bar says now. A change that landed between
    // the first render and this effect would otherwise be dropped until the
    // user clicked again, and a back/forward-cache restore would keep the
    // stale route.
    sync()
    window.addEventListener('hashchange', sync)
    window.addEventListener('pageshow', sync)
    return () => {
      window.removeEventListener('hashchange', sync)
      window.removeEventListener('pageshow', sync)
    }
  }, [])
  return parseRoute(hash)
}

export function navigate(to: string): void {
  window.location.hash = to
}

// currentRouteQuery returns the query string attached to the current hash
// route. Pages use it once on mount to restore filters, sort and page.
export function currentRouteQuery(): URLSearchParams {
  const hash = window.location.hash.replace(/^#/, '') || '/'
  const [, query = ''] = hash.split('?')
  return new URLSearchParams(query)
}

// replaceRouteQuery rewrites only the current hash route's query string.
// It uses replaceState so filter changes do not flood the history, and it
// deliberately does not fire hashchange: the page state is the source of
// truth while the user interacts with it.
export function replaceRouteQuery(params: URLSearchParams): void {
  const hash = window.location.hash.replace(/^#/, '') || '/'
  const path = hash.split('?')[0] || '/'
  const query = params.toString()
  const next = `#${path}${query ? `?${query}` : ''}`
  if (window.location.hash === next) return
  window.history.replaceState(null, '', next)
}

export function intParam(params: URLSearchParams, key: string): number | undefined {
  const raw = params.get(key)
  if (raw === null || raw.trim() === '') return undefined
  const n = Number(raw)
  return Number.isInteger(n) ? n : undefined
}

export function enumParam<T extends string>(
  params: URLSearchParams,
  key: string,
  allowed: readonly T[],
): T | undefined {
  const raw = params.get(key)
  return raw !== null && (allowed as readonly string[]).includes(raw) ? (raw as T) : undefined
}
