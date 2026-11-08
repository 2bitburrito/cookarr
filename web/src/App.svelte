<script lang="ts">
  import { onMount } from 'svelte'
  import { fixture } from './lib/fixture'
  import Workspace from './lib/Workspace.svelte'
  import { deviceClass } from './lib/device'
  import { getRecipe } from './lib/api'
  import type { Recipe } from './lib/types'

  // ?recipe=<id> loads a real recipe from the API; otherwise this renders
  // the local fixture, which is handy for frontend-only development.
  let recipe: Recipe = $state(fixture)
  let recipeId: number | undefined = $state(undefined)
  let loadError = $state('')

  onMount(async () => {
    const id = Number(new URLSearchParams(window.location.search).get('recipe'))
    if (!Number.isInteger(id) || id <= 0) return
    try {
      const loaded = await getRecipe(id)
      recipe = loaded
      recipeId = loaded.id
    } catch {
      loadError = `Could not load recipe ${id}; showing the sample recipe instead.`
    }
  })
</script>

{#if loadError}
  <div class="load-error">{loadError}</div>
{/if}
<Workspace {recipe} {recipeId} deviceClass={$deviceClass} />

<style>
  .load-error {
    padding: 0.4rem 1rem;
    font-size: 0.8rem;
    background: var(--danger, #d4553a);
    color: white;
  }
</style>
