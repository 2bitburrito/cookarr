<script lang="ts">
  import type { Recipe } from '../types'

  let { recipe }: { recipe: Recipe } = $props()
</script>

<ol class="panel">
  {#each recipe.steps ?? [] as step}
    <li>{step.text}</li>
  {/each}
</ol>

<style>
  ol {
    list-style: none;
    counter-reset: step;
    margin: 0;
    padding: 0.9rem 1rem;
    display: grid;
    gap: 0.9rem;
  }
  li {
    counter-increment: step;
    display: grid;
    grid-template-columns: 2rem 1fr;
    gap: 0.75rem;
    line-height: 1.55;
  }
  li::before {
    content: counter(step);
    display: grid;
    place-items: center;
    width: 2rem;
    height: 2rem;
    border-radius: 999px;
    background: var(--accent);
    color: var(--on-accent);
    font-weight: 700;
    font-size: 0.85rem;
  }
</style>
