<script lang="ts">
  import Illustration, { type IllustrationName } from '../Illustration.svelte'
  import type { Recipe } from '../types'

  let { recipe }: { recipe: Recipe } = $props()

  // Photos without a URL (fixture data, or not yet downloaded) fall back to a cartoon.
  const fallbacks: IllustrationName[] = ['pot', 'pan', 'blender', 'toaster']
</script>

<div class="panel grid">
  {#each recipe.photos ?? [] as photo, i}
    <figure>
      {#if photo.url}
        <img src={photo.url} alt={photo.caption ?? ''} loading="lazy" />
      {:else}
        <div class="placeholder"><Illustration name={fallbacks[i % fallbacks.length]} size={88} /></div>
      {/if}
      {#if photo.caption}<figcaption>{photo.caption}</figcaption>{/if}
    </figure>
  {/each}
</div>

<style>
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 0.75rem;
  }
  figure {
    margin: 0;
  }
  img,
  .placeholder {
    width: 100%;
    aspect-ratio: 4 / 3;
    object-fit: cover;
    border-radius: var(--radius-sm);
    border: 1px solid var(--border);
  }
  .placeholder {
    display: grid;
    place-items: center;
    background: var(--surface-2);
  }
  figcaption {
    margin-top: 0.3rem;
    font-size: 0.8rem;
    color: var(--text-dim);
  }
</style>
