import { useEffect } from 'react'
import type { Route } from './router'

export const APP_NAME = 'JobWatch'

export function pageTitle(label: string): string {
  return `${label} — ${APP_NAME}`
}

export function offerDocumentTitle(title: string, company: string): string {
  return `${title} — ${company} · ${APP_NAME}`
}

export function companyDocumentTitle(name: string): string {
  return pageTitle(name)
}

// routeTitle returns the title for list pages and null for detail pages,
// which own their title after they know the entity. The application state
// is deliberately absent from every title.
export function routeTitle(route: Route): string | null {
  switch (route.name) {
    case 'dashboard':
      return pageTitle('Dashboard')
    case 'offers':
      return pageTitle('Offers')
    case 'incomplete':
      return pageTitle('Incomplete')
    case 'sources':
      return pageTitle('Sources')
    case 'interviewPrep':
      return pageTitle('Interview prep')
    case 'settings':
      return pageTitle('Settings')
    case 'notfound':
      return pageTitle('Page not found')
    case 'offer':
    case 'company':
      return null
  }
}

// useDocumentTitle writes a non-null title. Detail pages pass null until
// they know the entity, and an unmounting page never restores a stale
// title: the next route's effect owns the final value.
export function useDocumentTitle(title: string | null): void {
  useEffect(() => {
    if (title !== null) document.title = title
  }, [title])
}
