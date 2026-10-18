// Random 4-digit confirmation code shown in the channel delete dialog. A fresh
// code is generated every time the dialog opens, so a code from a previous
// attempt can never authorise a later delete.
export const CONFIRMATION_CODE_LENGTH = 4;

// Inclusive lower bound keeps the code exactly 4 digits (no leading zeros), so
// the displayed code and the typed value always have the same length.
const MIN_CONFIRMATION_CODE = 1000;
const MAX_CONFIRMATION_CODE = 9999;

export function generateConfirmationCode(random: () => number = Math.random): string {
  const span = MAX_CONFIRMATION_CODE - MIN_CONFIRMATION_CODE + 1;
  const value = MIN_CONFIRMATION_CODE + Math.floor(random() * span);

  return value.toString();
}

// The confirm button stays disabled until the typed value matches the code
// exactly. Comparison is on the trimmed value so stray whitespace is tolerated,
// and an empty generated code (dialog not yet opened) never matches.
export function isConfirmationCodeMatched(input: string, code: string): boolean {
  return code.length > 0 && input.trim() === code;
}
