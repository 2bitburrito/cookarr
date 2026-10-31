// The ESM entry ships no CSS; the UMD bundle injects it. Same API, so reuse the types.
declare module 'dockview-core/dist/dockview-core.js' {
  export * from 'dockview-core'
}
