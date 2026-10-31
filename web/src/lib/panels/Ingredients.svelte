<script lang="ts">
  import type { Recipe } from '../types'

  let { recipe }: { recipe: Recipe } = $props()

  const groups = $derived.by(() => {
    const out: { name: string; items: string[] }[] = []
    for (const ing of recipe.ingredients ?? []) {
      const name = ing.group ?? ''
      let g = out.find((x) => x.name === name)
      if (!g) out.push((g = { name, items: [] }))
      g.items.push(ing.text)
    }
    return out
  })
</script>

<div class="panel">
  {#if recipe.yield}<p class="meta">Makes {recipe.yield}</p>{/if}
  {#each groups as g}
    {#if g.name}<h3>{g.name}</h3>{/if}
    <ul>
      {#each g.items as item}
        <li><label><input type="checkbox" /> <span>{item}</span></label></li>
      {/each}
    </ul>
  {/each}
</div>

<style>
  .meta {
    color: var(--text-dim);
    margin: 0 0 0.75rem;
  }
  ul {
    list-style: none;
    padding: 0;
    margin: 0 0 1rem;
  }
  li {
    padding: 0.35rem 0;
    border-bottom: 1px dashed var(--border);
  }
  label {
    display: flex;
    gap: 0.6rem;
    align-items: baseline;
    cursor: pointer;
  }
  input:checked + span {
    color: var(--text-dim);
    text-decoration: line-through;
  }
  input {
    accent-color: var(--accent);
  }
</style>
