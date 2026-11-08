// Minimal fetch wrapper for the Cookarr API. Kept framework-free so it's easy
// to swap/mock in tests.
import type { Recipe } from './types'

export type DeviceClass = 'desktop' | 'tablet' | 'phone'

class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    credentials: 'same-origin',
  })
  if (res.status === 204) return undefined as T
  if (!res.ok) {
    let message = res.statusText
    try {
      const err = (await res.json()) as { error?: { message?: string } }
      message = err.error?.message ?? message
    } catch {
      // Body wasn't JSON; fall back to statusText.
    }
    throw new ApiError(res.status, message)
  }
  return (await res.json()) as T
}

export function isNotFound(err: unknown): boolean {
  return err instanceof ApiError && err.status === 404
}

export function getRecipe(id: number): Promise<Recipe & { id: number }> {
  return request('GET', `/recipes/${id}`)
}

export interface LayoutResponse {
  source: 'override' | 'preset'
  layout: Record<string, unknown>
}

export function getRecipeLayout(id: number, deviceClass: DeviceClass): Promise<LayoutResponse> {
  return request('GET', `/recipes/${id}/layout?device_class=${deviceClass}`)
}

export function saveRecipeLayout(id: number, deviceClass: DeviceClass, layout: Record<string, unknown>): Promise<void> {
  return request('PUT', `/recipes/${id}/layout?device_class=${deviceClass}`, { layout })
}

export function clearRecipeLayout(id: number, deviceClass: DeviceClass): Promise<void> {
  return request('DELETE', `/recipes/${id}/layout?device_class=${deviceClass}`)
}

export interface LayoutPreset {
  name: string
  device_class: string
  layout: string
  updated_at: string
}

export function listLayoutPresets(deviceClass: DeviceClass): Promise<{ items: LayoutPreset[] }> {
  return request('GET', `/layout/presets?device_class=${deviceClass}`)
}

export function saveLayoutPreset(name: string, deviceClass: DeviceClass, layout: Record<string, unknown>): Promise<void> {
  return request('PUT', `/layout/presets/${encodeURIComponent(name)}?device_class=${deviceClass}`, { layout })
}

export function deleteLayoutPreset(name: string, deviceClass: DeviceClass): Promise<void> {
  return request('DELETE', `/layout/presets/${encodeURIComponent(name)}?device_class=${deviceClass}`)
}

export function getActivePreset(deviceClass: DeviceClass): Promise<{ preset: string }> {
  return request('GET', `/layout/active?device_class=${deviceClass}`)
}

export function setActivePreset(deviceClass: DeviceClass, preset: string): Promise<void> {
  return request('PUT', `/layout/active?device_class=${deviceClass}`, { preset })
}
