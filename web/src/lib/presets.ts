import type { DockviewApi } from 'dockview-core/dist/dockview-core.js'

export type PresetName = 'classic' | 'cook' | 'read' | 'phone'

type Title = (id: string) => string

function panel(api: DockviewApi, title: Title, id: string, position?: Parameters<DockviewApi['addPanel']>[0]['position']) {
  return api.addPanel({ id, component: id, title: title(id), position })
}

// Built-in layouts from docs/PLAN.md. Each builds panels on a cleared workspace.
export const presets: Record<PresetName, (api: DockviewApi, title: Title) => void> = {
  // Ingredients and steps side by side, story in a tab.
  classic(api, title) {
    const ing = panel(api, title, 'ingredients')
    panel(api, title, 'steps', { referencePanel: ing, direction: 'right' })
    panel(api, title, 'story', { referencePanel: ing, direction: 'within' })
    panel(api, title, 'photos', { referencePanel: ing, direction: 'below' })
    panel(api, title, 'author', { referencePanel: 'photos', direction: 'within' })
    ing.api.setActive()
  },
  // Narrow ingredients, one large steps panel.
  cook(api, title) {
    const steps = panel(api, title, 'steps')
    const ing = panel(api, title, 'ingredients', { referencePanel: steps, direction: 'left' })
    ing.api.setSize({ width: 300 })
    panel(api, title, 'story', { referencePanel: steps, direction: 'within' })
    steps.api.setActive()
  },
  // Story first, ingredients floating on top.
  read(api, title) {
    const story = panel(api, title, 'story')
    panel(api, title, 'photos', { referencePanel: story, direction: 'right' })
    const ing = panel(api, title, 'ingredients', { referencePanel: story, direction: 'within' })
    panel(api, title, 'steps', { referencePanel: 'photos', direction: 'below' })
    api.addFloatingGroup(ing, { position: { right: 40, top: 70 }, width: 320, height: 380 })
    story.api.setActive()
  },
  // One panel at a time (tabbed) for narrow screens, per docs/PLAN.md open
  // question #3: ingredients must stay reachable even on a phone, so they're
  // pinned as a floating overlay rather than another tab to swipe past.
  phone(api, title) {
    const steps = panel(api, title, 'steps')
    panel(api, title, 'story', { referencePanel: steps, direction: 'within' })
    panel(api, title, 'photos', { referencePanel: steps, direction: 'within' })
    panel(api, title, 'author', { referencePanel: steps, direction: 'within' })
    const ing = panel(api, title, 'ingredients', { referencePanel: steps, direction: 'within' })
    const width = Math.max(260, Math.min(420, window.innerWidth - 24))
    api.addFloatingGroup(ing, { position: { left: 12, bottom: 12 }, width, height: 180 })
    steps.api.setActive()
  },
}

export function applyPreset(api: DockviewApi, name: PresetName, title: Title) {
  presets[name](api, title)
}
