<script lang="ts">
  import { createDockview, type DockviewApi, type IContentRenderer } from 'dockview-core/dist/dockview-core.js'
  import { mount, onMount, onDestroy, unmount, untrack, type Component } from 'svelte'
  import Illustration from './Illustration.svelte'
  import Author from './panels/Author.svelte'
  import Ingredients from './panels/Ingredients.svelte'
  import Photos from './panels/Photos.svelte'
  import Steps from './panels/Steps.svelte'
  import Story from './panels/Story.svelte'
  import { createTab } from './tab'
  import { applyPreset, presets, type PresetName } from './presets'
  import { currentTheme, toggleTheme } from './theme'
  import { handleWorkspaceKeydown } from './keyboard'
  import * as api from './api'
  import type { DeviceClass } from './api'
  import type { Recipe } from './types'

  let { recipe, recipeId, deviceClass = 'desktop' }: {
    recipe: Recipe
    recipeId?: number
    deviceClass?: DeviceClass
  } = $props()

  // deviceClass only matters at creation time: Dockview owns its own layout
  // state once mounted, and remounting the whole workspace on every resize
  // would be jarring. A future enhancement could re-key the component on
  // breakpoint crossings if that's wanted; for now this reads once.
  const STORAGE_KEY = `cookarr:layout:${untrack(() => deviceClass)}`

  const components: Record<string, { component: Component<{ recipe: Recipe }>; title: string }> = {
    ingredients: { component: Ingredients, title: 'Ingredients' },
    steps: { component: Steps, title: 'Steps' },
    story: { component: Story, title: 'Story' },
    photos: { component: Photos, title: 'Photos' },
    author: { component: Author, title: 'Author' },
  }

  let host: HTMLDivElement
  let dvApi: DockviewApi | undefined = $state()
  let activePanel = $state('')
  let groupCount = $state(0)
  let message = $state('')
  let builtin: PresetName = $state(untrack(() => deviceClass) === 'phone' ? 'phone' : 'classic')
  let savedPresets: api.LayoutPreset[] = $state([])
  let activePresetName = $state('')
  let newPresetName = $state('')
  let showPresetManager = $state(false)

  function flash(text: string) {
    message = text
    setTimeout(() => message === text && (message = ''), 2500)
  }

  function renderer(name: string): IContentRenderer {
    const element = document.createElement('div')
    element.className = 'panel-host'
    let instance: ReturnType<typeof mount> | undefined
    return {
      element,
      init() {
        instance = mount(components[name].component, { target: element, props: { recipe } })
      },
      dispose() {
        if (instance) unmount(instance)
      },
    }
  }

  function loadBuiltin(name: PresetName) {
    if (!dvApi) return
    dvApi.clear()
    applyPreset(dvApi, name, (id) => components[id].title)
    builtin = name
  }

  async function refreshPresets() {
    try {
      const { items } = await api.listLayoutPresets(deviceClass)
      savedPresets = items
    } catch {
      savedPresets = []
    }
  }

  async function save() {
    if (!dvApi) return
    const layout = dvApi.toJSON() as unknown as Record<string, unknown>
    if (recipeId !== undefined) {
      try {
        await api.saveRecipeLayout(recipeId, deviceClass, layout)
        flash('Layout saved')
        return
      } catch {
        flash('Could not save layout')
        return
      }
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout))
    flash('Layout saved')
  }

  async function restore() {
    if (!dvApi) return
    if (recipeId !== undefined) {
      try {
        const { layout, source } = await api.getRecipeLayout(recipeId, deviceClass)
        dvApi.clear()
        dvApi.fromJSON(layout as never)
        flash(source === 'override' ? 'Layout restored' : `Restored preset "${activePresetName}"`)
        return
      } catch (err) {
        if (api.isNotFound(err)) {
          flash('Nothing saved yet')
        } else {
          flash('Could not load layout')
        }
        return
      }
    }
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return flash('Nothing saved yet')
    dvApi.clear()
    dvApi.fromJSON(JSON.parse(raw))
    flash('Layout restored')
  }

  async function applySavedPreset(name: string) {
    if (!dvApi) return
    const found = savedPresets.find((p) => p.name === name)
    if (!found) return
    dvApi.clear()
    dvApi.fromJSON(JSON.parse(found.layout))
    activePresetName = name
    try {
      await api.setActivePreset(deviceClass, name)
    } catch {
      // Non-fatal: the layout still applied locally.
    }
    flash(`Applied preset "${name}"`)
  }

  async function saveAsPreset() {
    if (!dvApi || !newPresetName.trim()) return
    const name = newPresetName.trim()
    const layout = dvApi.toJSON() as unknown as Record<string, unknown>
    try {
      await api.saveLayoutPreset(name, deviceClass, layout)
      newPresetName = ''
      await refreshPresets()
      flash(`Saved preset "${name}"`)
    } catch {
      flash('Could not save preset')
    }
  }

  async function removePreset(name: string) {
    try {
      await api.deleteLayoutPreset(name, deviceClass)
      await refreshPresets()
      flash(`Deleted preset "${name}"`)
    } catch {
      flash('Could not delete preset')
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (handleWorkspaceKeydown(e, dvApi)) e.preventDefault()
  }

  onMount(() => {
    const dv = createDockview(host, {
      theme: { name: 'cookarr', className: 'dockview-spaced', gap: 8 },
      createComponent: ({ name }) => renderer(name),
      defaultTabComponent: 'cookarr',
      createTabComponent: () => createTab(() => dvApi),
    })
    const sync = () => {
      activePanel = dv.activePanel?.title ?? ''
      groupCount = dv.groups.length
    }
    dv.onDidActivePanelChange(sync)
    dv.onDidLayoutChange(sync)
    dvApi = dv
    loadBuiltin(deviceClass === 'phone' ? 'phone' : 'classic')
    sync()
    void refreshPresets()
    window.addEventListener('keydown', onKeydown)
    return () => {
      window.removeEventListener('keydown', onKeydown)
      dv.dispose()
    }
  })

  onDestroy(() => window.removeEventListener('keydown', onKeydown))
</script>

<div class="shell" class:phone={deviceClass === 'phone'} class:tablet={deviceClass === 'tablet'}>
  <header>
    <div class="brand">
      <Illustration name="whisk" size={34} />
      <strong>cookarr</strong>
    </div>
    <div class="toolbar">
      <label>
        Layout
        <select bind:value={builtin} onchange={() => loadBuiltin(builtin)}>
          {#each Object.keys(presets) as p}<option value={p}>{p}</option>{/each}
        </select>
      </label>
      <button onclick={save}>Save</button>
      <button onclick={restore}>Restore</button>
      <button onclick={() => (showPresetManager = !showPresetManager)}>Presets</button>
      <button onclick={toggleTheme}>{$currentTheme === 'dark' ? 'Light' : 'Dark'}</button>
    </div>
  </header>

  {#if showPresetManager}
    <div class="preset-manager">
      <div class="preset-list">
        {#each savedPresets as p (p.name)}
          <span class="preset-chip" class:active={p.name === activePresetName}>
            <button onclick={() => applySavedPreset(p.name)}>{p.name}</button>
            <button class="preset-remove" aria-label="Delete preset" onclick={() => removePreset(p.name)}>&times;</button>
          </span>
        {:else}
          <span class="preset-empty">No saved presets for {deviceClass} yet.</span>
        {/each}
      </div>
      <div class="preset-new">
        <input placeholder="New preset name" bind:value={newPresetName} />
        <button onclick={saveAsPreset}>Save current as preset</button>
      </div>
    </div>
  {/if}

  <div class="dock" bind:this={host}></div>

  <footer>
    <span class="mode">{recipe.title}</span>
    <span>focus: {activePanel || '–'}</span>
    <span>{groupCount} group{groupCount === 1 ? '' : 's'}</span>
    <span class="flash">{message}</span>
    <span class="spacer"></span>
    {#if deviceClass === 'desktop'}
      <span class="hint">drag tabs to split · ⧉ floats a tab · alt+h/l focus · alt+\\ split · alt+w close</span>
    {/if}
  </footer>
</div>

<style>
  .shell {
    display: grid;
    grid-template-rows: auto 1fr auto;
    height: 100%;
    min-height: 0;
  }
  header,
  footer {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.5rem 1rem;
  }
  header {
    justify-content: space-between;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 1.15rem;
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .toolbar label {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .dock {
    min-height: 0;
  }
  footer {
    font-size: 0.8rem;
    color: var(--text-dim);
    border-top: 1px solid var(--border);
    background: var(--surface);
  }
  .mode {
    background: var(--accent);
    color: var(--on-accent);
    padding: 0.1rem 0.7rem;
    border-radius: 999px;
    font-weight: 700;
  }
  .flash {
    color: var(--accent-2);
  }
  .spacer {
    flex: 1;
  }
  .preset-manager {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 1rem;
    border-bottom: 1px solid var(--border);
    background: var(--surface);
  }
  .preset-list {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
  }
  .preset-chip {
    display: inline-flex;
    align-items: center;
    border: 1px solid var(--border);
    border-radius: 999px;
    overflow: hidden;
  }
  .preset-chip.active {
    border-color: var(--accent);
  }
  .preset-chip button {
    border: none;
    background: transparent;
    padding: 0.2rem 0.6rem;
  }
  .preset-remove {
    color: var(--danger, #d4553a);
  }
  .preset-empty {
    color: var(--text-dim);
    font-size: 0.85rem;
  }
  .preset-new {
    display: flex;
    gap: 0.4rem;
  }

  /* Phone/tablet: a cohesive single-column experience. The workspace itself
     stays Dockview-driven (so saved layouts still round-trip), but chrome
     around it gets out of the way per docs/ROADMAP.md "cohesive experience
     for phone/tablet". */
  .shell.phone header,
  .shell.tablet header {
    flex-wrap: wrap;
    row-gap: 0.4rem;
  }
  .shell.phone .toolbar label,
  .shell.phone .hint {
    display: none;
  }
  .shell.phone footer {
    font-size: 0.72rem;
    gap: 0.5rem;
  }
</style>
