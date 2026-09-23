import { useMemo, useState } from 'react'
import { api, type CatalogComponent, type EnvStatus, type Evidence, type Row } from '../api'
import {
  CategoryIcon,
  ErrorBox,
  FreshnessPill,
  KindBadge,
  Loading,
  PageHeader,
  Pill,
  categoryName,
  statusInfo,
  useAsync,
} from '../components'

const categoryOrder = [
  'runtime',
  'compute',
  'delivery',
  'infrastructure',
  'composition',
  'platform-api',
  'networking',
  'security',
  'observability',
  'data',
  'other',
]

const categoryBlurb: Record<string, string> = {
  runtime: 'Where workloads run',
  compute: 'Nodes, scaling and resource metrics',
  delivery: 'How changes reach every environment',
  infrastructure: 'Self-service cloud resources',
  composition: 'Building blocks for platform APIs',
  'platform-api': 'Crossplane configurations your teams consume',
  networking: 'Traffic, ingress, mesh and DNS',
  security: 'Certificates, secrets and policy',
  observability: 'Metrics, logs and traces',
  data: 'Backup and data services',
  other: 'Everything else',
}

interface Node {
  comp: CatalogComponent
  row?: Row
  // Discovered in a cluster but not described by any catalog definition.
  discovered?: boolean
  children: Node[]
}

const nameOf = (c: CatalogComponent) => c.spec.displayName ?? c.metadata.name

function versionOf(row?: Row) {
  if (!row) return undefined
  return row.declared ?? Object.values(row.cells).find((c) => c.version)?.version
}

function flatten(nodes: Node[]): Node[] {
  return nodes.flatMap((n) => [n, ...flatten(n.children)])
}

function byUseThenName(a: Node, b: Node) {
  if (!!a.row !== !!b.row) return a.row ? -1 : 1
  return nameOf(a.comp).localeCompare(nameOf(b.comp))
}

export default function CatalogPage() {
  const catalog = useAsync(() => api.catalog())
  const matrix = useAsync(() => api.matrix())
  const inventory = useAsync(() => api.inventory())
  const [mode, setMode] = useState<'platform' | 'all'>('platform')
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState<Set<string>>(() => new Set())

  const rows = useMemo(() => new Map((matrix.data?.rows ?? []).map((r) => [r.component, r])), [matrix.data])
  const evidence = useMemo(() => {
    const out = new Map<string, Evidence[]>()
    Object.values(inventory.data ?? {}).forEach((inv) =>
      inv.components.forEach((c) => {
        if (!out.has(c.name) && c.evidence?.length) out.set(c.name, c.evidence)
      }),
    )
    return out
  }, [inventory.data])
  const allComponents = useMemo(() => {
    const known = new Set((catalog.data ?? []).map((c) => c.metadata.name))
    const discovered: CatalogComponent[] = (matrix.data?.rows ?? [])
      .filter((r) => !known.has(r.component))
      .map((r) => ({
        metadata: { name: r.component },
        spec: { displayName: r.displayName, category: r.category, partOf: r.partOf, discovery: {} },
      }))
    return { list: [...(catalog.data ?? []), ...discovered], known }
  }, [catalog.data, matrix.data])
  const inUseCount = rows.size
  const effectiveMode = inUseCount === 0 ? 'all' : mode
  const q = query.trim().toLowerCase()

  const groups = useMemo(() => {
    if (!catalog.data) return []
    const nodes = new Map<string, Node>(
      allComponents.list.map((c) => [
        c.metadata.name,
        {
          comp: c,
          row: rows.get(c.metadata.name),
          discovered: !allComponents.known.has(c.metadata.name),
          children: [],
        },
      ]),
    )
    const roots: Node[] = []
    nodes.forEach((n) => {
      const parent = n.comp.spec.partOf ? nodes.get(n.comp.spec.partOf) : undefined
      if (parent && parent !== n) parent.children.push(n)
      else roots.push(n)
    })
    const matches = (n: Node) =>
      [n.comp.metadata.name, n.comp.spec.displayName, n.comp.spec.description].some((s) =>
        s?.toLowerCase().includes(q),
      )
    const visible = (n: Node): Node | null => {
      const children = n.children
        .map(visible)
        .filter((c): c is Node => c !== null)
        .sort(byUseThenName)
      const self = (effectiveMode === 'all' || n.row !== undefined) && (!q || matches(n))
      return self || children.length > 0 ? { ...n, children } : null
    }
    const byCat = new Map<string, Node[]>()
    roots
      .map(visible)
      .filter((n): n is Node => n !== null)
      .forEach((n) => {
        const k = n.comp.spec.category ?? 'other'
        byCat.set(k, [...(byCat.get(k) ?? []), n])
      })
    const rank = (c: string) => (categoryOrder.includes(c) ? categoryOrder.indexOf(c) : categoryOrder.length)
    return [...byCat.entries()]
      .map(([cat, ns]) => [cat, ns.sort(byUseThenName)] as const)
      .sort(([a], [b]) => rank(a) - rank(b))
  }, [catalog.data, allComponents, rows, effectiveMode, q])

  if (catalog.error) return <ErrorBox error={catalog.error} />
  if (!catalog.data) return <Loading what="components" />

  const envs = matrix.data?.environments ?? []
  const updates = (matrix.data?.rows ?? []).filter(
    (r) => r.known && r.freshness !== 'latest' && r.freshness !== 'unknown',
  ).length
  const toggle = (key: string) =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  const categoryOpen = (cat: string) => q !== '' || open.has('cat:' + cat)

  return (
    <div>
      <PageHeader
        title="Components"
        subtitle="What the platform is made of, grouped by capability. Expand a capability to see its components, their versions and where they run."
        actions={
          updates > 0 && (
            <a className="btn" href="#/updates">
              {updates} update{updates === 1 ? '' : 's'} available →
            </a>
          )
        }
      />
      <div className="toolbar">
        <div className="seg">
          <button
            className={effectiveMode === 'platform' ? 'active' : ''}
            disabled={inUseCount === 0}
            onClick={() => setMode('platform')}
          >
            In this platform ({inUseCount})
          </button>
          <button className={effectiveMode === 'all' ? 'active' : ''} onClick={() => setMode('all')}>
            Full catalog ({allComponents.list.length})
          </button>
        </div>
        <input
          className="search"
          placeholder="Search components…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <div className="toolbar-end">
          <button className="btn" onClick={() => setOpen(new Set(groups.map(([c]) => 'cat:' + c)))}>
            Expand all
          </button>
          <button className="btn" onClick={() => setOpen(new Set())}>
            Collapse all
          </button>
        </div>
      </div>

      <div className="tree">
        {groups.map(([cat, nodes]) => (
          <CategoryGroup
            key={cat}
            category={cat}
            nodes={nodes}
            envs={envs}
            evidence={evidence}
            expanded={categoryOpen(cat)}
            open={open}
            toggle={toggle}
          />
        ))}
        {groups.length === 0 && <div className="card empty">No components match.</div>}
      </div>
      <p className="muted small">
        Add or override component definitions with <code>componentsDir</code> in the platform file.
      </p>
    </div>
  )
}

interface TreeProps {
  envs: EnvStatus[]
  evidence: Map<string, Evidence[]>
  open: Set<string>
  toggle: (key: string) => void
}

function CategoryGroup({
  category,
  nodes,
  expanded,
  ...tree
}: TreeProps & { category: string; nodes: Node[]; expanded: boolean }) {
  const all = flatten(nodes)
  const used = all.filter((n) => n.row)
  const summary = nodes.filter((n) => n.row)
  return (
    <section className={`card cat-group ${expanded ? 'open' : ''}`}>
      <button className="cat-header" onClick={() => tree.toggle('cat:' + category)} aria-expanded={expanded}>
        <span className="caret">{expanded ? '▾' : '▸'}</span>
        <CategoryIcon category={category} />
        <span className="cat-title">
          <strong>{categoryName(category)}</strong>
          <span className="muted small">{categoryBlurb[category] ?? ''}</span>
        </span>
        <span className="cat-summary">
          {!expanded &&
            summary.slice(0, 4).map((n) => {
              const v = versionOf(n.row)
              return (
                <span key={n.comp.metadata.name} className="chip">
                  {nameOf(n.comp)}
                  {v && <span className="chip-version">{v}</span>}
                </span>
              )
            })}
          {!expanded && summary.length > 4 && <span className="muted small">+{summary.length - 4} more</span>}
        </span>
        <span className="muted small cat-count">
          {used.length > 0 ? `${used.length} in use` : `${all.length} available`}
        </span>
      </button>
      {expanded && (
        <div className="cat-body">
          {nodes.map((n) => (
            <ComponentNode key={n.comp.metadata.name} node={n} {...tree} />
          ))}
        </div>
      )}
    </section>
  )
}

function ComponentNode({ node, ...tree }: TreeProps & { node: Node }) {
  const { comp, row } = node
  const key = 'comp:' + comp.metadata.name
  const expanded = tree.open.has(key)
  const v = versionOf(row)
  return (
    <div className="comp-node">
      <button className={`comp-row ${row ? '' : 'unused'}`} onClick={() => tree.toggle(key)} aria-expanded={expanded}>
        <span className="caret">{expanded ? '▾' : '▸'}</span>
        <span className="comp-label">
          <span className="comp-name">
            {nameOf(comp)}
            <KindBadge kind={row?.kind} />
            {node.discovered && comp.spec.category !== 'platform-api' && <span className="tag">not in catalog</span>}
          </span>
          {comp.spec.description && <span className="muted small comp-desc">{comp.spec.description}</span>}
        </span>
        {row ? (
          <>
            <span className="env-dots">
              {tree.envs.map((e) => {
                const c = row.cells[e.name]
                const s = statusInfo[c?.status ?? 'absent']
                return (
                  <span
                    key={e.name}
                    className={`env-dot cell-${s.tone}`}
                    title={`${e.displayName ?? e.name}: ${c?.version || 'not installed'} · ${s.label}`}
                  >
                    <span className="cell-dot" />
                    {e.name}
                  </span>
                )
              })}
            </span>
            <span className="mono comp-version">{v ?? '–'}</span>
            <span className="comp-status">
              {row.freshness !== 'unknown' && <FreshnessPill value={row.freshness} />}
            </span>
          </>
        ) : (
          <span className="muted small unused-note">Not in this platform</span>
        )}
      </button>
      {expanded && <ComponentDetail node={node} envs={tree.envs} evidence={tree.evidence.get(comp.metadata.name)} />}
      {node.children.length > 0 && (
        <div className="comp-children">
          {node.children.map((c) => (
            <ComponentNode key={c.comp.metadata.name} node={c} {...tree} />
          ))}
        </div>
      )}
    </div>
  )
}

const maxEvidence = 8

function ComponentDetail({ node, envs, evidence }: { node: Node; envs: EnvStatus[]; evidence?: Evidence[] }) {
  const { comp, row } = node
  const d = comp.spec.discovery
  const signals = [
    d.kubernetes && 'cluster version',
    ...(d.images ?? []).map((i) => `image ${i}`),
    ...(d.helmCharts ?? []).map((h) => `chart ${h}`),
    ...(d.crossplanePackages ?? []).map((p) => `package ${p}`),
    ...(d.apiGroups ?? []).map((g) => `api ${g}`),
  ].filter(Boolean) as string[]
  const repo = comp.spec.upstream?.github

  return (
    <div className="comp-detail">
      <div className="detail-grid">
        {row && envs.length > 0 && (
          <div>
            <h4>Where it runs</h4>
            <table className="bom compact">
              <tbody>
                {envs.map((e) => {
                  const c = row.cells[e.name]
                  return (
                    <tr key={e.name}>
                      <td>{e.displayName ?? e.name}</td>
                      <td className="mono">{c?.version || '–'}</td>
                      <td className="right">
                        {c && c.status !== 'absent' && <Pill tone={statusInfo[c.status].tone}>{c.status}</Pill>}
                      </td>
                    </tr>
                  )
                })}
                {row.declared && (
                  <tr>
                    <td>Current release</td>
                    <td className="mono">{row.declared}</td>
                    <td />
                  </tr>
                )}
                {row.latest && (
                  <tr>
                    <td>Latest upstream</td>
                    <td className="mono">{row.latest}</td>
                    <td />
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        )}
        <div>
          <h4>How it is discovered</h4>
          {node.discovered && (
            <p className="small muted">
              No catalog definition yet, so pbom cannot check upstream releases. Add one with <code>componentsDir</code>.
            </p>
          )}
          <div className="signals">
            {signals.map((s) => (
              <code key={s}>{s}</code>
            ))}
          </div>
          {evidence && evidence.length > 0 && (
            <ul className="evidence">
              {evidence.slice(0, maxEvidence).map((e, i) => (
                <li key={i}>
                  <span className="tag">{e.source}</span>
                  <span className="mono">{e.object}</span>
                  {e.namespace && <span className="muted"> in {e.namespace}</span>}
                  {e.detail && <div className="mono muted evidence-detail">{e.detail}</div>}
                </li>
              ))}
              {evidence.length > maxEvidence && (
                <li className="muted">+{evidence.length - maxEvidence} more</li>
              )}
            </ul>
          )}
          <div className="detail-links small">
            {comp.spec.homepage && (
              <a href={comp.spec.homepage} target="_blank" rel="noreferrer noopener">
                Homepage ↗
              </a>
            )}
            {repo && (
              <a href={`https://github.com/${repo}/releases`} target="_blank" rel="noreferrer noopener">
                {repo} releases ↗
              </a>
            )}
            {row && row.freshness !== 'latest' && row.freshness !== 'unknown' && (
              <a href="#/updates">Update advice →</a>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
