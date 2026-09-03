export function parseOrderingWeightInput(rawValue: string, min: number, max: number): number | null {
  if (rawValue.trim() === '') {
    return null;
  }

  const value = Number(rawValue);
  if (!Number.isFinite(value) || !Number.isInteger(value) || value < min || value > max) {
    return null;
  }

  return value;
}
