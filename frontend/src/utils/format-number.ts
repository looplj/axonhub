export function formatNumber(value: number | null | undefined, options?: { digits?: number }) {
  if (value == null || Number.isNaN(value)) {
    return '0'
  }

  const digits = options?.digits ?? 1
  const absolute = Math.abs(value)

  const formatWithSuffix = (divisor: number, suffix: string) => {
    const raw = (value / divisor).toFixed(digits)
    const trimmed = raw.replace(/\.0+$|(?<=\.\d*[1-9])0+$/g, '').replace(/\.$/, '')
    return `${trimmed}${suffix}`
  }

  if (absolute >= 1_000_000_000) {
    return formatWithSuffix(1_000_000_000, 'B')
  }

  if (absolute >= 1_000_000) {
    return formatWithSuffix(1_000_000, 'M')
  }

  if (absolute >= 1_000) {
    return formatWithSuffix(1_000, 'K')
  }

  return value.toLocaleString()
}
