import { useState } from 'react'
import { api, type EnvStatus, type Offering, type PlatformResponse, type Row } from '../api'
import {
  CategoryIcon,
  ErrorBox,
  Loading,
  OfferingStatus,
  PageHeader,
  Pill,
  groupBy,
  humanize,
  statusInfo,
  useAsync,
} from '../components'

type Availability = { tone: string; label: string }

const installed = (row: Row | undefined, env: string) => {
  const s = row?.cells[env]?.status
  return s !== undefined && s !== 'absent' && s !== 'missing'
}

function availability(o: Offering, env: string, rows: Map<string, Row>): Availability {
  if (o.status === 'planned') return { tone: 'muted', label: 'Planned' }
  const comps = o.components ?? []
  if (comps.length === 0) return { tone: 'muted', label: 'No components declared' }
  const n = comps.filter((c) => installed(rows.get(c), env)).length
  if (n === comps.length) return { tone: 'good', label: 'Available' }
  if (n === 0) return { tone: 'none', label: 'Not available' }
  return { tone: 'warn', label: `Partially available (${n}/${comps.length} components)` }
}

export default function OfferingsPage({ platform }: { platform?: PlatformResponse }) {
  const matrix = useAsync(() => api.matrix())
  const [open, setOpen] = useState<Set<string>>(() => new Set())
  if (!platform) return <Loading what="offerings" />

  const offerings = platform.platform.spec.offerings ?? []
  const rows = new Map((matrix.data?.rows ?? []).map((r) => [r.component, r]))
  const envs = matrix.data?.environments ?? []
  const release = platform.currentRelease
  const groups = groupBy(offerings, (o) => o.category ?? 'other')
  const toggle = (key: string) =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })

  return (
    <div>
      <PageHeader
        title="Offerings"
        subtitle="What application teams get from the platform, grouped by capability, and where each offering is available."
        actions={
          <>
            <button className="btn" onClick={() => setOpen(new Set(groups.map(([c]) => 'cat:' + c)))}>
              Expand all
            </button>
            <button className="btn" onClick={() => setOpen(new Set())}>
              Collapse all
            </button>
          </>
        }
      />
      {matrix.error && <ErrorBox error={matrix.error} />}
      {offerings.length === 0 && (
        <div className="card empty-state">
          <h3>No offerings defined</h3>
          <p>
            Add <code>spec.offerings</code> to the platform file to describe what the platform provides.
          </p>
        </div>
      )}

      <div className="tree">
        {groups.map(([cat, items]) => {
          const expanded = open.has('cat:' + cat)
          const live = items.filter((o) => o.status !== 'planned')
          return (
            <section key={cat} className={`card cat-group ${expanded ? 'open' : ''}`}>
              <button className="cat-header" onClick={() => toggle('cat:' + cat)} aria-expanded={expanded}>
                <span className="caret">{expanded ? '▾' : '▸'}</span>
                <CategoryIcon category={cat} />
                <span className="cat-title">
                  <strong>{humanize(cat)}</strong>
                </span>
                <span className="cat-summary">
                  {!expanded &&
                    items.map((o) => (
                      <span key={o.name} className={`chip ${o.status === 'planned' ? 'chip-planned' : ''}`}>
                        {o.displayName ?? o.name}
                      </span>
                    ))}
                </span>
                <span className="muted small cat-count">
                  {live.length} offering{live.length === 1 ? '' : 's'}
                  {items.length > live.length && ` · ${items.length - live.length} planned`}
                </span>
              </button>
              {expanded && (
                <div className="cat-body">
                  {items.map((o) => (
                    <OfferingNode
                      key={o.name}
                      o={o}
                      envs={envs}
                      rows={rows}
                      versions={release?.spec.components ?? {}}
                      expanded={open.has('off:' + o.name)}
                      onToggle={() => toggle('off:' + o.name)}
                    />
                  ))}
                </div>
              )}
            </section>
          )
        })}
      </div>
    </div>
  )
}

function OfferingNode({
  o,
  envs,
  rows,
  versions,
  expanded,
  onToggle,
}: {
  o: Offering
  envs: EnvStatus[]
  rows: Map<string, Row>
  versions: Record<string, string>
  expanded: boolean
  onToggle: () => void
}) {
  const comps = o.components ?? []
  return (
    <div className="comp-node">
      <button className={`comp-row offering-row ${o.status === 'planned' ? 'unused' : ''}`} onClick={onToggle} aria-expanded={expanded}>
        <span className="caret">{expanded ? '▾' : '▸'}</span>
        <span className="comp-label">
          <span className="comp-name">{o.displayName ?? o.name}</span>
          {o.description && <span className="muted small comp-desc">{o.description}</span>}
        </span>
        <span className="env-dots">
          {o.status !== 'planned' &&
            envs.map((e) => {
              const a = availability(o, e.name, rows)
              return (
                <span key={e.name} className={`env-dot cell-${a.tone}`} title={`${e.displayName ?? e.name}: ${a.label}`}>
                  <span className="cell-dot" />
                  {e.name}
                </span>
              )
            })}
        </span>
        <span className="comp-status">
          <OfferingStatus status={o.status} />
        </span>
      </button>
      {expanded && (
        <div className="comp-detail">
          {o.description && <p className="offering-desc">{o.description}</p>}
          {comps.length > 0 ? (
            <>
              <h4>Powered by</h4>
              <table className="bom compact">
                <thead>
                  <tr>
                    <th>Component</th>
                    <th>Release</th>
                    {envs.map((e) => (
                      <th key={e.name}>{e.displayName ?? e.name}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {comps.map((c) => {
                    const row = rows.get(c)
                    return (
                      <tr key={c}>
                        <td>{row?.displayName ?? c}</td>
                        <td className="mono">{versions[c] ?? '–'}</td>
                        {envs.map((e) => {
                          const cell = row?.cells[e.name]
                          const s = statusInfo[cell?.status ?? 'absent']
                          return (
                            <td key={e.name} title={s.label}>
                              <span className={`cell cell-${s.tone}`}>
                                <span className="cell-dot" />
                                <span className="mono">{cell?.version || '–'}</span>
                              </span>
                            </td>
                          )
                        })}
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </>
          ) : (
            <p className="muted">No components declared yet.</p>
          )}
          <div className="detail-links small">
            {o.docs && (
              <a href={o.docs} target="_blank" rel="noreferrer noopener">
                Documentation ↗
              </a>
            )}
            {o.status === 'planned' && <Pill tone="muted">On the roadmap</Pill>}
          </div>
        </div>
      )}
    </div>
  )
}
