import { useEffect, useState } from 'react'
import { api, type ComponentChange, type PlatformRelease, type PlatformResponse } from '../api'
import { ErrorBox, Loading, Markdown, PageHeader, Pill, formatDate, useAsync } from '../components'

type Label = (name: string) => string

const changeTone: Record<ComponentChange['change'], string> = {
  added: 'good',
  removed: 'bad',
  upgraded: 'info',
  downgraded: 'warn',
  unchanged: 'muted',
}

export default function ReleasesPage({ selected, platform }: { selected?: string; platform?: PlatformResponse }) {
  const { data: releases, error } = useAsync(() => api.releases())
  const catalog = useAsync(() => api.catalog())
  const compNames = new Map((catalog.data ?? []).map((c) => [c.metadata.name, c.spec.displayName ?? c.metadata.name]))
  const offNames = new Map((platform?.platform.spec.offerings ?? []).map((o) => [o.name, o.displayName ?? o.name]))
  const compLabel: Label = (n) => compNames.get(n) ?? n
  const offLabel: Label = (n) => offNames.get(n) ?? n
  if (error) return <ErrorBox error={error} />
  if (!releases) return <Loading what="releases" />
  if (releases.length === 0) {
    return (
      <div>
        <PageHeader title="Releases" />
        <div className="card empty-state">
          <h3>No platform releases yet</h3>
          <p>Cut your first release from a live environment:</p>
          <pre>pbom release create v1.0.0 --from-env prod --summary "First platform release"</pre>
        </div>
      </div>
    )
  }
  const idx = Math.max(
    0,
    releases.findIndex((r) => r.metadata.name === selected),
  )
  const current = releases[idx]
  const previous = releases[idx + 1]

  return (
    <div>
      <PageHeader
        title="Releases"
        subtitle="Every platform release is a versioned bundle of components and offerings."
      />
      <div className="releases-layout">
        <aside className="timeline">
          {releases.map((r, i) => (
            <a
              key={r.metadata.name}
              href={`#/releases/${r.metadata.name}`}
              className={`timeline-item ${r === current ? 'active' : ''}`}
            >
              <span className="timeline-dot" />
              <div>
                <div className="timeline-version">
                  v{r.spec.version} {i === 0 && <Pill tone="good">Current</Pill>}
                </div>
                <div className="small muted">{formatDate(r.spec.date)}</div>
                {r.spec.summary && <div className="small">{r.spec.summary}</div>}
              </div>
            </a>
          ))}
        </aside>
        <ReleaseDetail
          release={current}
          previous={previous}
          releases={releases}
          compLabel={compLabel}
          offLabel={offLabel}
        />
      </div>
    </div>
  )
}

function ReleaseDetail({
  release,
  previous,
  releases,
  compLabel,
  offLabel,
}: {
  release: PlatformRelease
  previous?: PlatformRelease
  releases: PlatformRelease[]
  compLabel: Label
  offLabel: Label
}) {
  const [base, setBase] = useState(previous?.metadata.name ?? '')
  useEffect(() => setBase(previous?.metadata.name ?? ''), [previous, release])
  const diff = useAsync(
    () => (base ? api.diff(base, release.metadata.name) : Promise.resolve(undefined)),
    [base, release.metadata.name],
  )
  const changes = diff.data?.components.filter((c) => c.change !== 'unchanged') ?? []

  return (
    <div className="release-detail">
      <section className="card">
        <div className="row-between">
          <h2>Platform v{release.spec.version}</h2>
          <span className="muted">{formatDate(release.spec.date)}</span>
        </div>
        {release.spec.summary && <p className="lead">{release.spec.summary}</p>}
        {release.spec.highlights && release.spec.highlights.length > 0 && (
          <>
            <h3>What's new</h3>
            <ul className="highlights">
              {release.spec.highlights.map((h) => (
                <li key={h}>{h}</li>
              ))}
            </ul>
          </>
        )}
        <Markdown>{release.spec.notes}</Markdown>
      </section>

      <section className="card">
        <div className="row-between">
          <h3>Changes</h3>
          <label className="small">
            compared with{' '}
            <select value={base} onChange={(e) => setBase(e.target.value)}>
              <option value="">—</option>
              {releases
                .filter((r) => r.metadata.name !== release.metadata.name)
                .map((r) => (
                  <option key={r.metadata.name} value={r.metadata.name}>
                    v{r.spec.version}
                  </option>
                ))}
            </select>
          </label>
        </div>
        {diff.error && <ErrorBox error={diff.error} />}
        {!base && <p className="muted">First release — nothing to compare with.</p>}
        {base && diff.data && (
          <>
            {changes.length === 0 && <p className="muted">No component changes.</p>}
            <table className="bom">
              <tbody>
                {changes.map((c) => (
                  <tr key={c.component}>
                    <td>{c.displayName}</td>
                    <td className="mono muted right">{c.from ?? ''}</td>
                    <td className="arrow-col">→</td>
                    <td className="mono">{c.to ?? ''}</td>
                    <td className="right">
                      <Pill tone={changeTone[c.change]}>{c.change}</Pill>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {(diff.data.offeringsAdded?.length ?? 0) > 0 && (
              <p>
                New offerings: <strong>{diff.data.offeringsAdded!.map(offLabel).join(', ')}</strong>
              </p>
            )}
            {(diff.data.offeringsRemoved?.length ?? 0) > 0 && (
              <p>
                Removed offerings: <strong>{diff.data.offeringsRemoved!.map(offLabel).join(', ')}</strong>
              </p>
            )}
          </>
        )}
      </section>

      <section className="card">
        <h3>Components</h3>
        <table className="bom">
          <tbody>
            {Object.entries(release.spec.components)
              .sort(([a], [b]) => compLabel(a).localeCompare(compLabel(b)))
              .map(([name, v]) => (
                <tr key={name}>
                  <td>{compLabel(name)}</td>
                  <td className="mono right">{v}</td>
                </tr>
              ))}
          </tbody>
        </table>
        {release.spec.offerings && release.spec.offerings.length > 0 && (
          <>
            <h3>Offerings</h3>
            <div className="chips">
              {release.spec.offerings.map((o) => (
                <span key={o} className="chip">
                  {offLabel(o)}
                </span>
              ))}
            </div>
          </>
        )}
      </section>
    </div>
  )
}
