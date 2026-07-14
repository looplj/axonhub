function copyTextWithDocumentCommand(text: string): boolean {
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.left = '-9999px';
  textarea.style.opacity = '0';

  document.body.appendChild(textarea);
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);

  try {
    // Clipboard API is unavailable on non-secure origins; this is the only broadly supported fallback.
    return document.execCommand('copy');
  } finally {
    textarea.remove();
  }
}

export async function copyTextToClipboard(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }

  if (!copyTextWithDocumentCommand(text)) {
    throw new Error('Clipboard API is unavailable and the compatibility copy command failed.');
  }
}
