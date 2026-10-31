<script lang="ts">
  import { createDockview, type DockviewApi, type IContentRenderer } from 'dockview-core/dist/dockview-core.js'
  import { mount, onMount, unmount, type Component } from 'svelte'
  import Illustration from './Illustration.svelte'
  import Author from './panels/Author.svelte'
  import Ingredients from './panels/Ingredients.svelte'
  import Photos from './panels/Photos.svelte'
  import Steps from './panels/Steps.svelte'
  import Story from './panels/Story.svelte'
  import { createTab } from './tab'
  import { applyPreset, presets, type PresetName } from './presets'
  import { currentTheme, toggleTheme } from './theme'
  import type { Recipe } from './types'

  let { recipe }: { recipe: Recipe } = $props()

  const STORAGE_KEY = 'cookarr:layout-spike'

  const components: Record<string, { component: Component<{ recipe: Recipe }>; title: string }> = {
    ingredients: { component: Ingredients, title: 'Ingredients' },
    steps: { component: Steps, title: 'Steps' },
    story: { component: Story, title: 'Story' },
    photos: { component: Photos, title: 'Photos' },
    author: { component: Author, title: 'Author' },
  }

  let host: HTMLDivElement
  let api: DockviewApi | undefined = $state()
  let activePanel = $state('')
  let groupCount = $state(0)
  let message = $state('')
  let preset: PresetName = $state('classic')

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

  function load(name: PresetName) {
    if (!api) return
    api.clear()
    applyPreset(api, name, (id) => components[id].title)
  }

  function save() {
    if (!api) return
    localStorage.setItem(STORAGE_KEY, JSON.stringify(api.toJSON()))
    flash('Layout saved')
  }

  function restore() {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw || !api) return flash('Nothing saved yet')
    api.clear()
    api.fromJSON(JSON.parse(raw))
    flash('Layout restored')
  }

  onMount(() => {
    const dv = createDockview(host, {
      theme: { name: 'cookarr', className: 'dockview-spaced', gap: 8 },
      createComponent: ({ name }) => renderer(name),
      defaultTabComponent: 'cookarr',
      createTabComponent: () => createTab(() => api),
    })
    const sync = () => {
      activePanel = dv.activePanel?.title ?? ''
      groupCount = dv.groups.length
    }
    dv.onDidActivePanelChange(sync)
    dv.onDidLayoutChange(sync)
    api = dv
    load('classic')
    sync()
    return () => dv.dispose()
  })
</script>

<div class="shell">
  <header>
    <div class="brand">
      <Illustration name="whisk" size={34} />
      <strong>cookarr</strong>
    </div>
    <div class="toolbar">
      <label>
        Layout
        <select bind:value={preset} onchange={() => load(preset)}>
          {#each Object.keys(presets) as p}<option value={p}>{p}</option>{/each}
        </select>
      </label>
      <button onclick={save}>Save</button>
      <button onclick={restore}>Restore</button>
      <button onclick={toggleTheme}>{$currentTheme === 'dark' ? 'Light' : 'Dark'}</button>
    </div>
  </header>

  <div class="dock" bind:this={host}></div>

  <footer>
    <span class="mode">{recipe.title}</span>
    <span>focus: {activePanel || '–'}</span>
    <span>{groupCount} group{groupCount === 1 ? '' : 's'}</span>
    <span class="flash">{message}</span>
    <span class="spacer"></span>
    <span class="hint">drag tabs to split · ⧉ floats a tab</span>
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
</style>
