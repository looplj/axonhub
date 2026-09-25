let lastInteraction = 0;

if (typeof window !== 'undefined') {
  for (const event of ['pointerdown', 'keydown']) {
    window.addEventListener(event, () => {
      lastInteraction = Date.now();
    }, { passive: true });
  }
}

export function wasRecentlyActive(): boolean {
  return lastInteraction > 0 && Date.now() - lastInteraction < 5 * 60 * 1000;
}
