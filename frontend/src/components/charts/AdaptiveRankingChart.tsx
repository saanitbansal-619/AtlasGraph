import { signed } from '../../lib/format'
import { HorizontalBarChartCard, type HorizontalBarDatum } from './HorizontalBarChartCard'
import { ImpactRankingList, type RankingItem } from './ImpactRankingList'

const RANKING_THRESHOLD = 3

/** Use compact ranking list for 1–3 rows; horizontal bar chart for 4+. */
export function AdaptiveRankingChart({
  title,
  subtitle,
  data,
  valueLabel,
  valueDigits = 1,
  valueSuffix = ' Δ',
  emptyLabel = 'No data available.',
  topN = 8,
  rankingLimit = 5,
  forceRanking = false,
}: {
  title: string
  subtitle?: string
  data: HorizontalBarDatum[]
  valueLabel: string
  valueDigits?: number
  valueSuffix?: string
  emptyLabel?: string
  topN?: number
  rankingLimit?: number
  /** When true, always use ranking list (e.g. scenario comparison with ≤3 scenarios). */
  forceRanking?: boolean
}) {
  const items: RankingItem[] = data.map((d) => ({
    label: d.label,
    value: d.value,
    chip: d.meta?.risk || d.meta?.type,
  }))

  const useRanking = forceRanking || data.length <= RANKING_THRESHOLD

  if (useRanking) {
    return (
      <ImpactRankingList
        title={title}
        subtitle={subtitle}
        items={items}
        valueSuffix={valueSuffix}
        valueFormatter={(v) =>
          `${signed(v, valueDigits)}${valueSuffix.trim() ? ` ${valueSuffix.trim()}` : ''}`
        }
        limit={rankingLimit}
        emptyMessage={emptyLabel}
      />
    )
  }

  return (
    <HorizontalBarChartCard
      title={title}
      subtitle={subtitle}
      data={data}
      valueLabel={valueLabel}
      valueDigits={valueDigits}
      emptyLabel={emptyLabel}
      topN={topN}
      showValueLabels
      valueSuffix={valueSuffix}
    />
  )
}
