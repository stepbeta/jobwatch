import type {
  CompanyDetail,
  CompanyList,
  Event,
  EventType,
  Note,
  OfferDetail,
  OfferList,
  ScanResponse,
  Source,
  SourceType,
} from './types'

export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(code: string, message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
  }
}

type QueryValue = string | number | boolean | undefined | null

function queryString(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    search.set(key, String(value))
  }
  const s = search.toString()
  return s ? `?${s}` : ''
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  let body: unknown = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      throw new ApiError('invalid_response', 'The server returned invalid JSON', res.status)
    }
  }
  if (!res.ok) {
    const err = (body as { error?: { code?: string; message?: string } } | null)?.error
    throw new ApiError(err?.code ?? 'http_error', err?.message ?? res.statusText, res.status)
  }
  return body as T
}

function body(method: string, data: unknown): RequestInit {
  return { method, body: JSON.stringify(data) }
}

export interface OfferQuery {
  q?: string
  application_state?: string
  offer_state?: string
  remote_policy?: string
  min_title_confidence?: number
  min_location_confidence?: number
  company_id?: number
  has_application?: boolean
  sort?: string
  limit?: number
  offset?: number
}

export interface SourceInput {
  name: string
  url: string
  source_type: SourceType
  config: Record<string, unknown>
  enabled: boolean
}

export const api = {
  // sources
  listSources: () => request<{ sources: Source[] }>('/sources'),
  createSource: (input: SourceInput) => request<{ source: Source }>('/sources', body('POST', input)),
  updateSource: (id: number, input: Partial<SourceInput>) =>
    request<{ source: Source }>(`/sources/${id}`, body('PATCH', input)),
  deleteSource: (id: number) => request<void>(`/sources/${id}`, { method: 'DELETE' }),
  scanSource: (id: number) => request<ScanResponse>(`/sources/${id}/scan`, { method: 'POST' }),

  // offers
  listOffers: (query: OfferQuery = {}) => request<OfferList>(`/offers${queryString({ ...query })}`),
  getOffer: (id: number) => request<OfferDetail>(`/offers/${id}`),
  patchOffer: (
    id: number,
    patch: { application_state?: string; offer_state?: string; location?: string | null },
  ) => request<{ offer: OfferDetail['offer'] }>(`/offers/${id}`, body('PATCH', patch)),
  importOffer: (input: {
    company: string
    title: string
    location?: string
    url?: string
    description?: string
  }) => request<{ offer_id: number; created: boolean }>('/offers/import', body('POST', input)),

  // events
  listEvents: (offerId: number) => request<{ events: Event[] }>(`/offers/${offerId}/events`),
  createEvent: (
    offerId: number,
    input: { type: EventType; description?: string; occurred_at?: string },
  ) => request<{ event: Event }>(`/offers/${offerId}/events`, body('POST', input)),
  updateEvent: (
    offerId: number,
    eventId: number,
    patch: { type?: EventType; description?: string | null; occurred_at?: string },
  ) => request<{ event: Event }>(`/offers/${offerId}/events/${eventId}`, body('PATCH', patch)),
  deleteEvent: (offerId: number, eventId: number) =>
    request<void>(`/offers/${offerId}/events/${eventId}`, { method: 'DELETE' }),

  // notes
  listNotes: (entityType: 'offer' | 'company', entityId: number) =>
    request<{ notes: Note[] }>(`/notes${queryString({ entity_type: entityType, entity_id: entityId })}`),
  createNote: (input: {
    entity_type: 'offer' | 'company'
    entity_id: number
    body: string
    pinned_event_id?: number | null
  }) => request<{ note: Note }>('/notes', body('POST', input)),
  updateNote: (id: number, patch: { body?: string; pinned_event_id?: number | null }) =>
    request<{ note: Note }>(`/notes/${id}`, body('PATCH', patch)),
  deleteNote: (id: number) => request<void>(`/notes/${id}`, { method: 'DELETE' }),

  // companies
  listCompanies: (query: { q?: string; limit?: number; offset?: number } = {}) =>
    request<CompanyList>(`/companies${queryString({ ...query })}`),
  getCompany: (id: number) => request<CompanyDetail>(`/companies/${id}`),
  createCompany: (input: { name: string; website?: string | null }) =>
    request<{ company: CompanyDetail['company'] }>('/companies', body('POST', input)),
  updateCompany: (id: number, patch: { name?: string; website?: string | null }) =>
    request<{ company: CompanyDetail['company'] }>(`/companies/${id}`, body('PATCH', patch)),
  deleteCompany: (id: number) => request<void>(`/companies/${id}`, { method: 'DELETE' }),
  mergeCompany: (id: number, intoId: number) =>
    request<{
      company: CompanyDetail['company']
      merged_company_id: number
      moved_offers: number
      moved_notes: number
    }>(`/companies/${id}/merge`, body('POST', { into_id: intoId })),
}

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return `${err.message} (${err.code})`
  if (err instanceof Error) return err.message
  return String(err)
}
