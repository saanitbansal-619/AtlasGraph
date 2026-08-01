export type AppTab =
  | 'dashboard'
  | 'shock'
  | 'client'
  | 'data-ops'
  | 'analytics'
  | 'history'

export const TAB_META: Record<
  AppTab,
  { title: string; description: string }
> = {
  dashboard: {
    title: 'Dashboard',
    description:
      'Analyst workflow: client portfolio → shock → dollars at risk → evidence-backed actions.',
  },
  shock: {
    title: 'Shock Simulation',
    description:
      'Configure shock parameters and operational assumptions, run propagation, and generate scenario intelligence reports.',
  },
  client: {
    title: 'Client Analytics',
    description:
      'Upload client supplier dependency data and review concentration, HHI, and exposure metrics.',
  },
  'data-ops': {
    title: 'Data Operations Monitor',
    description:
      'Monitor ETL pipeline health, validation checks, loaded analytics tables, and scalability path.',
  },
  analytics: {
    title: 'Analytics Explorer',
    description:
      'Browse planned analytical workspaces for commodities, countries, suppliers, and SQL-backed rankings.',
  },
  history: {
    title: 'History',
    description: 'Review previously saved scenario runs and analyst snapshots.',
  },
}
