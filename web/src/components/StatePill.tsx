import type { ReactNode } from 'react'

export function StatePill({ kind, children }: { kind: string; children: ReactNode }) {
  return <span className={`pill pill-${kind}`}>{children}</span>
}
