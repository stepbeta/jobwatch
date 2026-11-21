export type SourceType =
  | 'greenhouse'
  | 'lever'
  | 'ashby'
  | 'personio'
  | 'workable'
  | 'bamboohr'
  | 'wttj'
  | 'rss'
  | 'html'
  | 'manual'

export type ScanStatus = 'ok' | 'error' | 'blocked'
export type OfferState = 'active' | 'disappeared'
export type ApplicationState =
  | 'todo'
  | 'applied'
  | 'responded'
  | 'interview'
  | 'offer'
  | 'negative'
  | 'maybe'
  | 'positive'
export type RemotePolicy = 'remote' | 'hybrid' | 'onsite' | 'unknown'
export type EventType =
  | 'cv_sent'
  | 'message_received'
  | 'message_sent'
  | 'interview'
  | 'offer'
  | 'rejected'
  | 'ghosted'
  | 'other'
export type EntityType = 'offer' | 'company'

export interface Source {
  id: number
  name: string
  url: string
  source_type: SourceType
  config: Record<string, unknown>
  enabled: boolean
  last_scan_at: string | null
  last_scan_status: ScanStatus | null
  created_at: string | null
}

export interface Company {
  id: number
  name: string
  name_normalized: string
  website: string | null
  created_at: string | null
  offer_count: number
  note_count: number
}

export interface CompanyCandidate {
  id: number
  name: string
  name_normalized: string
  score: number
  reason: string
}

export interface Offer {
  id: number
  company_id: number | null
  company_name: string
  title: string
  location: string | null
  remote_policy: RemotePolicy
  url: string | null
  offer_state: OfferState
  application_state: ApplicationState
  first_seen_at: string
  last_seen_at: string
  updated_at: string
  title_confidence: number | null
  location_confidence: number | null
  match_reasons: MatchReasons | null
  last_event_at: string | null
  saved_at: string | null
  raw_text?: string | null
  incomplete_reason?: string
  incomplete_raw?: IncompleteRaw | null
}

export interface IncompleteRaw {
  company?: string
  company_key?: string
  title?: string
  location?: string
  url?: string
  external_uid?: string
}

export interface MatchReasons {
  title?: string[]
  location?: string[]
}

export interface Provenance {
  id: number
  source_id: number | null
  source_name: string | null
  external_uid: string | null
  url: string
  first_seen_at: string | null
  last_seen_at: string | null
}

export interface Event {
  id: number
  offer_id: number
  type: EventType
  description: string | null
  occurred_at: string
  created_at: string | null
}

export interface Note {
  id: number
  entity_type: EntityType
  entity_id: number
  body: string
  pinned_event_id: number | null
  created_at: string | null
  updated_at: string | null
}

export interface Snapshot {
  id: number
  origin: SnapshotOrigin
  raw_text: string
  captured_at: string
}

export type SnapshotOrigin = 'scan' | 'manual'

export type AnswerFormat = 'markdown'

export interface InterviewEntry {
  id: number
  question: string
  answer: string
  answer_format: AnswerFormat
  category: string | null
  display_order: number | null
  created_at: string | null
  updated_at: string | null
}

export interface SnapshotSummary {
  id: number
  origin: SnapshotOrigin
  captured_at: string
  text_length: number
  raw_text?: string
}

export interface SnapshotRef {
  id: number
  captured_at: string
}

export interface DiffLine {
  op: 'equal' | 'added' | 'removed'
  text: string
}

export interface SnapshotDiff {
  from: SnapshotRef
  to: SnapshotRef
  lines: DiffLine[]
}

export interface OfferDetail {
  offer: Offer
  company: Company | null
  provenance: Provenance[]
  events: Event[]
  notes: Note[]
  snapshots: SnapshotSummary[]
  latest_snapshot: Snapshot | null
  snapshot_diff: SnapshotDiff | null
  snapshot_count: number
}

export interface CompanyDetail {
  company: Company
  offers: Offer[]
  notes: Note[]
  possible_duplicates: CompanyCandidate[]
  keys: string[]
}

export interface OfferList {
  offers: Offer[]
  total: number
  limit: number
  offset: number
}

export interface CompanyList {
  companies: Company[]
  total: number
  limit: number
  offset: number
}

export interface ScanResult {
  source_id: number
  source_name: string
  status: ScanStatus | 'skipped'
  fetched: number
  skipped: number
  new: number
  seen: number
  reappeared: number
  disappeared: number
  incomplete: number
  error?: string
}

export interface ScanResponse {
  result: ScanResult
  source: Source
}
