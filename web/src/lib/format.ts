export const number = new Intl.NumberFormat('en-US');

export function pct(value: number) { return `${(value * 100).toFixed(1)}%`; }
export function ms(value: number, samples: number) { return samples ? `${Math.round(value)} ms` : '—'; }
