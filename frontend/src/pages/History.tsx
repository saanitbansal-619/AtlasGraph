import { History } from 'lucide-react'
import { SectionCard } from '../components/shared/SectionCard'
import { EmptyHint } from '../components/ui'

export function HistoryPage() {
  return (
    <div className="space-y-4">
      <SectionCard title="Saved scenarios" dense>
        <EmptyHint>
          <div className="flex flex-col items-center gap-2">
            <History className="h-5 w-5 text-slate-600" strokeWidth={1.75} />
            <span>No saved scenarios yet.</span>
          </div>
        </EmptyHint>
      </SectionCard>
    </div>
  )
}
