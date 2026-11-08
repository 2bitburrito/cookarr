// tmux/i3-style keyboard shortcuts for the workspace, built on Dockview's
// activate/move/close APIs (Dockview has no shortcut system of its own —
// see docs/ROADMAP.md "Keyboard shortcuts, status bar").
//
// Keymap (all with Alt held, to avoid colliding with browser/OS shortcuts):
//   Alt+L / Alt+ArrowRight   focus next panel
//   Alt+H / Alt+ArrowLeft    focus previous panel
//   Alt+\                    split the active panel to the right (vertical split)
//   Alt+-                    split the active panel downward (horizontal split)
//   Alt+W                    close the active panel
//   Alt+P                    pop the active panel out into its own window
//   Alt+1..9                 jump straight to the Nth panel, in tab order
import type { DockviewApi } from 'dockview-core/dist/dockview-core.js'

export function handleWorkspaceKeydown(e: KeyboardEvent, api: DockviewApi | undefined): boolean {
  if (!api || !e.altKey || e.ctrlKey || e.metaKey) return false

  const active = api.activePanel
  const group = api.activeGroup

  switch (e.key) {
    case 'l':
    case 'ArrowRight':
      api.activateNext({ includePanel: true })
      return true
    case 'h':
    case 'ArrowLeft':
      api.activatePrevious({ includePanel: true })
      return true
    case '\\':
      if (active && group) active.api.moveTo({ group, position: 'right' })
      return true
    case '-':
      if (active && group) active.api.moveTo({ group, position: 'bottom' })
      return true
    case 'w':
      active?.api.close()
      return true
    case 'p':
      if (active) void api.addPopoutGroup(active)
      return true
    default: {
      const n = Number(e.key)
      if (Number.isInteger(n) && n >= 1 && n <= 9) {
        const panels = api.panels
        const target = panels[n - 1]
        if (target) target.api.setActive()
        return true
      }
      return false
    }
  }
}
