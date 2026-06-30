import { BACKEND_COMMAND } from '../lib/api'

// CommandBlock renders the backend start command in a copyable code block.
export function CommandBlock({ command }: { command: string }) {
  return (
    <pre className="mt-2 overflow-x-auto rounded border border-slate-800 bg-slate-950/80 p-3 text-[12px] leading-relaxed text-cyan-200">
      <code>{command}</code>
    </pre>
  )
}

// BackendDownNotice is shown when the API is unreachable, with the exact command
// to start it.
export function BackendDownNotice({
  message,
  onRetry,
}: {
  message?: string
  onRetry?: () => void
}) {
  return (
    <div className="panel border-rose-500/30 bg-rose-500/[0.06] p-5">
      <div className="flex items-center gap-2 text-sm font-semibold text-rose-300">
        <span className="inline-block h-2 w-2 rounded-full bg-rose-400" />
        API unavailable
      </div>
      <p className="mt-2 text-sm text-slate-300">
        {message || 'The AtlasGraph API server is not responding.'}
      </p>
      <p className="mt-3 text-xs uppercase tracking-wider text-slate-500">
        Start backend with:
      </p>
      <CommandBlock command={BACKEND_COMMAND} />
      {onRetry && (
        <button
          onClick={onRetry}
          className="mt-3 rounded border border-slate-700 px-3 py-1.5 text-xs font-semibold text-slate-200 transition hover:border-cyan-500/60 hover:text-cyan-200"
        >
          Retry connection
        </button>
      )}
    </div>
  )
}

// InlineError is a compact error strip for a single failed request.
export function InlineError({ message, hint }: { message: string; hint?: string }) {
  return (
    <div className="rounded border border-rose-500/30 bg-rose-500/[0.06] px-3 py-2 text-sm text-rose-200">
      <div className="font-semibold">{message}</div>
      {hint && <div className="mt-1 whitespace-pre-line text-xs text-rose-300/80">{hint}</div>}
    </div>
  )
}
