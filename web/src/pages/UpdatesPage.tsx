import { useState } from 'react'
import { api, type Update } from '../api'
import {
  CategoryIcon,
  ErrorBox,
  FreshnessPill,
  Loading,
  Markdown,
  PageHeader,
  RefreshButton,
  formatDate,
  useAsync,
} from '../components'

export default function UpdatesPage() {
  const { data, error, loading, reload } = useAsync((refresh) => api.updates(refresh))
  const [showAll, setShowAll] = useState(false)
  if (error) return <ErrorBox error={error} />
  if (!data) return <Loading what="upstream releases" />

  const actionable = data.filter((u) => u.freshness !== 'latest' && u.freshness !== 'unknown')
  const shown = showAll ? data : actionable

  return (
    <div>
      <a className="small" href="#/components">
        ← Components
      </a>
      <PageHeader
        title="Component updates"
        subtitle={
          actionable.length === 0
            ? 'Every component is on its latest upstream release.'
            : `${actionable.length} of ${data.length} components have newer upstream releases.`
        }
        actions={
          <>
            <label className="toggle">
              <input type="checkbox" checked={showAll} onChange={(e) => setShowAll(e.target.checked)} />
              Show all components
            </label>
            <RefreshButton onClick={reload} loading={loading} />
          </>
        }
      />
      <div className="updates">
        {shown.map((u) => (
          <UpdateCard key={u.component} u={u} />
        ))}
      </div>
    </div>
  )
}

function UpdateCard({ u }: { u: Update }) {
  const [open, setOpen] = useState(false)
  return (
    <article className={`card update update-${u.freshness}`}>
      <div className="update-head">
        <CategoryIcon category={u.category} />
        <div className="update-title">
          <h3>{u.displayName}</h3>
          <div className="version-jump">
            <span className="mono">{u.current ?? '?'}</span>
            {u.latest && u.latest.version !== u.current && (
              <>
                <span className="muted">→</span>
                <a className="mono" href={u.latest.url} target="_blank" rel="noreferrer noopener">
                  {u.latest.version}
                </a>
              </>
            )}
          </div>
        </div>
        <FreshnessPill value={u.freshness} />
      </div>

      <div className="chips">
        {Object.entries(u.versions)
          .sort()
          .map(([env, v]) => (
            <span key={env} className="chip">
              {env}
              <span className="chip-version">{v}</span>
            </span>
          ))}
      </div>

      <ul className="reasons">
        {u.reasons.map((r) => (
          <li key={r}>{r}</li>
        ))}
      </ul>

      <div className="update-links small">
        {u.repository && (
          <a href={u.repository + '/releases'} target="_blank" rel="noreferrer noopener">
            Releases ↗
          </a>
        )}
        {u.homepage && (
          <a href={u.homepage} target="_blank" rel="noreferrer noopener">
            Homepage ↗
          </a>
        )}
        {u.newer && u.newer.length > 0 && (
          <button className="link-btn" onClick={() => setOpen(!open)}>
            {open ? 'Hide' : 'Show'} {u.newer.length} newer release{u.newer.length > 1 ? 's' : ''}
          </button>
        )}
      </div>

      {open && (
        <div className="newer">
          {u.newer?.map((r) => (
            <details key={r.tag}>
              <summary>
                <span className="mono">{r.version}</span>
                <span className="muted small"> · {formatDate(r.publishedAt)}</span>
                <a href={r.url} target="_blank" rel="noreferrer noopener" className="small">
                  {' '}
                  GitHub ↗
                </a>
              </summary>
              <Markdown>{r.notes || '_No release notes._'}</Markdown>
            </details>
          ))}
        </div>
      )}
    </article>
  )
}
