/**
 * Available font families, grouped by category.
 * - `system`: use the OS default font (ui-sans-serif / ui-serif / ui-monospace), independent of theme.
 * - `theme`: follow the active theme's default font (no override).
 *
 * 📝 How to Add a New Font (Tailwind v4+):
 * 1. Add the font name to the matching list below.
 * 2. Update the `<link>` tag in 'index.html' to include the new font from Google Fonts (or any other source).
 * 3. Add the corresponding font-family stack to `fontStacks`.
 */

/** Sans-serif options for the main UI font (--font-sans). */
export const sansFonts = ['inter', 'manrope', 'montserrat', 'open-sans', 'system', 'theme'] as const;

/** Serif options (--font-serif). */
export const serifFonts = ['source-serif-4', 'georgia', 'system', 'theme'] as const;

/** Monospace options (--font-mono). */
export const monoFonts = ['jetbrains-mono', 'fira-code', 'system', 'theme'] as const;

/** Font-family stack for each named font ('system' / 'theme' are handled separately). */
export const fontStacks: Record<string, string> = {
  inter: "'Inter', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif",
  manrope: "'Manrope', ui-sans-serif, system-ui, sans-serif",
  montserrat: "'Montserrat', ui-sans-serif, system-ui, sans-serif",
  'open-sans': "'Open Sans', ui-sans-serif, system-ui, sans-serif",
  'source-serif-4': "'Source Serif 4', ui-serif, Georgia, Cambria, 'Times New Roman', Times, serif",
  georgia: "Georgia, Cambria, 'Times New Roman', Times, serif",
  'jetbrains-mono': "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
  'fira-code': "'Fira Code', ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
};

/** OS default font stacks — the 'system' option maps to these (not affected by theme). */
export const systemStacks: Record<'sans' | 'serif' | 'mono', string> = {
  sans: "ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif",
  serif: "ui-serif, Georgia, Cambria, 'Times New Roman', Times, serif",
  mono: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
};

/** Human-readable label for each option. */
export const fontLabels: Record<string, string> = {
  inter: 'Inter',
  manrope: 'Manrope',
  system: 'System',
  theme: 'Theme',
  'jetbrains-mono': 'JetBrains Mono',
  'fira-code': 'Fira Code',
  montserrat: 'Montserrat',
  'open-sans': 'Open Sans',
  'source-serif-4': 'Source Serif 4',
  georgia: 'Georgia',
};
