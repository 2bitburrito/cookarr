<script lang="ts" module>
  export type IllustrationName = 'pot' | 'pan' | 'toaster' | 'blender' | 'whisk'
</script>

<script lang="ts">
  // Placeholder cartoon appliances until real art exists. Colours come from
  // theme variables so the set follows the active theme.
  let { name, size = 96 }: { name: IllustrationName; size?: number } = $props()
</script>

<svg
  class="ill"
  width={size}
  height={size}
  viewBox="0 0 96 96"
  fill="none"
  stroke-width="3"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  {#if name === 'pot'}
    <path class="steam" d="M34 22c-4-5 4-8 0-14M48 22c-4-5 4-8 0-14M62 22c-4-5 4-8 0-14" />
    <path class="line fill-a" d="M20 34h56v30a16 16 0 0 1-16 16H36a16 16 0 0 1-16-16z" />
    <path class="line fill-b" d="M16 34h64a4 4 0 0 0 0-8H16a4 4 0 0 0 0 8z" />
    <path class="line" d="M20 46H8m68 0h12" />
    <circle class="eye" cx="40" cy="56" r="2.5" />
    <circle class="eye" cx="56" cy="56" r="2.5" />
    <path class="line" d="M43 63q5 5 10 0" />
  {:else if name === 'pan'}
    <path class="steam" d="M30 24c-4-5 4-8 0-14M44 24c-4-5 4-8 0-14" />
    <ellipse class="line fill-a" cx="40" cy="52" rx="30" ry="14" />
    <path class="line fill-b" d="M70 48l22-6a3 3 0 0 1 2 5l-22 9" />
    <circle class="eye" cx="32" cy="50" r="2.5" />
    <circle class="eye" cx="46" cy="50" r="2.5" />
    <path class="line" d="M34 57q5 4 10 0" />
  {:else if name === 'toaster'}
    <rect class="line fill-a" x="12" y="34" width="72" height="42" rx="14" />
    <path class="line fill-b" d="M26 34V24a4 4 0 0 1 4-4h12a4 4 0 0 1 4 4v10M50 34V24a4 4 0 0 1 4-4h12a4 4 0 0 1 4 4v10" />
    <path class="line" d="M84 50h6M84 62h6" />
    <circle class="eye" cx="34" cy="54" r="2.5" />
    <circle class="eye" cx="54" cy="54" r="2.5" />
    <path class="line" d="M38 61q6 5 12 0" />
    <path class="line" d="M24 82v4m48-4v4" />
  {:else if name === 'blender'}
    <path class="line fill-b" d="M28 10h40l-6 52H34z" />
    <path class="line fill-a" d="M24 62h48l4 22a4 4 0 0 1-4 4H24a4 4 0 0 1-4-4z" />
    <path class="line" d="M28 10h40M68 22h8v18h-8" />
    <circle class="eye" cx="42" cy="75" r="2.5" />
    <circle class="eye" cx="54" cy="75" r="2.5" />
    <path class="line" d="M44 81q4 3 8 0" />
    <path class="steam" d="M42 36q6 6 12 0M44 48q4 4 8 0" />
  {:else if name === 'whisk'}
    <path class="line fill-b" d="M48 6a8 8 0 0 1 8 8v22H40V14a8 8 0 0 1 8-8z" />
    <path class="line fill-a" d="M40 36c-14 8-20 22-12 44 4 8 14 8 20 8s16 0 20-8c8-22 2-36-12-44" />
    <path class="line" d="M48 36v52M44 38c-12 16-12 34 0 50M52 38c12 16 12 34 0 50" />
  {/if}
</svg>

<style>
  .ill {
    stroke: var(--ill-line, #191724);
    flex: none;
  }
  .ill :global(.fill-a) {
    fill: var(--ill-a, var(--accent-2));
  }
  .ill :global(.fill-b) {
    fill: var(--ill-b, var(--accent));
  }
  .ill :global(.eye) {
    fill: var(--ill-line, #191724);
    stroke: none;
  }
  .ill :global(.steam) {
    stroke: var(--text-dim);
    stroke-width: 2.5;
  }
</style>
