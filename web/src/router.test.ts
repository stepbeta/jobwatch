import { describe, expect, it } from 'vitest'
import { parseRoute } from './router'

describe('parseRoute', () => {
  it('maps the dashboard, offers list and offer detail routes', () => {
    expect(parseRoute('#/')).toEqual({ name: 'dashboard' })
    expect(parseRoute('#/offers')).toEqual({ name: 'offers', saved: undefined })
    expect(parseRoute('#/offers?saved=true')).toEqual({ name: 'offers', saved: true })
    expect(parseRoute('#/offers?saved=false')).toEqual({ name: 'offers', saved: false })
    expect(parseRoute('#/offers/7')).toEqual({ name: 'offer', id: 7 })
  })
})
