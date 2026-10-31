import type { DockviewApi, ITabRenderer, TabPartInitParameters } from 'dockview-core/dist/dockview-core.js'

const FLOAT_ICON =
  '<svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="5.5" y="2.5" width="8" height="8" rx="2"/><path d="M3 5.5v5a2.5 2.5 0 0 0 2.5 2.5h5"/></svg>'
const DOCK_ICON =
  '<svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="2.5" y="2.5" width="11" height="11" rx="2.5"/><path d="M2.5 6.5h11"/></svg>'
const CLOSE_ICON =
  '<svg viewBox="0 0 16 16" width="10" height="10" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"><path d="M3.5 3.5l9 9M12.5 3.5l-9 9"/></svg>'

function button(cls: string, label: string, onClick: () => void) {
  const b = document.createElement('button')
  b.type = 'button'
  b.className = `tab-btn ${cls}`
  b.title = label
  b.setAttribute('aria-label', label)
  // Keep dockview from treating the click as a tab drag or activation.
  b.addEventListener('pointerdown', (e) => e.stopPropagation())
  b.addEventListener('click', (e) => {
    e.stopPropagation()
    onClick()
  })
  return b
}

// Tab with a float/dock toggle next to the title and a close button.
export function createTab(getApi: () => DockviewApi | undefined): ITabRenderer {
  const element = document.createElement('div')
  element.className = 'cookarr-tab'
  const title = document.createElement('span')
  title.className = 'tab-title'
  const subs: { dispose(): void }[] = []

  return {
    element,
    init({ api, title: initial }: TabPartInitParameters) {
      title.textContent = initial ?? api.title ?? ''

      const dv = () => getApi()
      const float = button('tab-float', 'Float', () => {
        const d = dv()
        const panel = d?.getPanel(api.id)
        if (!d || !panel) return
        if (api.location.type === 'floating') {
          const home = d.groups.find((g) => g.api.location.type === 'grid')
          if (home) panel.api.moveTo({ group: home })
        } else {
          d.addFloatingGroup(panel, { position: { left: 120, top: 120 }, width: 360, height: 300 })
        }
      })
      const close = button('tab-close', 'Close', () => api.close())

      const syncFloat = () => {
        const floating = api.location.type === 'floating'
        float.innerHTML = floating ? DOCK_ICON : FLOAT_ICON
        float.title = floating ? 'Dock' : 'Float'
        float.setAttribute('aria-label', float.title)
      }
      close.innerHTML = CLOSE_ICON
      syncFloat()

      element.replaceChildren(title, float, close)
      subs.push(api.onDidTitleChange((e) => (title.textContent = e.title)))
      subs.push(api.onDidLocationChange(syncFloat))
    },
    dispose() {
      subs.forEach((s) => s.dispose())
    },
  }
}
