<script lang="ts">
  import Illustration from '../Illustration.svelte'
  import type { Recipe } from '../types'

  let { recipe }: { recipe: Recipe } = $props()
</script>

<article class="panel">
  <h2>{recipe.title}</h2>
  {#if recipe.description}<p class="lede">{recipe.description}</p>{/if}
  {#if recipe.story_html}
    <!-- story_html is sanitised server-side (bluemonday) before storage. -->
    <div class="story">{@html recipe.story_html}</div>
  {:else}
    <div class="empty">
      <Illustration name="pot" />
      <p>No story on this one, just the recipe.</p>
    </div>
  {/if}
</article>

<style>
  h2 {
    margin: 0 0 0.4rem;
    font-size: 1.5rem;
    font-family: var(--heading);
  }
  .lede {
    color: var(--accent-2);
    font-size: 1.05rem;
    margin: 0 0 1rem;
  }
  .story :global(p) {
    line-height: 1.7;
    max-width: 62ch;
  }
  .empty {
    display: grid;
    justify-items: center;
    gap: 0.5rem;
    padding: 2rem 0;
    color: var(--text-dim);
  }
</style>
