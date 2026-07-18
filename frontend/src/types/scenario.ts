// Frontend-only scenario metadata. These fields enrich the analyst's framing of
// a shock (name, hypothesis, qualitative assumptions) but are NOT sent to the
// backend — current propagation uses only source, commodity, shock type, drop
// and depth. They are captured at run time and shown in the result header.

export type ShockMode = 'preset' | 'custom'

export type Duration = '7 days' | '30 days' | '90 days' | '180 days'
export type RecoverySpeed = 'Fast' | 'Moderate' | 'Slow'
export type SubstituteAvailability = 'Low' | 'Medium' | 'High'
export type InventoryBuffer = '0 days' | '30 days' | '60 days' | '90 days'

export interface ScenarioAssumptions {
  duration: Duration
  recovery: RecoverySpeed
  substitute: SubstituteAvailability
  inventory: InventoryBuffer
}

export interface ScenarioMeta {
  name: string
  notes: string
  assumptions: ScenarioAssumptions
}

// Snapshot of the scenario as it was actually submitted to the backend. Captured
// at run time so the result header reflects the real shock, not stale preset text
// the analyst may have edited away from.
export interface SubmittedScenario {
  title: string
  mode: ShockMode
  modifiedPreset: boolean
  meta: ScenarioMeta
}

export const DURATION_OPTIONS: Duration[] = ['7 days', '30 days', '90 days', '180 days']
export const RECOVERY_OPTIONS: RecoverySpeed[] = ['Fast', 'Moderate', 'Slow']
export const SUBSTITUTE_OPTIONS: SubstituteAvailability[] = ['Low', 'Medium', 'High']
export const INVENTORY_OPTIONS: InventoryBuffer[] = ['0 days', '30 days', '60 days', '90 days']

export const DEFAULT_ASSUMPTIONS: ScenarioAssumptions = {
  duration: '30 days',
  recovery: 'Moderate',
  substitute: 'Low',
  inventory: '30 days',
}

export const DEFAULT_SCENARIO_NAME = 'Custom Shock Scenario'

export const DEFAULT_META: ScenarioMeta = {
  name: DEFAULT_SCENARIO_NAME,
  notes: '',
  assumptions: { ...DEFAULT_ASSUMPTIONS },
}

export const ASSUMPTION_NOTE =
  'Operational assumptions adjust interpreted impact after baseline graph propagation. Sectors are more sensitive to substitutes; countries are more sensitive to inventory and recovery.'

export function operationalRequestFields(assumptions: ScenarioAssumptions) {
  return {
    duration_days: Number.parseInt(assumptions.duration, 10),
    recovery_speed: assumptions.recovery,
    substitute_availability: assumptions.substitute,
    inventory_buffer_days: Number.parseInt(assumptions.inventory, 10),
  }
}
