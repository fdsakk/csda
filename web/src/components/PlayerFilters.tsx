import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { countActiveFilters, DEFAULT_FILTERS, StatusFilter, TableFilters } from '@/lib/filters';
import { number } from '@/lib/format';
import { cn } from '@/lib/utils';

// Threshold select; 0 always means "any".
function ThresholdSelect({
  label,
  value,
  options,
  format,
  onChange,
}: {
  label: string;
  value: number;
  options: number[];
  format: (value: number) => string;
  onChange: (value: number) => void;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-xs text-muted-foreground">{label}</label>
      <Select value={String(value)} onValueChange={(next) => onChange(Number(next))}>
        <SelectTrigger className={cn('h-8 w-full', value !== 0 && 'border-primary/50')}><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="0">Any</SelectItem>
          {options.map((option) => (
            <SelectItem key={option} value={String(option)}>{format(option)}</SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All statuses' },
  { value: 'flagged', label: 'Flagged only' },
  { value: 'cheater', label: 'Cheater' },
  { value: 'watch', label: 'Watch' },
  { value: 'normal', label: 'Normal' },
  { value: 'insufficient_sample', label: 'Low sample' },
];

function FilterCheckbox({ label, checked, onChange }: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm">
      <input type="checkbox" className="size-4 accent-primary" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  );
}

// Inline filter panel: plain selects in the page flow, so nothing floats,
// flips or locks scrolling.
export function FiltersPanel({ filters, onChange }: { filters: TableFilters; onChange: (filters: TableFilters) => void }) {
  const set = <K extends keyof TableFilters>(key: K, value: TableFilters[K]) => onChange({ ...filters, [key]: value });
  const active = countActiveFilters(filters);
  return (
    <Card className="gap-4 p-4">
      <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <div className="flex flex-col gap-1">
          <label className="text-xs text-muted-foreground">Status</label>
          <Select value={filters.status} onValueChange={(value) => set('status', value as StatusFilter)}>
            <SelectTrigger className={cn('h-8 w-full', filters.status !== 'all' && 'border-primary/50')}><SelectValue /></SelectTrigger>
            <SelectContent>
              {STATUS_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <ThresholdSelect label="Min demos" value={filters.minDemos} options={[2, 3, 5, 10]} format={(v) => `${v}+`} onChange={(v) => set('minDemos', v)} />
        <ThresholdSelect label="Min shots" value={filters.minShots} options={[100, 500, 1000, 3000]} format={(v) => `${number.format(v)}+`} onChange={(v) => set('minShots', v)} />
        <ThresholdSelect label="Accuracy" value={filters.minAccuracy} options={[0.15, 0.2, 0.25, 0.3]} format={(v) => `≥ ${Math.round(v * 100)}%`} onChange={(v) => set('minAccuracy', v)} />
        <ThresholdSelect label="Head hit rate" value={filters.minHeadHit} options={[0.2, 0.3, 0.4, 0.5]} format={(v) => `≥ ${Math.round(v * 100)}%`} onChange={(v) => set('minHeadHit', v)} />
        <ThresholdSelect label="HS kill rate" value={filters.minHsKill} options={[0.4, 0.6, 0.8]} format={(v) => `≥ ${Math.round(v * 100)}%`} onChange={(v) => set('minHsKill', v)} />
        <ThresholdSelect label="TTD (rifle)" value={filters.maxTtdMs} options={[450, 400, 350, 300]} format={(v) => `≤ ${v} ms`} onChange={(v) => set('maxTtdMs', v)} />
        <ThresholdSelect label="Reaction (rifle)" value={filters.maxReactionMs} options={[350, 300, 250, 200]} format={(v) => `≤ ${v} ms`} onChange={(v) => set('maxReactionMs', v)} />
        <ThresholdSelect label="K/D" value={filters.minKd} options={[1, 1.5, 2, 3]} format={(v) => `≥ ${v}`} onChange={(v) => set('minKd', v)} />
      </div>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-4">
          <FilterCheckbox label="Saved only" checked={filters.savedOnly} onChange={(value) => set('savedOnly', value)} />
          <FilterCheckbox label="Banned only" checked={filters.bannedOnly} onChange={(value) => set('bannedOnly', value)} />
          <FilterCheckbox label="Hide banned" checked={filters.hideBanned} onChange={(value) => set('hideBanned', value)} />
        </div>
        <Button variant="ghost" size="sm" disabled={!active} onClick={() => onChange(DEFAULT_FILTERS)}>Reset filters</Button>
      </div>
    </Card>
  );
}
