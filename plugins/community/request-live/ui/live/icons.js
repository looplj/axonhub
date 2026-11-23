const MARKS = {
  claude: "C",
  codex: "X",
  cursor: "C",
  gemini: "G",
  copilot: "P",
  opencode: "O",
  pi: "π",
  crush: "H",
  goose: "G",
  cline: "L",
  zed: "Z",
  kimi: "K",
  grok: "K",
  droid: "F",
  curl: ">",
  playground: "P",
  test: "T",
  unknown: "?",
};

function escapeXml(text) {
  return String(text).replace(/[&<>"']/g, (char) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&apos;",
  }[char]));
}

function frame(inner) {
  return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden="true"><rect x="2.5" y="2.5" width="19" height="19" rx="6" stroke="currentColor" stroke-width="1.5"/>${inner}</svg>`;
}

function monogram(letter) {
  const safe = escapeXml(Array.from(String(letter || "?"))[0] || "?");
  return frame(`<text x="12" y="16.2" text-anchor="middle" font-size="11" font-family="ui-sans-serif, system-ui, sans-serif" fill="currentColor">${safe}</text>`);
}

export function iconSvg(clientId) {
  if (Object.prototype.hasOwnProperty.call(MARKS, clientId)) return monogram(MARKS[clientId]);
  const letter = Array.from(String(clientId || "?").trim())[0] || "?";
  return monogram(letter.toUpperCase());
}
