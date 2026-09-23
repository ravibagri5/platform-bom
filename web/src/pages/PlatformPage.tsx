import { api, type Matrix, type PlatformResponse } from '../api'
import {
  CategoryIcon,
  ErrorBox,
  Loading,
  Markdown,
  OfferingStatus,
  Percent,
  Pill,
  formatDate,
  groupBy,
  humanize,
  useAsync,
  vname,
} from '../components'

interface Async<T> {
  data?: T
  error?: string
  loading: boolean
}

export default function PlatformPage({ platform }: { platform: Async<PlatformResponse> }) {
  const matrix = useAsync(() => api.matrix())
  if (platform.error) return <ErrorBox error={platform.error} />
  if (!platform.data) return <Loading what="platform" />

  const { platform: p, readme, currentRelease } = platform.data
  const m = matrix.data
  const displayName = (name: string) => m?.rows.find((r) => r.component === name)?.displayName ?? name

  return (
    <div className="platform-page">
      <section className="hero">
        <div className="hero-inner">
          {currentRelease && (
            <a className="release-badge" href={`#/releases/${currentRelease.metadata.name}`}>
              <span className="dot" /> Platform v{currentRelease.spec.version}
              {currentRelease.spec.date && <span className="muted"> · {formatDate(currentRelease.spec.date)}</span>}
            </a>
          )}
          <h1>{p.metadata.displayName ?? p.metadata.name}</h1>
          {p.spec.tagline && <p className="tagline">{p.spec.tagline}</p>}
          {p.spec.description && <p className="lead">{p.spec.description}</p>}
          <div className="hero-meta">
            {p.spec.owners?.map((o) => (
              <span key={o.name} className="owner">
                Owned by <strong>{o.name}</strong>
                {o.channel && <span className="muted"> · {o.channel}</span>}
              </span>
            ))}
            {p.spec.links?.map((l) => (
              <a key={l.url} href={l.url} target="_blank" rel="noreferrer noopener" className="hero-link">
                {l.title} ↗
              </a>
            ))}
          </div>
        </div>
      </section>

      {m && <HealthStrip m={m} />}
      {matrix.error && <ErrorBox error={matrix.error} />}

      {p.spec.offerings && p.spec.offerings.length > 0 && (
        <section className="section">
          <div className="row-between">
            <h2>What the platform provides</h2>
            <a href="#/offerings">Explore offerings →</a>
          </div>
          <p className="section-lead">Capabilities offered to application teams.</p>
          <div className="offering-groups">
            {groupBy(p.spec.offerings, (o) => o.category ?? 'other').map(([cat, items]) => (
              <a key={cat} href="#/offerings" className="card offering-group">
                <div className="offering-head">
                  <CategoryIcon category={cat} />
                  <h3>{humanize(cat)}</h3>
                </div>
                <ul>
                  {items.map((o) => (
                    <li key={o.name} className={o.status === 'planned' ? 'planned' : ''}>
                      <span>{o.displayName ?? o.name}</span>
                      {o.status && o.status !== 'ga' && <OfferingStatus status={o.status} />}
                    </li>
                  ))}
                </ul>
              </a>
            ))}
          </div>
        </section>
      )}

      <div className="two-col">
        {p.spec.guarantees && p.spec.guarantees.length > 0 && (
          <section className="section card">
            <h2>Platform guarantees</h2>
            <ul className="checklist">
              {p.spec.guarantees.map((g) => (
                <li key={g}>{g}</li>
              ))}
            </ul>
          </section>
        )}
        {currentRelease && (
          <section className="section card">
            <div className="row-between">
              <h2>Current release · v{currentRelease.spec.version}</h2>
              <a href="#/releases">All releases →</a>
            </div>
            {currentRelease.spec.summary && <p>{currentRelease.spec.summary}</p>}
            {currentRelease.spec.highlights && (
              <ul className="highlights">
                {currentRelease.spec.highlights.map((h) => (
                  <li key={h}>{h}</li>
                ))}
              </ul>
            )}
            <table className="bom compact">
              <tbody>
                {Object.entries(currentRelease.spec.components)
                  .sort(([a], [b]) => displayName(a).localeCompare(displayName(b)))
                  .map(([name, v]) => (
                    <tr key={name}>
                      <td>{displayName(name)}</td>
                      <td className="mono right">{v}</td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </section>
        )}
      </div>

      {m && m.environments.length > 0 && (
        <section className="section">
          <h2>Where it runs</h2>
          <div className="envs">
            {m.environments.map((e) => (
              <a key={e.name} href="#/environments" className="card env-card">
                <div className="row-between">
                  <h3>{e.displayName ?? e.name}</h3>
                  {e.tier && <span className="muted small">{e.tier}</span>}
                </div>
                <div className="env-release">
                  {e.matchedRelease ? vname(e.matchedRelease) : 'No release matched'}
                </div>
                <div className="small muted">
                  Kubernetes {e.cluster.version ?? '–'} {e.cluster.distribution?.toUpperCase()}
                </div>
                <div className="env-status">
                  {e.errors && e.errors.length > 0 && !e.cluster.version ? (
                    <Pill tone="bad">Unreachable</Pill>
                  ) : !e.targetRelease ? (
                    <Pill tone="muted">No target</Pill>
                  ) : e.compliant ? (
                    <Pill tone="good">On target {vname(e.targetRelease)}</Pill>
                  ) : (
                    <Pill tone="warn">
                      {e.drift + e.missing} off target {vname(e.targetRelease)}
                    </Pill>
                  )}
                </div>
              </a>
            ))}
          </div>
        </section>
      )}

      {readme && (
        <section className="section card readme">
          <Markdown>{readme}</Markdown>
        </section>
      )}
    </div>
  )
}

function HealthStrip({ m }: { m: Matrix }) {
  const s = m.summary
  return (
    <section className="stats">
      <a href="#/components" className="stat">
        <div className="stat-value">{s.components}</div>
        <div className="stat-label">Components</div>
      </a>
      <a href="#/offerings" className="stat">
        <div className="stat-value">{s.offerings}</div>
        <div className="stat-label">Offerings</div>
      </a>
      <a href="#/environments" className="stat">
        <div className="stat-value">{s.environments}</div>
        <div className="stat-label">Environments</div>
      </a>
      <a href="#/environments" className={`stat ${s.alignment >= 90 ? 'good' : s.alignment >= 0 ? 'warn' : ''}`}>
        <div className="stat-value">
          <Percent value={s.alignment} />
        </div>
        <div className="stat-label">Aligned with target release</div>
      </a>
      <a href="#/updates" className={`stat ${s.unsupported > 0 ? 'bad' : s.behind > 0 ? 'warn' : 'good'}`}>
        <div className="stat-value">
          <Percent value={s.currency} />
        </div>
        <div className="stat-label">
          Current with upstream{s.behind > 0 && ` · ${s.behind} behind`}
        </div>
      </a>
    </section>
  )
}
