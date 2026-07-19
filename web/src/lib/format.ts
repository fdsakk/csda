export const number = new Intl.NumberFormat('en-US');

const SCORE_COLOR_STOPS = [
  { value: 0, color: [255, 255, 255] },
  { value: 34, color: [250, 204, 21] },
  { value: 67, color: [249, 115, 22] },
  { value: 100, color: [220, 38, 38] },
] as const;

/** Returns the continuous white → yellow → orange → red color for a 0–100 score. */
export function scoreColor(score: number) {
  const value = Math.max(0, Math.min(100, score));
  const endIndex = SCORE_COLOR_STOPS.findIndex((stop) => value <= stop.value);
  const end = SCORE_COLOR_STOPS[Math.max(1, endIndex)];
  const start = SCORE_COLOR_STOPS[Math.max(0, endIndex - 1)];
  const progress = (value - start.value) / (end.value - start.value);
  const [red, green, blue] = start.color.map((component, index) => Math.round(component + (end.color[index] - component) * progress));
  return `rgb(${red} ${green} ${blue})`;
}

export function pct(value: number) { return `${(value * 100).toFixed(1)}%`; }
export function ms(value: number, samples: number) { return samples ? `${Math.round(value)} ms` : '—'; }
