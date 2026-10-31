<script lang="ts">
  import type { Recipe } from '../types'

  let { recipe }: { recipe: Recipe } = $props()

  const links = $derived(recipe.author?.links ?? [])
</script>

<div class="panel">
  {#if recipe.author}
    <h3>
      {#if recipe.author.url}<a href={recipe.author.url} target="_blank" rel="noreferrer">{recipe.author.name}</a>
      {:else}{recipe.author.name}{/if}
    </h3>
  {/if}
  <p class="source">
    From <a href={recipe.source_url} target="_blank" rel="noreferrer">{recipe.site_name ?? recipe.source_url}</a>
  </p>
  <div class="pills">
    {#each links as l}
      <a class="pill {l.kind}" href={l.url} target="_blank" rel="noreferrer">{l.label ?? l.url}</a>
    {/each}
  </div>
</div>

<style>
  h3 {
    margin: 0 0 0.25rem;
  }
  .source {
    margin: 0 0 1rem;
    color: var(--text-dim);
  }
  .pills {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .pill {
    padding: 0.25rem 0.8rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: var(--surface-2);
    text-decoration: none;
    font-size: 0.85rem;
  }
  .pill.support {
    border-color: var(--accent-2);
    color: var(--accent-2);
  }
</style>
