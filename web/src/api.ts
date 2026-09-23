export interface Metadata {
  name: string
  displayName?: string
}

export interface Offering {
  name: string
  displayName?: string
  category?: string
  description?: string
  status?: string
  docs?: string
  components?: string[]
}

export interface Environment {
  name: string
  displayName?: string
  tier?: string
  targetRelease?: string
}

export interface Platform {
  metadata: Metadata
  spec: {
    tagline?: string
    description?: string
    owners?: { name: string; email?: string; channel?: string }[]
    links?: { title: string; url: string }[]
    guarantees?: string[]
    offerings?: Offering[]
    environments: Environment[]
  }
}

export interface PlatformRelease {
  metadata: Metadata
  spec: {
    version: string
    date?: string
    summary?: string
    highlights?: string[]
    notes?: string
    components: Record<string, string>
    offerings?: string[]
  }
}

export interface PlatformResponse {
  platform: Platform
  readme?: string
  currentRelease?: PlatformRelease
}

export type Freshness = 'unknown' | 'latest' | 'patch-available' | 'minor-behind' | 'major-behind' | 'unsupported'

export interface Cell {
  version?: string
  expected?: string
  status: 'aligned' | 'drift' | 'missing' | 'untracked' | 'present' | 'absent'
  freshness: Freshness
}

export interface Row {
  component: string
  displayName: string
  category: string
  known: boolean
  partOf?: string
  kind?: string
  declared?: string
  latest?: string
  freshness: Freshness
  cells: Record<string, Cell>
}

export interface EnvStatus {
  name: string
  displayName?: string
  tier?: string
  targetRelease?: string
  matchedRelease?: string
  compliant: boolean
  cluster: { version?: string; distribution?: string; nodes?: number; context?: string }
  collectedAt: string
  errors?: string[]
  aligned: number
  drift: number
  missing: number
  untracked: number
}

export interface Matrix {
  generatedAt: string
  currentRelease?: string
  environments: EnvStatus[]
  rows: Row[]
  summary: {
    components: number
    environments: number
    offerings: number
    releases: number
    aligned: number
    drift: number
    missing: number
    untracked: number
    upToDate: number
    behind: number
    unsupported: number
    alignment: number
    currency: number
  }
}

export interface UpstreamRelease {
  version: string
  tag: string
  name?: string
  publishedAt: string
  url: string
  notes?: string
}

export interface Update {
  component: string
  displayName: string
  category: string
  homepage?: string
  repository?: string
  versions: Record<string, string>
  current?: string
  currentRelease?: UpstreamRelease
  latest?: UpstreamRelease
  freshness: Freshness
  minorsBehind: number
  reasons: string[]
  newer?: UpstreamRelease[]
  error?: string
}

export interface ComponentChange {
  component: string
  displayName: string
  category: string
  from?: string
  to?: string
  change: 'added' | 'removed' | 'upgraded' | 'downgraded' | 'unchanged'
}

export interface ReleaseDiff {
  from: string
  to: string
  components: ComponentChange[]
  offeringsAdded?: string[]
  offeringsRemoved?: string[]
}

export interface CatalogComponent {
  metadata: Metadata
  spec: {
    displayName?: string
    category?: string
    description?: string
    homepage?: string
    partOf?: string
    discovery: {
      kubernetes?: boolean
      images?: string[]
      helmCharts?: string[]
      apiGroups?: string[]
      crossplanePackages?: string[]
    }
    upstream?: { github?: string }
  }
}

export interface Evidence {
  source: string
  namespace?: string
  object?: string
  detail?: string
  version?: string
}

export interface Inventory {
  environment: string
  components: { name: string; version?: string; known: boolean; evidence?: Evidence[] }[]
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: 'application/json' } })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? `${res.status} ${res.statusText}`)
  }
  return res.json()
}

const q = (refresh?: boolean) => (refresh ? '?refresh=true' : '')

export const api = {
  platform: () => get<PlatformResponse>('/api/platform'),
  matrix: (refresh?: boolean) => get<Matrix>('/api/matrix' + q(refresh)),
  updates: (refresh?: boolean) => get<Update[]>('/api/updates' + q(refresh)),
  releases: () => get<PlatformRelease[]>('/api/releases'),
  diff: (from: string, to: string) =>
    get<ReleaseDiff>(`/api/diff?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`),
  catalog: () => get<CatalogComponent[]>('/api/catalog'),
  inventory: () => get<Record<string, Inventory>>('/api/inventory'),
}
