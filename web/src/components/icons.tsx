import type { ReactNode } from 'react'
import type { EventType } from '../types'

// Inline SVG icons keep the UI dependency-free. Every icon uses
// currentColor so it follows the surrounding text colour in both themes.

const EVENT_PATHS: Record<EventType, ReactNode> = {
  cv_sent: (
    <>
      <path d="M21 3 3 10.5l7 2.5 2.5 7L21 3Z" />
      <path d="M10 13 21 3" />
    </>
  ),
  message_received: (
    <>
      <path d="M4 5h16v11H4z" />
      <path d="M8 9l4 4 4-4" />
    </>
  ),
  message_sent: (
    <>
      <path d="M4 20 20 4" />
      <path d="M10 4h10v10" />
    </>
  ),
  interview: (
    <>
      <circle cx="9" cy="8" r="3" />
      <path d="M3 20c0-3.3 2.7-5.5 6-5.5s6 2.2 6 5.5" />
      <circle cx="17.5" cy="9" r="2.5" />
      <path d="M15 20c0-2.6 1.8-4.2 4.5-4.2" />
    </>
  ),
  offer: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="m8 12 3 3 5-6" />
    </>
  ),
  rejected: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="m9 9 6 6M15 9l-6 6" />
    </>
  ),
  ghosted: (
    <>
      <path d="M3 3l18 18" />
      <path d="M10.6 6.2A9.8 9.8 0 0 1 12 6c5 0 9 6 9 6a17 17 0 0 1-3 3.6" />
      <path d="M6.6 6.6C4 8.3 3 12 3 12s4 6 9 6a9.7 9.7 0 0 0 4.4-1" />
    </>
  ),
  other: (
    <>
      <circle cx="12" cy="12" r="8" />
      <circle cx="12" cy="12" r="2" fill="currentColor" stroke="none" />
    </>
  ),
}

export function EventIcon({
  type,
  size = 14,
  className,
}: {
  type: EventType
  size?: number
  className?: string
}) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {EVENT_PATHS[type]}
    </svg>
  )
}

export type ActionIconName = 'edit' | 'trash' | 'note' | 'refresh'

const ACTION_PATHS: Record<ActionIconName, ReactNode> = {
  edit: (
    <>
      <path d="M4 20h4L19 9l-4-4L4 16v4Z" />
      <path d="m13.5 6.5 4 4" />
    </>
  ),
  trash: (
    <>
      <path d="M4 7h16" />
      <path d="M9 7V4h6v3" />
      <path d="M6 7l1 13h10l1-13" />
      <path d="M10 11v5M14 11v5" />
    </>
  ),
  note: (
    <>
      <path d="M5 4h14v11l-5 5H5z" />
      <path d="M14 20v-5h5" />
      <path d="M8 9h8M8 12h5" />
    </>
  ),
  refresh: (
    <>
      <path d="M20 11a8 8 0 1 0-2.3 5.6" />
      <path d="M20 4v7h-7" />
    </>
  ),
}

export function ActionIcon({
  name,
  size = 14,
  className,
}: {
  name: ActionIconName
  size?: number
  className?: string
}) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {ACTION_PATHS[name]}
    </svg>
  )
}
