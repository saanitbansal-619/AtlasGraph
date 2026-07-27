export function PageHeader({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <div className="mb-4">
      <h2 className="text-lg font-semibold tracking-tight text-slate-50 sm:text-xl">
        {title}
      </h2>
      <p className="mt-1 max-w-3xl text-xs leading-relaxed text-slate-400 sm:text-sm">
        {description}
      </p>
    </div>
  )
}
