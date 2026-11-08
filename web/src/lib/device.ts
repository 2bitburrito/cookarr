// Detects the workspace's device class from viewport width, per
// docs/PLAN.md: layouts are stored and resolved per class (desktop, tablet,
// phone). Thresholds are deliberately simple breakpoints, not UA sniffing.
import { readable } from 'svelte/store'
import type { DeviceClass } from './api'

const PHONE_MAX = 640
const TABLET_MAX = 1024

export function classifyWidth(width: number): DeviceClass {
  if (width <= PHONE_MAX) return 'phone'
  if (width <= TABLET_MAX) return 'tablet'
  return 'desktop'
}

export const deviceClass = readable<DeviceClass>(classifyWidth(window.innerWidth), (set) => {
  const onResize = () => set(classifyWidth(window.innerWidth))
  window.addEventListener('resize', onResize)
  return () => window.removeEventListener('resize', onResize)
})
