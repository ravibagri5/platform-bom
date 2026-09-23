import { useCallback, useEffect, useState, type ReactNode } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { Cell, Freshness } from './api'

export function useAsync<T>(fn: (refresh: boolean) => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T>()
  const [error, setError] = useState<string>()
  const [loading, setLoading] = useState(true)
  const run = useCallback(fn, deps)
  const load = useCallback(
    (refresh: boolean) => {
      setLoading(true)
      run(refresh)
        .then((d) => {
          setData(d)
          setError(undefined)
        })
        .catch((e: Error) => setError(e.message))
        .finally(() => setLoading(false))
    },
    [run],
  )
  useEffect(() => load(false), [load])
  return { data, error, loading, reload: () => load(true) }
}

export const freshnessInfo: Record<Freshness, { label: string; tone: string }> = {
  latest: { label: 'Up to date', tone: 'good' },
  'patch-available': { label: 'Patch available', tone: 'info' },
  'minor-behind': { label: 'Minor behind', tone: 'warn' },
  'major-behind': { label: 'Major behind', tone: 'alert' },
  unsupported: { label: 'Unsupported', tone: 'bad' },
  unknown: { label: 'Unknown', tone: 'muted' },
}

export const statusInfo: Record<Cell['status'], { label: string; tone: string }> = {
  aligned: { label: 'Matches target release', tone: 'good' },
  drift: { label: 'Differs from target release', tone: 'warn' },
  missing: { label: 'Missing from environment', tone: 'bad' },
  untracked: { label: 'Not part of target release', tone: 'violet' },
  present: { label: 'Running (no target release)', tone: 'muted' },
  absent: { label: 'Not installed', tone: 'none' },
}

export function Pill({ tone, children, title }: { tone: string; children: ReactNode; title?: string }) {
  return (
    <span className={`pill pill-${tone}`} title={title}>
      {children}
    </span>
  )
}

export function FreshnessPill({ value }: { value: Freshness }) {
  const f = freshnessInfo[value]
  return <Pill tone={f.tone}>{f.label}</Pill>
}

export function Markdown({ children }: { children?: string }) {
  if (!children) return null
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => (
            <a href={href} target="_blank" rel="noreferrer noopener">
              {children}
            </a>
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  )
}

export function Loading({ what }: { what: string }) {
  return (
    <div className="loading">
      <div className="spinner" />
      Loading {what}…
    </div>
  )
}

export function ErrorBox({ error }: { error: string }) {
  return <div className="error-box">{error}</div>
}

export function PageHeader({
  title,
  subtitle,
  actions,
}: {
  title: string
  subtitle?: ReactNode
  actions?: ReactNode
}) {
  return (
    <div className="page-header">
      <div>
        <h1>{title}</h1>
        {subtitle && <p className="subtitle">{subtitle}</p>}
      </div>
      {actions && <div className="actions">{actions}</div>}
    </div>
  )
}

export function RefreshButton({ onClick, loading }: { onClick: () => void; loading: boolean }) {
  return (
    <button className="btn" onClick={onClick} disabled={loading}>
      {loading ? 'Refreshing…' : 'Rediscover'}
    </button>
  )
}

const categoryGlyph: Record<string, string> = {
  runtime: 'K',
  compute: 'C',
  delivery: 'D',
  infrastructure: 'I',
  composition: 'F',
  networking: 'N',
  security: 'S',
  observability: 'O',
  data: 'B',
  'developer-experience': 'X',
  'platform-api': 'A',
}

const categoryLabels: Record<string, string> = {
  'platform-api': 'Platform APIs',
  'developer-experience': 'Developer experience',
}

export const categoryName = (c: string) => categoryLabels[c] ?? titleCase(c)

export const kindInfo: Record<string, { label: string; plural: string }> = {
  cluster: { label: 'Cluster', plural: 'clusters' },
  controller: { label: 'Controller', plural: 'controllers' },
  helm: { label: 'Helm chart', plural: 'Helm charts' },
  provider: { label: 'Provider', plural: 'providers' },
  function: { label: 'Function', plural: 'functions' },
  configuration: { label: 'Configuration', plural: 'configurations' },
  api: { label: 'API only', plural: 'APIs' },
}

export function KindBadge({ kind }: { kind?: string }) {
  if (!kind || !kindInfo[kind]) return null
  return <span className={`kind kind-${kind}`}>{kindInfo[kind].label}</span>
}

export function CategoryIcon({ category }: { category?: string }) {
  const c = category ?? 'other'
  return <span className={`cat-icon cat-${c}`}>{categoryGlyph[c] ?? '•'}</span>
}

export function titleCase(s: string) {
  return s.charAt(0).toUpperCase() + s.slice(1)
}

export const vname = (s: string) => (/^v\d/i.test(s) ? s : `v${s}`)

export const humanize = (s: string) => s.split(/[-_ ]+/).map(titleCase).join(' ')

export const offeringStatusTone: Record<string, string> = {
  ga: 'good',
  beta: 'info',
  alpha: 'warn',
  planned: 'muted',
  deprecated: 'bad',
}

export function OfferingStatus({ status }: { status?: string }) {
  if (!status) return null
  return <Pill tone={offeringStatusTone[status] ?? 'muted'}>{status === 'ga' ? 'GA' : titleCase(status)}</Pill>
}

/** Groups items by key, keeping first-seen order. */
export function groupBy<T>(items: T[], key: (t: T) => string): [string, T[]][] {
  const m = new Map<string, T[]>()
  items.forEach((i) => m.set(key(i), [...(m.get(key(i)) ?? []), i]))
  return [...m.entries()]
}

export function formatDate(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })
}

export function Percent({ value }: { value: number }) {
  return <>{value < 0 ? '–' : `${value}%`}</>
}
