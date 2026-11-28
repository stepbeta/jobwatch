import { describe, expect, it } from 'vitest'
import { eventTimestamp } from './format'

describe('eventTimestamp', () => {
  it('sends a date-only value as local midnight on the entered date', () => {
    const iso = eventTimestamp('2026-11-12', '')
    expect(iso).not.toBe('')

    // Parse the instant back in the test's local zone: the calendar date
    // and the midnight wall clock must survive, whatever the offset is.
    const d = new Date(iso)
    expect(d.getFullYear()).toBe(2026)
    expect(d.getMonth()).toBe(10) // November, zero-based
    expect(d.getDate()).toBe(12)
    expect(d.getHours()).toBe(0)
    expect(d.getMinutes()).toBe(0)
    expect(d.getSeconds()).toBe(0)
  })

  it('keeps the entered local instant when a time is present', () => {
    const iso = eventTimestamp('2026-11-12', '14:30')
    expect(iso).toBe(new Date('2026-11-12T14:30').toISOString())

    const d = new Date(iso)
    expect(d.getFullYear()).toBe(2026)
    expect(d.getMonth()).toBe(10)
    expect(d.getDate()).toBe(12)
    expect(d.getHours()).toBe(14)
    expect(d.getMinutes()).toBe(30)
  })

  it('returns an empty string for a missing or invalid date', () => {
    expect(eventTimestamp('', '09:00')).toBe('')
    expect(eventTimestamp('not-a-date', '09:00')).toBe('')
  })
})
