/** Vertical chart height tuned to row count — avoids giant bars for tiny datasets. */
export function chartHeightForRows(rowCount: number): number {
  const n = Math.max(0, rowCount)
  if (n === 0) return 72
  if (n === 1) return 68
  if (n === 2) return 92
  if (n === 3) return 118
  if (n <= 5) return 36 + n * 26
  return 36 + Math.min(n, 8) * 24
}

export function sortByValueDesc<T extends { value: number }>(items: T[]): T[] {
  return [...items].sort((a, b) => b.value - a.value)
}

export function clampItems<T>(items: T[], limit: number): T[] {
  return items.length > limit ? items.slice(0, limit) : items
}
