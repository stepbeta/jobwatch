import { useEffect, useState } from 'react'
import { api, errorMessage } from '../api'
import { Link } from '../components/Link'
import {
  ATTACHMENT_KINDS,
  attachmentKindLabel,
  fmtBytes,
  fmtDateTime,
  fmtPDFDate,
} from '../format'
import type { AttachmentKind, DocumentItem } from '../types'

// DocumentsPage is the read-only aggregate over stored files. Uploads,
// re-tagging and detaching stay on the offer page; this page only lists,
// downloads and links back to the offers that reference each file.
export function DocumentsPage() {
  const [documents, setDocuments] = useState<DocumentItem[] | null>(null)
  const [kindFilter, setKindFilter] = useState<AttachmentKind | 'all'>('all')
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    api
      .listDocuments()
      .then((res) => {
        if (!cancelled) setDocuments(res.documents)
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [])

  if (documents === null && !error) {
    return (
      <p className="muted page" aria-busy="true">
        Loading documents…
      </p>
    )
  }

  // A stored file carries no single kind: kinds live on the references, so a
  // filter selects files referenced at least once with that kind and its
  // count is a number of distinct files, not of attachments.
  const stored = documents ?? []
  const kinds = ATTACHMENT_KINDS.filter((kind) =>
    stored.some((d) => d.offers.some((ref) => ref.kind === kind)),
  )
  const activeFilter = kinds.includes(kindFilter as AttachmentKind)
    ? (kindFilter as AttachmentKind)
    : 'all'
  const visible =
    activeFilter === 'all'
      ? stored
      : stored.filter((d) => d.offers.some((ref) => ref.kind === activeFilter))
  const countFor = (kind: AttachmentKind) =>
    stored.filter((d) => d.offers.some((ref) => ref.kind === kind)).length

  return (
    <div className="page">
      <div className="section-head">
        <h1>Documents</h1>
        <span className="muted">
          {documents?.length ?? 0} file{(documents?.length ?? 0) === 1 ? '' : 's'}
        </span>
      </div>
      <p className="muted">
        Every stored file once, with the offers that reference it. Upload a CV or cover letter from
        the offer page. The PDF creation and modification dates come from the file; the first
        upload time is when JobWatch received these bytes. The browser's file modification time is
        never stored or shown.
      </p>

      {error && <div className="error-box">{error}</div>}

      {documents !== null && documents.length === 0 && (
        <div className="panel empty">No documents stored yet.</div>
      )}

      {documents !== null && kinds.length > 1 && (
        <div className="kind-filters" role="group" aria-label="Filter documents by kind">
          <button
            type="button"
            className="kind-filter"
            aria-pressed={activeFilter === 'all'}
            onClick={() => setKindFilter('all')}
          >
            All {stored.length}
          </button>
          {kinds.map((kind) => (
            <button
              key={kind}
              type="button"
              className="kind-filter"
              aria-pressed={activeFilter === kind}
              onClick={() => setKindFilter(kind)}
            >
              {attachmentKindLabel(kind)} {countFor(kind)}
            </button>
          ))}
        </div>
      )}

      {documents !== null && documents.length > 0 && (
        <div className="documents-grid">
          {visible.map((d) => (
            <article className="panel document-card" key={d.sha256}>
              <div className="section-head">
                <div>
                  <h2>{d.pdf.title ?? 'PDF document'}</h2>
                  <div className="muted small mono">{d.sha256.slice(0, 16)}…</div>
                </div>
                <a href={d.download_url}>Download</a>
              </div>

              <div className="document-dates small">
                <span>Size: {fmtBytes(d.size)}</span>
                <span>Pages: {d.pdf.page_count ?? 'Unknown'}</span>
                <span>PDF version: {d.pdf.version ?? 'Unknown'}</span>
                <span>Author: {d.pdf.author ?? 'Unknown'}</span>
                <span>Producer: {d.pdf.producer ?? 'Unknown'}</span>
                <span>PDF creation: {fmtPDFDate(d.pdf.creation_date)}</span>
                <span>PDF modified: {fmtPDFDate(d.pdf.modification_date)}</span>
                <span>First uploaded: {fmtDateTime(d.first_uploaded_at)}</span>
              </div>

              <div className="document-offers">
                <h3>Referenced by</h3>
                <ul>
                  {d.offers.map((ref) => (
                    <li key={ref.attachment_id}>
                      <Link to={`/offers/${ref.offer_id}`}>
                        {ref.offer_title || 'Untitled offer'}
                      </Link>
                      <span className="muted"> — {ref.company_name || 'No company'}</span>
                      <span className={`kind-badge kind-${ref.kind}`}>
                        {attachmentKindLabel(ref.kind)}
                      </span>
                      <span className="muted small">Attached: {fmtDateTime(ref.attached_at)}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}
