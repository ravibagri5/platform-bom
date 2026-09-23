import { useEffect, useState } from 'react'
import { api } from './api'
import { useAsync } from './components'
import PlatformPage from './pages/PlatformPage'
import InventoryPage from './pages/InventoryPage'
import ReleasesPage from './pages/ReleasesPage'
import UpdatesPage from './pages/UpdatesPage'
import CatalogPage from './pages/CatalogPage'
import OfferingsPage from './pages/OfferingsPage'

const nav = [
  { path: '', label: 'Platform' },
  { path: 'components', label: 'Components' },
  { path: 'offerings', label: 'Offerings' },
  { path: 'releases', label: 'Releases' },
  { path: 'environments', label: 'Environments' },
]

// Old paths and sub-pages mapped to their top-level concept.
const aliases: Record<string, string> = { catalog: 'components', updates: 'components', inventory: 'environments' }

function useHashRoute() {
  const read = () => window.location.hash.replace(/^#\/?/, '').split('/').filter(Boolean)
  const [route, setRoute] = useState(read)
  useEffect(() => {
    const on = () => {
      setRoute(read())
      window.scrollTo(0, 0)
    }
    window.addEventListener('hashchange', on)
    return () => window.removeEventListener('hashchange', on)
  }, [])
  return route
}

export default function App() {
  const route = useHashRoute()
  const page = route[0] ?? ''
  const section = aliases[page] ?? page
  const platform = useAsync(() => api.platform())
  const name = platform.data?.platform.metadata.displayName ?? platform.data?.platform.metadata.name

  useEffect(() => {
    document.title = name ? `${name} · Platform BOM` : 'Platform BOM'
  }, [name])

  return (
    <div className="app">
      <header className="topbar">
        <a className="brand" href="#/">
          <span className="brand-mark" />
          <span>
            <strong>{name ?? 'Platform BOM'}</strong>
            {platform.data?.currentRelease && (
              <span className="brand-version">v{platform.data.currentRelease.spec.version}</span>
            )}
          </span>
        </a>
        <nav>
          {nav.map((n) => (
            <a key={n.path} href={`#/${n.path}`} className={section === n.path ? 'active' : ''}>
              {n.label}
            </a>
          ))}
        </nav>
      </header>
      <main>
        {page === '' && <PlatformPage platform={platform} />}
        {(page === 'components' || page === 'catalog') && <CatalogPage />}
        {page === 'updates' && <UpdatesPage />}
        {page === 'offerings' && <OfferingsPage platform={platform.data} />}
        {page === 'releases' && <ReleasesPage selected={route[1]} platform={platform.data} />}
        {(page === 'environments' || page === 'inventory') && <InventoryPage />}
      </main>
      <footer>
        Platform BOM · the platform as a versioned product
      </footer>
    </div>
  )
}
