import type { ShockResponse } from '../types/api'
import type { SubmittedScenario } from './scenario'

export type ShockWorkspaceTab = 'setup' | 'results' | 'comparison'

export interface SavedShockScenario {
  id: string
  savedAt: string
  title: string
  source: string
  commodity: string
  shock_type: string
  drop: number
  depth: number
  result: ShockResponse
  submitted?: SubmittedScenario | null
}

const STORAGE_KEY = 'gfip.savedShockScenarios.v1'

export function loadSavedScenarios(): SavedShockScenario[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as SavedShockScenario[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function persistSavedScenarios(items: SavedShockScenario[]): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(items))
  } catch {
    // Ignore quota / private-mode failures; in-memory state still works.
  }
}

export function createSavedScenario(input: {
  result: ShockResponse
  submitted?: SubmittedScenario | null
}): SavedShockScenario {
  const sc = input.result.scenario
  const title =
    input.submitted?.title?.trim() ||
    sc.name ||
    `${sc.source} · ${sc.commodity} · ${sc.shock_type}`
  return {
    id: `saved-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    savedAt: new Date().toISOString(),
    title,
    source: sc.source,
    commodity: sc.commodity,
    shock_type: sc.shock_type || input.result.shock_profile.type,
    drop: sc.shock_percent,
    depth: sc.depth,
    result: input.result,
    submitted: input.submitted ?? null,
  }
}
