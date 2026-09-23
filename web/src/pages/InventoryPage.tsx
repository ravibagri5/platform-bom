import { Fragment, useMemo, useState } from 'react'
import { api, type Cell, type Row } from '../api'
import {
  CategoryIcon,
  ErrorBox,
  FreshnessPill,
  Loading,
  PageHeader,
  Pill,
  KindBadge,
  RefreshButton,
  categoryName,
  formatDate,
  freshnessInfo,
  kindInfo,
  statusInfo,
  titleCase,
  useAsync,
  vname,
} from '../components'

const isDrift = (c?: Cell) => c?.status === 'drift' || c?.status === 'missing'
const isBehind = (r: Row) => r.freshness !== 'latest' && r.freshness !== 'unknown'
const hasIssue = (r: Row) => Object.values(r.cells).some(isDrift) || isBehind(r)

interface TNode {
  row: Row
  children: TNode[]
}

const flatten = (nodes: TNode[]): TNode[] => nodes.flatMap((n) => [n, ...flatten(n.children)])

function stats(nodes: TNode[], envs: string[]) {
  const s = { drift: 0, missing: 0, untracked: 0, behind: 0 }
  flatten(nodes).forEach(({ row }) => {
    envs.forEach((e) => {
      const st = row.cells[e]?.status
      if (st === 'drift') s.drift++
      if (st === 'missing') s.missing++
      if (st === 'untracked') s.untracked++
    })
    if (isBehind(row)) s.behind++
  })
  return s
}

function StatPills({ s }: { s: ReturnType<typeof stats> }) {
  return (
    <>
      {s.drift > 0 && <Pill tone="warn">{s.drift} drift</Pill>}
      {s.missing > 0 && <Pill tone="bad">{s.missing} missing</Pill>}
      {s.untracked > 0 && <Pill tone="violet">{s.untracked} untracked</Pill>}
      {s.behind > 0 && <Pill tone="info">{s.behind} behind upstream</Pill>}
    </>
  )
}

function childSummary(children: TNode[]) {
  const counts = new Map<string, number>()
  children.forEach((c) => counts.set(c.row.kind ?? 'api', (counts.get(c.row.kind ?? 'api') ?? 0) + 1))
  return [...counts.entries()]
    .map(([k, n]) => `${n} ${n === 1 ? (kindInfo[k]?.label ?? k).toLowerCase() : (kindInfo[k]?.plural ?? k)}`)
    .join(' · ')
}

export default function InventoryPage() {
  const { data: m, error, loading, reload } = useAsync((refresh) => api.matrix(refresh))
  const [issuesOnly, setIssuesOnly] = useState(false)
  const [query, setQuery] = useState('')
  const [showUnclassified, setShowUnclassified] = useState(true)
  const [closedCats, setClosedCats] = useState<Set<string>>(() => new Set())
  const [openParents, setOpenParents] = useState<Set<string>>(() => new Set())

  const groups = useMemo(() => {
    if (!m) return []
    const q = query.toLowerCase()
    const nodes = new Map<string, TNode>(m.rows.map((r) => [r.component, { row: r, children: [] }]))
    const roots: TNode[] = []
    m.rows.forEach((r) => {
      const node = nodes.get(r.component)!
      const parent = r.partOf ? nodes.get(r.partOf) : undefined
      if (parent && parent !== node) parent.children.push(node)
      else roots.push(node)
    })
    const keep = (r: Row) =>
      (showUnclassified || r.known) &&
      (!issuesOnly || hasIssue(r)) &&
      (!q || r.displayName.toLowerCase().includes(q) || r.component.includes(q))
    const visible = (n: TNode): TNode | null => {
      const children = n.children.map(visible).filter((c): c is TNode => c !== null)
      return keep(n.row) || children.length > 0 ? { ...n, children } : null
    }
    const byCat = new Map<string, TNode[]>()
    roots
      .map(visible)
      .filter((n): n is TNode => n !== null)
      .forEach((n) => byCat.set(n.row.category, [...(byCat.get(n.row.category) ?? []), n]))
    return [...byCat.entries()]
  }, [m, issuesOnly, query, showUnclassified])

  if (error) return <ErrorBox error={error} />
  if (!m) return <Loading what="inventory" />
  const unclassified = m.rows.filter((r) => !r.known).length
  const envNames = m.environments.map((e) => e.name)
  const filtering = issuesOnly || query !== ''
  const parents = groups.flatMap(([, ns]) => flatten(ns).filter((n) => n.children.length > 0).map((n) => n.row.component))
  const tree: TreeState = {
    envs: envNames,
    isOpen: (name) => filtering || openParents.has(name),
    toggle: (name) =>
      setOpenParents((prev) => {
        const next = new Set(prev)
        if (next.has(name)) next.delete(name)
        else next.add(name)
        return next
      }),
  }
  const toggleCat = (cat: string) =>
    setClosedCats((prev) => {
      const next = new Set(prev)
      if (next.has(cat)) next.delete(cat)
      else next.add(cat)
      return next
    })

  return (
    <div>
      <PageHeader
        title="Environments"
        subtitle={
          <>
            What is actually running, compared with each environment's target platform release
            {m.currentRelease && <> and the current release {vname(m.currentRelease)}</>}.
          </>
        }
        actions={<RefreshButton onClick={reload} loading={loading} />}
      />

      <div className="envs">
        {m.environments.map((e) => (
          <div key={e.name} className="card env-card">
            <div className="row-between">
              <h3>{e.displayName ?? e.name}</h3>
              {e.tier && <span className="muted small">{e.tier}</span>}
            </div>
            <dl className="kv">
              <dt>Kubernetes</dt>
              <dd>
                {e.cluster.version ?? '–'} {e.cluster.distribution?.toUpperCase()}
              </dd>
              {e.cluster.nodes ? (
                <>
                  <dt>Nodes</dt>
                  <dd>{e.cluster.nodes}</dd>
                </>
              ) : null}
              <dt>Target</dt>
              <dd>{e.targetRelease ? vname(e.targetRelease) : '–'}</dd>
              <dt>Running</dt>
              <dd>{e.matchedRelease ? vname(e.matchedRelease) : 'no release matched'}</dd>
              <dt>Collected</dt>
              <dd>{formatDate(e.collectedAt)}</dd>
            </dl>
            <div className="env-status">
              {e.compliant && <Pill tone="good">Compliant</Pill>}
              {e.drift > 0 && <Pill tone="warn">{e.drift} drift</Pill>}
              {e.missing > 0 && <Pill tone="bad">{e.missing} missing</Pill>}
              {e.untracked > 0 && <Pill tone="violet">{e.untracked} untracked</Pill>}
            </div>
            {e.errors?.map((err) => (
              <div key={err} className="env-error" title={err}>
                {err}
              </div>
            ))}
          </div>
        ))}
      </div>

      <div className="toolbar">
        <input
          className="search"
          placeholder="Filter components…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <label className="toggle">
          <input type="checkbox" checked={issuesOnly} onChange={(e) => setIssuesOnly(e.target.checked)} />
          Only show issues
        </label>
        {unclassified > 0 && (
          <label className="toggle">
            <input
              type="checkbox"
              checked={showUnclassified}
              onChange={(e) => setShowUnclassified(e.target.checked)}
            />
            Include components not in catalog ({unclassified})
          </label>
        )}
        <div className="toolbar-end">
          <button
            className="btn"
            onClick={() => {
              setClosedCats(new Set())
              setOpenParents(new Set(parents))
            }}
          >
            Expand all
          </button>
          <button
            className="btn"
            onClick={() => {
              setClosedCats(new Set(groups.map(([c]) => c)))
              setOpenParents(new Set())
            }}
          >
            Collapse all
          </button>
        </div>
      </div>
      <div className="legends">
        <Legend />
        <KindLegend />
      </div>

      <div className="card table-card">
        <table className="matrix">
          <thead>
            <tr>
              <th>Component</th>
              <th>Release {m.currentRelease ? vname(m.currentRelease) : '–'}</th>
              {m.environments.map((e) => (
                <th key={e.name}>{e.displayName ?? e.name}</th>
              ))}
              <th>Latest upstream</th>
            </tr>
          </thead>
          <tbody>
            {groups.map(([cat, nodes]) => {
              const open = filtering || !closedCats.has(cat)
              return (
                <Fragment key={cat}>
                  <tr className={`group-row clickable catc-${cat}`} onClick={() => toggleCat(cat)}>
                    <td colSpan={envNames.length + 3}>
                      <div className="group-head">
                        <span className="caret">{open ? '▾' : '▸'}</span>
                        <CategoryIcon category={cat} />
                        <span className="group-name">{categoryName(cat)}</span>
                        <span className="muted">{flatten(nodes).length}</span>
                        <span className="group-pills">
                          <StatPills s={stats(nodes, envNames)} />
                        </span>
                      </div>
                    </td>
                  </tr>
                  {open &&
                    nodes.map((n, i) => (
                      <NodeRows key={n.row.component} node={n} depth={0} last={i === nodes.length - 1} cat={cat} {...tree} />
                    ))}
                </Fragment>
              )
            })}
            {groups.length === 0 && (
              <tr>
                <td colSpan={envNames.length + 3} className="empty">
                  Nothing to show.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  )
}

interface TreeState {
  envs: string[]
  isOpen: (name: string) => boolean
  toggle: (name: string) => void
}

function NodeRows({
  node,
  depth,
  last,
  cat,
  ...tree
}: TreeState & { node: TNode; depth: number; last: boolean; cat: string }) {
  const r = node.row
  const hasKids = node.children.length > 0
  const open = hasKids && tree.isOpen(r.component)
  return (
    <>
      <tr className={`tree-row catc-${cat}`}>
        <td>
          <div
            className={`tree-cell ${depth > 0 ? 'child' : ''} ${last ? 'last' : ''}`}
            style={{ ['--depth' as string]: depth }}
          >
            {hasKids ? (
              <button className="tree-toggle" onClick={() => tree.toggle(r.component)} aria-expanded={open}>
                {open ? '▾' : '▸'}
              </button>
            ) : (
              <span className="tree-spacer" />
            )}
            <div className="tree-label">
              <div className="comp-name">
                {r.displayName}
                <KindBadge kind={r.kind} />
                {!r.known && r.category !== 'platform-api' && <span className="tag">not in catalog</span>}
              </div>
              {hasKids && (
                <button className="link-btn small" onClick={() => tree.toggle(r.component)}>
                  {open ? 'Hide' : 'Show'} {childSummary(node.children)}
                </button>
              )}
              {hasKids && !open && (
                <span className="group-pills inline">
                  <StatPills s={stats(node.children, tree.envs)} />
                </span>
              )}
            </div>
          </div>
        </td>
        <td className="mono">{r.declared ?? '–'}</td>
        {tree.envs.map((env) => (
          <MatrixCell key={env} cell={r.cells[env]} />
        ))}
        <td>
          <span className="mono">{r.latest ?? '–'}</span>{' '}
          {r.freshness !== 'unknown' && <FreshnessPill value={r.freshness} />}
        </td>
      </tr>
      {open &&
        node.children.map((c, i) => (
          <NodeRows
            key={c.row.component}
            node={c}
            depth={depth + 1}
            last={i === node.children.length - 1}
            cat={cat}
            {...tree}
          />
        ))}
    </>
  )
}

function KindLegend() {
  return (
    <div className="legend">
      {Object.entries(kindInfo).map(([k, v]) => (
        <span key={k} className={`kind kind-${k}`}>
          {v.label}
        </span>
      ))}
    </div>
  )
}

function MatrixCell({ cell }: { cell?: Cell }) {
  if (!cell || cell.status === 'absent') return <td className="cell-absent">·</td>
  const s = statusInfo[cell.status]
  const behind = cell.freshness !== 'latest' && cell.freshness !== 'unknown'
  const title = [s.label, cell.expected && `expected ${cell.expected}`, behind && freshnessInfo[cell.freshness].label]
    .filter(Boolean)
    .join(' · ')
  return (
    <td title={title}>
      <span className={`cell cell-${s.tone}`}>
        <span className="cell-dot" />
        <span className="mono">{cell.version || (cell.status === 'missing' ? 'missing' : '?')}</span>
        {behind && <span className={`arrow tone-${freshnessInfo[cell.freshness].tone}`}>↑</span>}
      </span>
      {cell.status === 'drift' && <div className="expected">want {cell.expected}</div>}
    </td>
  )
}

function Legend() {
  return (
    <div className="legend">
      {(['aligned', 'drift', 'missing', 'untracked'] as const).map((s) => (
        <span key={s} className={`cell cell-${statusInfo[s].tone}`}>
          <span className="cell-dot" />
          {titleCase(s)}
        </span>
      ))}
      <span className="muted">↑ newer upstream</span>
    </div>
  )
}
