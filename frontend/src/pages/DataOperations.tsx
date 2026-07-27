import type {
  DBHealthResponse,
  DBSummaryResponse,
  PipelineRunSummary,
} from '../types/api'
import { DataQualityCenter } from '../components/DataQualityCenter'
import { DataPipelineMonitor } from '../components/DataPipelineMonitor'

export function DataOperationsPage({
  dbHealth,
  dbSummary,
  dbLoading,
  dbErr,
  pipelineSummary,
  pipelineLoading,
  pipelineErr,
}: {
  dbHealth: DBHealthResponse | null
  dbSummary: DBSummaryResponse | null
  dbLoading: boolean
  dbErr?: { message: string; hint?: string } | null
  pipelineSummary: PipelineRunSummary | null
  pipelineLoading: boolean
  pipelineErr?: { message: string; hint?: string } | null
}) {
  return (
    <div className="space-y-4">
      <DataQualityCenter
        health={dbHealth}
        summary={dbSummary}
        loading={dbLoading}
        error={dbErr}
      />

      <DataPipelineMonitor
        summary={pipelineSummary}
        loading={pipelineLoading}
        error={pipelineErr}
      />
    </div>
  )
}
