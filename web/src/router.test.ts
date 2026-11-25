import { describe, expect, it } from 'vitest'
import { parseRoute } from './router'

describe('parseRoute', () => {
  it('maps the dashboard, offers list and offer detail routes', () => {
    expect(parseRoute('#/')).toEqual({ name: 'dashboard' })
    expect(parseRoute('#/offers')).toEqual({ name: 'offers' })
    expect(parseRoute('#/offers?application_state=saved')).toEqual({ name: 'offers' })
    expect(parseRoute('#/offers/7')).toEqual({ name: 'offer', id: 7 })
    expect(parseRoute('#/incomplete')).toEqual({ name: 'incomplete' })
    expect(parseRoute('#/incomplete/7')).toEqual({ name: 'notfound' })
    expect(parseRoute('#/interview-prep')).toEqual({ name: 'interviewPrep' })
    expect(parseRoute('#/interview-prep/7')).toEqual({ name: 'notfound' })
  })
})
