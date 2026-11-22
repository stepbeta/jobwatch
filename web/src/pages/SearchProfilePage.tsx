import { useEffect, useState, type FormEvent } from 'react'
import { api, errorMessage } from '../api'
import { BUCKET_LABELS, remoteLabel } from '../format'
import type {
  BucketScale,
  RemotePolicy,
  SearchProfile,
  SearchProfileInput,
  TermAffinity,
  TermBucket,
} from '../types'

const THREE_BUCKETS: TermBucket[] = ['positive', 'neutral', 'negative']
const FIVE_BUCKETS: TermBucket[] = [
  'strong_positive',
  'mild_positive',
  'neutral',
  'mild_negative',
  'strong_negative',
]
const REMOTE_POLICIES_UI: RemotePolicy[] = ['remote', 'hybrid', 'onsite', 'unknown']

// profileBuckets is the bucket list offered for a scale. The editor uses
// the profile's buckets; the stored weight stays authoritative.
export function profileBuckets(scale: BucketScale): TermBucket[] {
  return scale === 'three' ? THREE_BUCKETS : FIVE_BUCKETS
}

// mapBucket converts a bucket when the user switches scale. Positive and
// negative collapse into the three-bucket values and expand to the mild
// variants on the way back.
export function mapBucket(bucket: TermBucket, to: BucketScale): TermBucket {
  if (to === 'five') {
    if (bucket === 'positive') return 'mild_positive'
    if (bucket === 'negative') return 'mild_negative'
    return bucket
  }
  if (bucket === 'strong_positive' || bucket === 'mild_positive') return 'positive'
  if (bucket === 'strong_negative' || bucket === 'mild_negative') return 'negative'
  return bucket
}

// bucketWeight is the default numeric weight of a bucket. Title and
// location/description use separate scales, matching the seeded profile.
export function bucketWeight(affinity: TermAffinity, bucket: TermBucket): number {
  const title = affinity === 'title'
  switch (bucket) {
    case 'strong_positive':
      return title ? 60 : 35
    case 'mild_positive':
      return title ? 25 : 20
    case 'positive':
      return title ? 60 : 35
    case 'neutral':
      return 0
    case 'mild_negative':
      return -20
    case 'strong_negative':
      return title ? -120 : -35
    case 'negative':
      return title ? -120 : -35
  }
}

interface EditableTerm {
  key: number
  affinity: TermAffinity
  bucket: TermBucket
  term: string
  weight: number
}

let termSeq = 0

function editableTerms(p: SearchProfile): EditableTerm[] {
  return p.terms.map((t) => ({
    key: ++termSeq,
    affinity: t.affinity,
    bucket: t.bucket,
    term: t.term,
    weight: t.weight,
  }))
}

function remoteMap(p: SearchProfile): Record<RemotePolicy, boolean> {
  const out: Record<RemotePolicy, boolean> = {
    remote: false,
    hybrid: false,
    onsite: false,
    unknown: false,
  }
  for (const r of p.remote) out[r.policy] = r.accepted
  return out
}

export function SearchProfilePage({
  onNotice,
}: {
  onNotice: (kind: 'info' | 'success' | 'error', text: string) => void
}) {
  const [profile, setProfile] = useState<SearchProfile | null>(null)
  const [name, setName] = useState('')
  const [titleLabel, setTitleLabel] = useState('')
  const [locationLabel, setLocationLabel] = useState('')
  const [scale, setScale] = useState<BucketScale>('five')
  const [terms, setTerms] = useState<EditableTerm[]>([])
  const [remote, setRemote] = useState<Record<RemotePolicy, boolean>>({
    remote: true,
    hybrid: false,
    onsite: false,
    unknown: true,
  })
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    api
      .listSearchProfiles()
      .then((r) => {
        if (cancelled) return
        const def = r.profiles.find((p) => p.is_default) ?? r.profiles[0]
        if (!def) {
          setError('No search profile exists. Run the database migration first.')
          return
        }
        setProfile(def)
        setName(def.name)
        setTitleLabel(def.title_label)
        setLocationLabel(def.location_label)
        setScale(def.bucket_scale)
        setTerms(editableTerms(def))
        setRemote(remoteMap(def))
      })
      .catch((e) => {
        if (!cancelled) setError(errorMessage(e))
      })
    return () => {
      cancelled = true
    }
  }, [])

  function changeScale(next: BucketScale) {
    setScale(next)
    setTerms((list) => list.map((t) => ({ ...t, bucket: mapBucket(t.bucket, next) })))
  }

  function setTerm(key: number, patch: Partial<EditableTerm>) {
    setTerms((list) => list.map((t) => (t.key === key ? { ...t, ...patch } : t)))
  }

  function changeBucket(term: EditableTerm, bucket: TermBucket) {
    setTerm(term.key, { bucket, weight: bucketWeight(term.affinity, bucket) })
  }

  function addTerm(affinity: TermAffinity) {
    const bucket = affinity === 'title' ? 'strong_positive' : 'mild_positive'
    setTerms((list) => [
      ...list,
      { key: ++termSeq, affinity, bucket, term: '', weight: bucketWeight(affinity, bucket) },
    ])
  }

  function removeTerm(key: number) {
    setTerms((list) => list.filter((t) => t.key !== key))
  }

  async function save(e: FormEvent) {
    e.preventDefault()
    if (!profile) return
    setError('')
    if (name.trim() === '') {
      setError('Profile name is required.')
      return
    }
    if (terms.some((t) => t.term.trim() === '')) {
      setError('Every term needs a value; remove empty rows or fill them in.')
      return
    }
    const input: SearchProfileInput = {
      name: name.trim(),
      title_label: titleLabel.trim(),
      location_label: locationLabel.trim(),
      bucket_scale: scale,
      terms: terms.map((t) => ({
        affinity: t.affinity,
        bucket: t.bucket,
        term: t.term.trim(),
        weight: t.weight,
      })),
      remote: REMOTE_POLICIES_UI.map((policy) => ({ policy, accepted: remote[policy] })),
    }
    setSaving(true)
    try {
      const r = await api.replaceSearchProfile(profile.id, input)
      setProfile(r.profile)
      setName(r.profile.name)
      setTerms(editableTerms(r.profile))
      setRemote(remoteMap(r.profile))
      onNotice('success', 'Search profile saved. Offer affinities update on the next view.')
    } catch (err) {
      const text = errorMessage(err)
      setError(text)
      onNotice('error', text)
    } finally {
      setSaving(false)
    }
  }

  if (!profile && !error) {
    return (
      <div className="page">
        <p className="muted" aria-busy="true">
          Loading search profile…
        </p>
      </div>
    )
  }

  const buckets = profileBuckets(scale)
  const section = (affinity: TermAffinity, explanation: string) => {
    const list = terms.filter((t) => t.affinity === affinity)
    return (
      <fieldset className="profile-section">
        <legend>
          {affinity === 'title'
            ? 'Title terms'
            : affinity === 'location'
              ? 'Location terms'
              : 'Description terms'}
        </legend>
        <p className="hint">{explanation}</p>
        <table className="grid profile-terms">
          <thead>
            <tr>
              <th>Term</th>
              <th>Bucket</th>
              <th>Weight</th>
              <th aria-label="Remove" />
            </tr>
          </thead>
          <tbody>
            {list.map((t) => (
              <tr key={t.key}>
                <td>
                  <input
                    aria-label={`${affinity} term`}
                    value={t.term}
                    onChange={(e) => setTerm(t.key, { term: e.target.value })}
                  />
                </td>
                <td>
                  <select
                    aria-label={`${affinity} bucket for ${t.term || 'new term'}`}
                    value={t.bucket}
                    onChange={(e) => changeBucket(t, e.target.value as TermBucket)}
                  >
                    {buckets.map((b) => (
                      <option key={b} value={b}>
                        {BUCKET_LABELS[b]}
                      </option>
                    ))}
                  </select>
                </td>
                <td>
                  <input
                    type="number"
                    aria-label={`${affinity} weight for ${t.term || 'new term'}`}
                    value={t.weight}
                    onChange={(e) => setTerm(t.key, { weight: Number(e.target.value) })}
                  />
                </td>
                <td>
                  <button type="button" onClick={() => removeTerm(t.key)}>
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <button type="button" onClick={() => addTerm(affinity)}>
          Add {affinity} term
        </button>
      </fieldset>
    )
  }

  return (
    <div className="page">
      <section className="panel">
        <div className="section-head">
          <h2>Search profile</h2>
          <span className="muted">
            Affinity is computed from this profile on every read; no reclassification is needed.
          </span>
        </div>
        {error && <div className="error-box">{error}</div>}
        <form className="profile-form" onSubmit={(e) => void save(e)}>
          <div className="filters">
            <label>
              Name
              <input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              Title label
              <input value={titleLabel} onChange={(e) => setTitleLabel(e.target.value)} />
            </label>
            <label>
              Location label
              <input value={locationLabel} onChange={(e) => setLocationLabel(e.target.value)} />
            </label>
            <label>
              Bucket scale
              <select value={scale} onChange={(e) => changeScale(e.target.value as BucketScale)}>
                <option value="five">5 buckets</option>
                <option value="three">3 buckets</option>
              </select>
            </label>
          </div>
          <p className="hint">
            Buckets are editor labels. Choosing one sets a default weight; the numeric weight next
            to each term is the value used for scoring, so advanced users can fine-tune it.
          </p>

          {section(
            'title',
            'Positive title terms raise title affinity, negative terms lower it. Neutral terms only appear in the reasons.',
          )}
          {section(
            'location',
            'Location terms match the offer location. Positive terms add evidence for the target geography, negative terms subtract it.',
          )}

          <details className="profile-advanced">
            <summary>Advanced: description terms</summary>
            <p className="hint">
              Description terms add weaker location evidence only. They never change the detected
              remote policy.
            </p>
            {section(
              'description',
              'Description terms are matched against the offer description and add to location affinity. They do not affect remote detection.',
            )}
          </details>

          <fieldset className="profile-section">
            <legend>Remote policy</legend>
            <p className="hint">
              The policy is detected from the offer text. Check the policies you accept; this is a
              membership check, not a weighted score.
            </p>
            <div className="remote-choices">
              {REMOTE_POLICIES_UI.map((policy) => (
                <label key={policy} className="checkbox">
                  <input
                    type="checkbox"
                    checked={remote[policy]}
                    onChange={(e) => setRemote((r) => ({ ...r, [policy]: e.target.checked }))}
                  />
                  {remoteLabel(policy)}
                </label>
              ))}
            </div>
          </fieldset>

          <div className="form-actions">
            <button type="submit" className="primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save profile'}
            </button>
          </div>
        </form>
      </section>
    </div>
  )
}
