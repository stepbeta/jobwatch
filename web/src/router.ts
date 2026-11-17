import { useEffect, useState } from 'react'

export type Route =
  | { name: 'dashboard' }
  | { name: 'offers'; saved?: boolean }
  | { name: 'sources' }
  | { name: 'interviewPrep' }
  | { name: 'offer'; id: number }
  | { name: 'company'; id: number }
  | { name: 'notfound' }

export function parseRoute(hash: string): Route {
  const path = hash.replace(/^#/, '') || '/'
  const [pathname, query = ''] = path.split('?')
  const params = new URLSearchParams(query)
  const parts = pathname.split('/').filter(Boolean)
  if (parts.length === 0) return { name: 'dashboard' }
  if (parts[0] === 'offers' && parts.length === 1) {
    const saved = params.get('saved')
    return { name: 'offers', saved: saved === 'true' ? true : saved === 'false' ? false : undefined }
  }
  if (parts[0] === 'sources' && parts.length === 1) return { name: 'sources' }
  if (parts[0] === 'interview-prep' && parts.length === 1) return { name: 'interviewPrep' }
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
    const onChange = () => setHash(window.location.hash || '#/')
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])
  return parseRoute(hash)
}

export function navigate(to: string): void {
  window.location.hash = to
}
