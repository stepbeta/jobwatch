import type { ReactNode } from 'react'
import type { Route } from '../router'

// Section identifies the sidebar entry a route belongs to. Detail pages
// (offer and company) belong to Offers.
export type Section =
  | 'dashboard'
  | 'offers'
  | 'incomplete'
  | 'sources'
  | 'interviewPrep'
  | 'profile'

export function activeSection(route: Route): Section | null {
  switch (route.name) {
    case 'dashboard':
      return 'dashboard'
    case 'offers':
    case 'offer':
    case 'company':
      return 'offers'
    case 'incomplete':
      return 'incomplete'
    case 'sources':
      return 'sources'
    case 'interviewPrep':
      return 'interviewPrep'
    case 'profile':
      return 'profile'
    default:
      return null
  }
}

interface NavItem {
  section: Section
  href: string
  label: string
}

const NAV_ITEMS: NavItem[] = [
  { section: 'dashboard', href: '#/', label: 'Dashboard' },
  { section: 'offers', href: '#/offers', label: 'Offers' },
  { section: 'incomplete', href: '#/incomplete', label: 'Incomplete' },
  { section: 'sources', href: '#/sources', label: 'Sources' },
  { section: 'interviewPrep', href: '#/interview-prep', label: 'Interview prep' },
  { section: 'profile', href: '#/search-profile', label: 'Settings' },
]

export function Shell({
  route,
  incompleteCount,
  children,
}: {
  route: Route
  incompleteCount: number
  children: ReactNode
}) {
  const active = activeSection(route)
  return (
    <>
      <header className="app-header">
        <a className="brand" href="#/">
          JobWatch
        </a>
        <span className="spacer" />
        <div className="header-controls" />
      </header>
      <div className="app-body">
        <nav className="sidebar" aria-label="Sections">
          {NAV_ITEMS.map((item) => {
            const current = active === item.section
            return (
              <a
                key={item.section}
                href={item.href}
                className={current ? 'active' : undefined}
                aria-current={current ? 'page' : undefined}
              >
                <span>{item.label}</span>
                {item.section === 'incomplete' && incompleteCount > 0 && (
                  <span
                    className="nav-count"
                    aria-label={`${incompleteCount} incomplete offers`}
                  >
                    {incompleteCount}
                  </span>
                )}
              </a>
            )
          })}
        </nav>
        <main>{children}</main>
      </div>
    </>
  )
}
