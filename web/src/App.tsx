import { DragEvent, Fragment, type ReactNode, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowDown, ArrowUp, Ban, BookOpen, Bookmark, ChevronDown, ChevronLeft, ChevronRight, ExternalLink, FileDown, FileUp, Film, ListFilter, Search, Trash2, Upload, X } from 'lucide-react';
import { deleteDemo, Demo, getJobs, getReport, importStats, Job, Player, PlayerWeapon, Report, Rule, setDemoEnabled, setPlayerBanned, setPlayerSaved, uploadDemos } from './api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';

type SortKey = 'status' | 'name' | 'demoCount' | 'shots' | 'kills' | 'deaths' | 'accuracy' | 'headHitRate' | 'headshotKillRate' | 'ttdWeightedMs' | 'reactionWeightedMs';
type StatusFilter = 'all' | 'flagged' | Player['status'];

const EMPTY_REPORT: Report = { players: [], playersByWeapon: [], evidence: [], importedDemos: [] };
const number = new Intl.NumberFormat('en-US');
const FLAGGED = ['watch', 'cheater'];
const STATUS_RANK: Record<Player['status'], number> = { cheater: 3, watch: 2, normal: 1, insufficient_sample: 0 };

const STATUS_LABEL: Record<Player['status'], string> = {
  normal: 'Normal',
  watch: 'Watch',
  cheater: 'Cheater',
  insufficient_sample: 'Low sample',
};

function pct(value: number) { return `${(value * 100).toFixed(1)}%`; }
function ms(value: number, samples: number) { return samples ? `${Math.round(value)} ms` : '—'; }

function statusVariant(status: Player['status']) {
  if (status === 'cheater') return 'destructive' as const;
  if (status === 'watch') return 'warning' as const;
  if (status === 'insufficient_sample') return 'outline' as const;
  return 'default' as const;
}

function Dropzone({ onQueued }: { onQueued: (job: Job) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState('');

  const accept = useCallback((incoming: File[]) => {
    const demos = incoming.filter((file) => /\.dem$/i.test(file.name));
    setError(demos.length === incoming.length ? '' : 'Only .dem files were added.');
    setFiles((current) => {
      const known = new Set(current.map((file) => `${file.name}:${file.size}`));
      return [...current, ...demos.filter((file) => !known.has(`${file.name}:${file.size}`))];
    });
  }, []);

  const drop = (event: DragEvent) => { event.preventDefault(); setDragging(false); accept([...event.dataTransfer.files]); };
  const submit = async () => {
    if (!files.length || progress !== null) return;
    setError(''); setProgress(0);
    try { const job = await uploadDemos(files, setProgress); setFiles([]); onQueued(job); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Upload failed'); }
    finally { setProgress(null); }
  };

  return (
    <div className="rounded-lg border border-border bg-card">
      <div
        className={cn(
          'flex flex-col items-center justify-center gap-2 rounded-t-lg border-b border-dashed border-border px-6 py-10 text-center transition-colors',
          dragging && 'border-ring bg-accent/50'
        )}
        onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={drop}
      >
        <input ref={input} type="file" accept=".dem" multiple hidden onChange={(event) => accept([...(event.target.files ?? [])])} />
        <div className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Upload className="size-5" />
        </div>
        <p className="text-sm font-medium">Drop CS2 demo files here</p>
        <p className="text-sm text-muted-foreground">.dem recordings — drag &amp; drop or</p>
        <Button variant="outline" size="sm" onClick={() => input.current?.click()}>Browse files</Button>
      </div>

      <div className="flex flex-col gap-3 border-t border-border p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-wrap gap-2">
          {files.length ? files.map((file) => (
            <span key={`${file.name}:${file.size}`} className="flex max-w-full items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
              <span className="truncate">{file.name}</span>
              <button className="shrink-0 text-muted-foreground hover:text-foreground" onClick={() => setFiles((all) => all.filter((item) => item !== file))}>
                <X className="size-3" />
              </button>
            </span>
          )) : <span className="text-sm text-muted-foreground">Source detected automatically</span>}
        </div>
        <Button className="shrink-0 self-end sm:self-auto" disabled={!files.length || progress !== null} onClick={submit}>
          {progress === null
            ? `Analyze ${files.length || ''} file${files.length === 1 ? '' : 's'}`.replace('  ', ' ')
            : `Uploading ${progress}%`}
        </Button>
      </div>

      {progress !== null ? (
        <div className="mx-4 mb-4 h-1 overflow-hidden rounded-full bg-muted">
          <div className="h-full bg-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
      ) : null}
      {error ? <p className="px-4 pb-4 text-sm text-destructive">{error}</p> : null}
    </div>
  );
}

function Jobs({ jobs }: { jobs: Job[] }) {
  const active = jobs.filter((job) => job.status === 'queued' || job.status === 'running');
  if (!active.length) return null;
  return (
    <div className="space-y-3">
      {active.map((job) => {
        const total = job.total || job.files.length || 1;
        const running = job.status === 'running';
        const percent = running ? Math.round(job.progress || (job.processed / total) * 100) : 0;
        return (
          <div key={job.id} className="rounded-lg border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="flex min-w-0 items-center gap-2">
                <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-warning" />
                <span className="truncate text-foreground">{job.files.join(', ')}</span>
              </span>
              <span className="shrink-0 tabular-nums text-muted-foreground">
                {running ? `Analyzing ${job.processed}/${total} · ${percent}%` : 'Queued'}
              </span>
            </div>
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className={cn('h-full bg-primary transition-all duration-300', !running && 'animate-pulse w-1/3')}
                style={running ? { width: `${percent}%` } : undefined}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}

const HIST_BIN_MS = 50;

function Histogram({
  title,
  bins,
  samples,
  medianMs,
  p10Ms,
  color,
}: {
  title: string;
  bins: number[] | null;
  samples: number;
  medianMs: number;
  p10Ms: number;
  color: string;
}) {
  const data = bins && bins.length ? bins : [];
  const max = Math.max(1, ...data);
  const width = 240;
  const height = 56;
  const barW = width / 20;
  const x = (msValue: number) => Math.min(width, (msValue / 1000) * width);
  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <div className="flex items-baseline justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs font-medium text-foreground">
          <span className="size-2 rounded-full" style={{ background: color }} />
          {title}
        </span>
        <span className="text-xs tabular-nums text-muted-foreground">
          {samples ? <>median <span className="font-medium text-foreground">{Math.round(medianMs)} ms</span> · p10 {Math.round(p10Ms)} ms · n={samples}</> : 'no samples'}
        </span>
      </div>
      <svg viewBox={`0 0 ${width} ${height + 14}`} className="mt-2 w-full" role="img" aria-label={`${title} distribution`}>
        <line x1="0" y1={height} x2={width} y2={height} stroke="var(--chart-grid)" strokeWidth="1" />
        {data.map((count, i) => {
          const h = count ? Math.max(2, (count / max) * (height - 6)) : 0;
          return count ? (
            <rect key={i} x={i * barW + 1} y={height - h} width={barW - 2} height={h} rx="1.5" fill={color}>
              <title>{`${i * HIST_BIN_MS}–${(i + 1) * HIST_BIN_MS} ms · ${count} encounter${count === 1 ? '' : 's'}`}</title>
            </rect>
          ) : null;
        })}
        {samples ? (
          <>
            <line x1={x(p10Ms)} y1="2" x2={x(p10Ms)} y2={height} stroke="var(--muted-foreground)" strokeWidth="1" strokeDasharray="3 2" />
            <line x1={x(medianMs)} y1="0" x2={x(medianMs)} y2={height} stroke="var(--foreground)" strokeWidth="1.5" />
          </>
        ) : null}
        {[0, 250, 500, 750, 1000].map((tick) => (
          <text key={tick} x={tick === 0 ? 1 : tick === 1000 ? width - 1 : x(tick)} y={height + 11} fontSize="8.5" fill="var(--muted-foreground)" textAnchor={tick === 0 ? 'start' : tick === 1000 ? 'end' : 'middle'}>
            {tick}
          </text>
        ))}
      </svg>
    </div>
  );
}

function BarRow({ label, detail, value, color, reference }: { label: string; detail: string; value: number; color: string; reference?: number }) {
  return (
    <div className="grid grid-cols-[7rem_1fr_3rem] items-center gap-2 text-xs">
      <span className="truncate text-[13px] text-foreground" title={label}>{label}</span>
      <div className="relative h-2.5 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, value * 100)}%`, background: color }} />
        {reference !== undefined ? (
          <div
            className="absolute inset-y-0 w-px bg-foreground/60"
            style={{ left: `${Math.min(100, reference * 100)}%` }}
            title={`overall ${pct(reference)}`}
          />
        ) : null}
      </div>
      <span className="text-right tabular-nums text-muted-foreground" title={detail}>{pct(value)}</span>
    </div>
  );
}

const RULE_LABEL: Record<string, string> = {
  ttd_impossible: 'Time-to-damage impossibly low',
  ttd_low_elite_stats: 'Low TTD + elite fragging',
  ttd_low: 'Low time-to-damage',
  reaction_impossible: 'Reaction below human floor',
  head_hit_rate: 'Head hit rate',
  smoke_wall_kills: 'Smoke / wall kills',
  unspotted_damage: 'Unspotted damage',
};

function formatRuleValue(rule: Rule) {
  if (rule.name.startsWith('ttd') || rule.name.startsWith('reaction')) return `${Math.round(rule.value)} ms`;
  if (rule.name === 'smoke_wall_kills') return `${Math.round(rule.value)} kills`;
  return `${(rule.value * 100).toFixed(1)}%`;
}

function PlayerDetails({ player, weapons }: { player: Player; weapons: PlayerWeapon[] }) {
  const rules = player.triggeredRules ?? [];
  const flagSignals = rules.filter((rule) => rule.tier === 'watch' || rule.tier === 'cheater');
  const infoSignals = rules.filter((rule) => rule.tier === 'info');
  const accuracyWeapons = weapons
    .filter((weapon) => weapon.weaponName && weapon.shots >= 10)
    .toSorted((a, b) => b.shots - a.shots)
    .slice(0, 6);
  const killWeapons = weapons
    .filter((weapon) => weapon.weaponName && weapon.kills > 0)
    .toSorted((a, b) => b.kills - a.kills);
  const situational: { label: string; shots: number; rate: number }[] = [
    { label: 'Moving', shots: player.movingShots, rate: player.movingHitRate },
    { label: 'Flashed', shots: player.flashedShots, rate: player.flashedHitRate },
    { label: 'Scoped', shots: player.scopedShots, rate: player.scopedHitRate },
    { label: 'Airborne', shots: player.airborneShots, rate: player.airborneHitRate },
  ].filter((row) => row.shots > 0);
  const stats: [string, string][] = [
    ['Crosshair @ exposure', `${player.crosshairMedianAngle.toFixed(1)}°`],
    ['First shot error', `${player.firstShotMedianAngle.toFixed(1)}°`],
    ['Unspotted damage', pct(player.unspottedDamageRate)],
    ['TTD p10', ms(player.ttdP10Ms, player.ttdSamples)],
    ['TTD with AWP', ms(player.awpTtdMedianMs, player.awpTtdSamples)],
    ['TTD without AWP', ms(player.nonAwpTtdMedianMs, player.nonAwpTtdSamples)],
    ['Reaction p10', ms(player.reactionP10Ms, player.reactionSamples)],
    ['Smoke / wall kills', `${player.smokeKills} / ${player.wallKills}`],
  ];
  return (
    <div className="space-y-3 border-l-2 border-primary/40 bg-muted/40 px-6 py-5">
      <div className="grid gap-3 lg:grid-cols-2">
        <Histogram
          title="Time to damage"
          bins={player.ttdHistogram}
          samples={player.ttdSamples}
          medianMs={player.ttdWeightedMs}
          p10Ms={player.ttdP10Ms}
          color="var(--chart-1)"
        />
        <Histogram
          title="Reaction time (first shot)"
          bins={player.reactionHistogram}
          samples={player.reactionSamples}
          medianMs={player.reactionWeightedMs}
          p10Ms={player.reactionP10Ms}
          color="var(--chart-2)"
        />
      </div>

      <div className="grid gap-3 lg:grid-cols-3">
        <div className="space-y-2 rounded-lg border border-border bg-card p-3">
          <span className="mb-3 block text-xs font-medium text-foreground">Kills by weapon</span>
          {killWeapons.length ? (
            <div className="space-y-1.5">
              {killWeapons.map((weapon) => (
                <BarRow
                  key={weapon.weaponName}
                  label={weapon.weaponName}
                  detail={`${weapon.kills} kills · ${pct(weapon.kills / player.kills)} of all kills`}
                  value={weapon.kills / player.kills}
                  color="var(--chart-1)"
                />
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No weapon kill data.</p>
          )}
        </div>

        <div className="space-y-2 rounded-lg border border-border bg-card p-3">
          <span className="mb-3 block text-xs font-medium text-foreground">Accuracy</span>
          {accuracyWeapons.length ? (
            <div className="space-y-1.5">
              {accuracyWeapons.map((weapon) => (
                <BarRow
                  key={weapon.weaponName}
                  label={weapon.weaponName}
                  detail={`${number.format(weapon.shots)} shots · ${weapon.kills} kills`}
                  value={weapon.accuracy}
                  color="#f2a65a"
                  reference={player.accuracy}
                />
              ))}
              <p className="text-[10px] text-muted-foreground">vertical mark = overall accuracy ({pct(player.accuracy)})</p>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Not enough weapon data.</p>
          )}

          <div className="border-t border-border pt-2">
            <span className="text-[11px] font-medium text-muted-foreground">Situational</span>
          </div>
          {situational.length ? (
            <div className="space-y-1.5">
              {situational.map((row) => (
                <BarRow
                  key={row.label}
                  label={row.label}
                  detail={`${number.format(row.shots)} shots`}
                  value={row.rate}
                  color="var(--chart-2)"
                  reference={player.accuracy}
                />
              ))}
              <p className="text-[10px] text-muted-foreground">vertical mark = overall accuracy ({pct(player.accuracy)})</p>
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No situational shots tracked.</p>
          )}
        </div>

        <div className="grid grid-cols-2 content-start gap-x-4 gap-y-3 rounded-lg border border-border bg-card p-3">
          <div className="col-span-2 flex flex-col gap-0.5">
            <span className="text-xs text-muted-foreground">Steam ID</span>
            <a
              href={`https://steamcommunity.com/profiles/${player.steamId}`}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1 text-sm font-medium text-foreground underline decoration-muted-foreground/40 underline-offset-2 hover:decoration-foreground"
            >
              {player.steamId}
              <ExternalLink className="size-3" />
            </a>
          </div>
          {stats.map(([label, value]) => (
            <div key={label} className="flex flex-col gap-0.5">
              <span className="text-xs text-muted-foreground">{label}</span>
              <span className="text-sm font-medium tabular-nums">{value}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="flex flex-col gap-2 rounded-lg border border-border bg-card p-3">
        <span className="text-xs font-medium text-foreground">Triggered signals</span>
        {flagSignals.length ? (
          <div className="flex flex-wrap gap-2">
            {flagSignals.map((rule) => (
              <span
                key={rule.name}
                className={cn(
                  'flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs',
                  rule.tier === 'cheater'
                    ? 'border-destructive/40 bg-destructive/10 text-destructive'
                    : 'border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400',
                )}
              >
                <span className="font-medium">{RULE_LABEL[rule.name] ?? rule.name.replaceAll('_', ' ')}</span>
                <span className="opacity-80">{formatRuleValue(rule)} · n={rule.sample}</span>
              </span>
            ))}
          </div>
        ) : (
          <span className="text-sm text-muted-foreground">No suspicious signals.</span>
        )}
        {infoSignals.length ? (
          <div className="flex flex-wrap gap-2 border-t border-border pt-2">
            {infoSignals.map((rule) => (
              <span key={rule.name} className="flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
                {RULE_LABEL[rule.name] ?? rule.name.replaceAll('_', ' ')}
                <span>{formatRuleValue(rule)}{rule.sample ? ` · n=${rule.sample}` : ''}</span>
              </span>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

const PAGE_SIZES = [12, 25, 50, 100];

type TableFilters = {
  status: StatusFilter;
  savedOnly: boolean;
  bannedOnly: boolean;
  hideBanned: boolean;
  minDemos: number;
  minShots: number;
  minAccuracy: number;
  minHeadHit: number;
  minHsKill: number;
  maxTtdMs: number;
  maxReactionMs: number;
  minKd: number;
};

const DEFAULT_FILTERS: TableFilters = {
  status: 'all',
  savedOnly: false,
  bannedOnly: false,
  hideBanned: false,
  minDemos: 0,
  minShots: 0,
  minAccuracy: 0,
  minHeadHit: 0,
  minHsKill: 0,
  maxTtdMs: 0,
  maxReactionMs: 0,
  minKd: 0,
};

function countActiveFilters(filters: TableFilters) {
  return (Object.keys(DEFAULT_FILTERS) as (keyof TableFilters)[]).filter((key) => filters[key] !== DEFAULT_FILTERS[key]).length;
}

function playerKd(player: Player) {
  return player.deaths ? player.kills / player.deaths : player.kills;
}

function matchesFilters(player: Player, filters: TableFilters) {
  if (filters.status !== 'all' && !(filters.status === 'flagged' ? FLAGGED.includes(player.status) : player.status === filters.status)) return false;
  if (filters.savedOnly && !player.saved) return false;
  if (filters.bannedOnly && !player.banned) return false;
  if (filters.hideBanned && player.banned) return false;
  if (player.demoCount < filters.minDemos) return false;
  if (player.shots < filters.minShots) return false;
  if (player.accuracy < filters.minAccuracy) return false;
  if (player.headHitRate < filters.minHeadHit) return false;
  if (player.headshotKillRate < filters.minHsKill) return false;
  if (filters.maxTtdMs && !(player.ttdSamples && player.ttdWeightedMs <= filters.maxTtdMs)) return false;
  if (filters.maxReactionMs && !(player.reactionSamples && player.reactionWeightedMs <= filters.maxReactionMs)) return false;
  if (filters.minKd && playerKd(player) < filters.minKd) return false;
  return true;
}

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
function FiltersPanel({ filters, onChange }: { filters: TableFilters; onChange: (filters: TableFilters) => void }) {
  const set = <K extends keyof TableFilters>(key: K, value: TableFilters[K]) => onChange({ ...filters, [key]: value });
  const active = countActiveFilters(filters);
  return (
    <div className="flex flex-col gap-4 rounded-lg border border-border bg-card p-4">
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
        <ThresholdSelect label="TTD" value={filters.maxTtdMs} options={[450, 400, 350, 300]} format={(v) => `≤ ${v} ms`} onChange={(v) => set('maxTtdMs', v)} />
        <ThresholdSelect label="Reaction time" value={filters.maxReactionMs} options={[350, 300, 250, 200]} format={(v) => `≤ ${v} ms`} onChange={(v) => set('maxReactionMs', v)} />
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
    </div>
  );
}

const COLUMNS: { key: SortKey | null; label: string }[] = [
  { key: 'name', label: 'Player' },
  { key: 'demoCount', label: 'Demos' },
  { key: 'shots', label: 'Shots' },
  { key: 'kills', label: 'Kills' },
  { key: 'deaths', label: 'Deaths' },
  { key: 'accuracy', label: 'Accuracy' },
  { key: 'headHitRate', label: 'Head hit' },
  { key: 'headshotKillRate', label: 'HS kills' },
  { key: 'ttdWeightedMs', label: 'TTD' },
  { key: 'reactionWeightedMs', label: 'Reaction' },
  { key: 'status', label: 'Status' },
  { key: null, label: 'Actions' },
];

function PlayerTable({
  players,
  weapons,
  onToggleSaved,
  onToggleBanned,
}: {
  players: Player[];
  weapons: PlayerWeapon[];
  onToggleSaved: (player: Player) => void;
  onToggleBanned: (player: Player) => void;
}) {
  const [query, setQuery] = useState('');
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const [filters, setFilters] = useState<TableFilters>(DEFAULT_FILTERS);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [pageSize, setPageSize] = useState(12);
  const [sortKey, setSortKey] = useState<SortKey>('status');
  const [ascending, setAscending] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const [showAll, setShowAll] = useState(false);

  const shown = useMemo(() => {
    return players.filter((player) => {
      const matchesText = !deferredQuery || player.name.toLowerCase().includes(deferredQuery) || String(player.steamId).includes(deferredQuery);
      return matchesText && matchesFilters(player, filters);
    }).toSorted((a, b) => {
      // saved players pin to the top only under the default (status) sort
      if (sortKey === 'status' && a.saved !== b.saved) return a.saved ? -1 : 1;
      const order = sortKey === 'status'
        ? STATUS_RANK[a.status] - STATUS_RANK[b.status]
        : (() => {
            const av = a[sortKey], bv = b[sortKey];
            return typeof av === 'string' ? av.localeCompare(String(bv)) : Number(av) - Number(bv);
          })();
      return ascending ? order : -order;
    });
  }, [players, deferredQuery, filters, sortKey, ascending]);

  const sort = (key: SortKey) => {
    if (sortKey === key) setAscending((value) => !value);
    else { setSortKey(key); setAscending(key === 'name'); }
  };

  useEffect(() => { setPage(0); }, [deferredQuery, filters, sortKey, ascending, pageSize]);

  const hasFilters = Boolean(query) || countActiveFilters(filters) > 0;
  const reset = () => { setQuery(''); setFilters(DEFAULT_FILTERS); setPage(0); };

  const pageCount = Math.max(1, Math.ceil(shown.length / pageSize));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = showAll ? shown : shown.slice(currentPage * pageSize, (currentPage + 1) * pageSize);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div className="relative flex-1 sm:min-w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search name or Steam ID" className="pl-9" />
        </div>
        <Button variant="outline" className={cn(filtersOpen && 'bg-accent')} onClick={() => setFiltersOpen((value) => !value)}>
          <ListFilter data-icon="inline-start" />
          Filters
          <Badge variant="outline" className={cn(!countActiveFilters(filters) && 'invisible')}>{countActiveFilters(filters)}</Badge>
        </Button>
        <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setShowAll(false); }}>
          <SelectTrigger className="w-32"><SelectValue /></SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((size) => (
              <SelectItem key={size} value={String(size)}>{size} / page</SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {filtersOpen ? <FiltersPanel filters={filters} onChange={setFilters} /> : null}

      <div className="overflow-x-auto rounded-lg border border-border">
        <Table className="min-w-[1100px]">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              {COLUMNS.map((col) => (
                <TableHead key={col.label} className={cn('bg-muted/40', !col.key && 'w-[1%] px-2 text-center')}>
                  {col.key ? (
                    <button className="inline-flex items-center gap-1 hover:text-foreground" onClick={() => sort(col.key!)}>
                      {col.label}
                      {sortKey === col.key ? (ascending ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />) : null}
                    </button>
                  ) : (
                    col.label
                  )}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          <TableBody>
            {visible.map((player) => {
              const open = expanded === player.steamId;
              return (
                <Fragment key={player.steamId}>
                  <TableRow
                    className={cn('cursor-pointer', player.banned && 'bg-destructive/10 text-muted-foreground hover:bg-destructive/15')}
                    onClick={() => setExpanded((value) => (value === player.steamId ? null : player.steamId))}
                  >
                    <TableCell>
                      <div className="flex items-center gap-1.5 font-medium">
                        {player.name}
                        {player.isAwper ? (
                          <Badge variant="outline" title={`${player.awpKills} AWP kills (${pct(player.awpKillRate)} of all kills)`}>AWPer</Badge>
                        ) : null}
                        <ChevronDown className={cn('size-3.5 text-muted-foreground transition-transform', open && 'rotate-180')} />
                      </div>
                    </TableCell>
                    <TableCell className="tabular-nums">{player.demoCount}</TableCell>
                    <TableCell className="tabular-nums">{number.format(player.shots)}</TableCell>
                    <TableCell className="tabular-nums">{number.format(player.kills)}</TableCell>
                    <TableCell className="tabular-nums">{number.format(player.deaths)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.accuracy)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.headHitRate)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.headshotKillRate)}</TableCell>
                    <TableCell className="tabular-nums">{ms(player.ttdWeightedMs, player.ttdSamples)}</TableCell>
                    <TableCell className="tabular-nums">{ms(player.reactionWeightedMs, player.reactionSamples)}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(player.status)}>{STATUS_LABEL[player.status]}</Badge>
                    </TableCell>
                    <TableCell className="w-[1%] px-2">
                      <div className="flex items-center justify-center gap-1">
                        <button
                          className={cn('text-muted-foreground hover:text-foreground', player.saved && 'text-primary hover:text-primary')}
                          title={player.saved ? 'Unsave player' : 'Save player'}
                          onClick={(event) => { event.stopPropagation(); onToggleSaved(player); }}
                        >
                          <Bookmark className={cn('size-4', player.saved && 'fill-current')} />
                        </button>
                        <button
                          className={cn('text-muted-foreground hover:text-destructive', player.banned && 'text-destructive')}
                          title={player.banned ? 'Unmark as banned' : 'Mark as banned'}
                          onClick={(event) => { event.stopPropagation(); onToggleBanned(player); }}
                        >
                          <Ban className="size-4" />
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                  {open ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={COLUMNS.length} className="p-0">
                        <PlayerDetails player={player} weapons={weapons.filter((weapon) => weapon.steamId === player.steamId)} />
                      </TableCell>
                    </TableRow>
                  ) : null}
                </Fragment>
              );
            })}
          </TableBody>
        </Table>
        {!shown.length ? <div className="py-14 text-center text-sm text-muted-foreground">No players match these filters.</div> : null}
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          {shown.length} of {players.length} players · click a row for signal detail
        </p>
        <div className="flex items-center gap-2">
          {!showAll && pageCount > 1 ? (
            <>
              <Button variant="outline" size="sm" disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}>
                <ChevronLeft className="size-4" />
              </Button>
              <span className="text-sm tabular-nums text-muted-foreground">{currentPage + 1} / {pageCount}</span>
              <Button variant="outline" size="sm" disabled={currentPage >= pageCount - 1} onClick={() => setPage(currentPage + 1)}>
                <ChevronRight className="size-4" />
              </Button>
            </>
          ) : null}
          {shown.length > pageSize ? (
            <Button variant="ghost" size="sm" onClick={() => { setShowAll((value) => !value); setPage(0); }}>
              {showAll ? 'Show pages' : 'Show all'}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

function DemosSection({ demos, onChanged }: { demos: Demo[]; onChanged: () => void }) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const [failed, setFailed] = useState(false);
  const [pageSize, setPageSize] = useState(12);
  const [page, setPage] = useState(0);
  const [showAll, setShowAll] = useState(false);
  const enabledCount = demos.filter((demo) => demo.enabled).length;

  const sorted = useMemo(() => demos.toSorted((a, b) => (b.date || '').localeCompare(a.date || '')), [demos]);
  const pageCount = Math.max(1, Math.ceil(sorted.length / pageSize));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = showAll ? sorted : sorted.slice(currentPage * pageSize, (currentPage + 1) * pageSize);

  const dayKey = (demo: Demo) => (demo.date ? demo.date.slice(0, 10) : 'unknown');
  const dayGroups = useMemo(() => {
    const groups = new Map<string, Demo[]>();
    for (const demo of visible) {
      const key = dayKey(demo);
      const group = groups.get(key);
      if (group) group.push(demo);
      else groups.set(key, [demo]);
    }
    return [...groups.entries()];
  }, [visible]);

  const report = (text: string, isError: boolean) => { setMessage(text); setFailed(isError); };

  const toggle = async (demo: Demo) => {
    setPending(demo.checksum);
    try { await setDemoEnabled(demo.checksum, !demo.enabled); setMessage(''); onChanged(); }
    catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
    finally { setPending(null); }
  };

  const remove = async (demo: Demo) => {
    if (!window.confirm(`Permanently delete ${demo.fileName} and all of its stats?`)) return;
    setPending(demo.checksum);
    try { await deleteDemo(demo.checksum); setMessage(''); onChanged(); }
    catch (cause) { report(cause instanceof Error ? cause.message : 'Delete failed', true); }
    finally { setPending(null); }
  };

  // Toggles every demo of the day (also the ones on other pages).
  const toggleDay = async (key: string, target: boolean) => {
    setPending(key);
    try {
      const changed = demos.filter((demo) => dayKey(demo) === key && demo.enabled !== target);
      await Promise.all(changed.map((demo) => setDemoEnabled(demo.checksum, target)));
      setMessage('');
      onChanged();
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
    finally { setPending(null); }
  };

  const importFile = async (file: File) => {
    try {
      const result = await importStats(file);
      report(`Imported ${result.imported} demo${result.imported === 1 ? '' : 's'}, skipped ${result.skipped} already known.`, false);
      onChanged();
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Import failed', true); }
  };

  return (
    <Dialog>
      <DialogTrigger
        render={
          <Button variant="outline" size="sm">
            <Film className="size-4" /> Demos ({enabledCount}/{demos.length})
          </Button>
        }
      />
      <DialogContent className="max-w-7xl px-16 py-12">
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <DialogTitle className="text-2xl">Demos</DialogTitle>
              <p className="text-sm text-muted-foreground">{demos.length} demo{demos.length === 1 ? '' : 's'} · {enabledCount} included in stats</p>
            </div>
            <div className="flex gap-2">
              {demos.length ? (
                <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setShowAll(false); setPage(0); }}>
                  <SelectTrigger className="h-9 w-32"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {PAGE_SIZES.map((size) => (
                      <SelectItem key={size} value={String(size)}>{size} / page</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : null}
              <input ref={fileInput} type="file" accept=".json,application/json" hidden onChange={(event) => { const file = event.target.files?.[0]; if (file) void importFile(file); event.target.value = ''; }} />
              <Button variant="outline" size="sm" className="h-9" onClick={() => fileInput.current?.click()}>
                <FileUp className="size-4" /> Import
              </Button>
              <Button variant="outline" size="sm" className="h-9" onClick={() => { window.location.href = '/api/export'; }}>
                <FileDown className="size-4" /> Export
              </Button>
            </div>
          </div>

          {message ? <p className={cn('text-sm', failed ? 'text-destructive' : 'text-muted-foreground')}>{message}</p> : null}

          {demos.length ? (
            <>
              <div className="max-h-[60vh] overflow-auto rounded-lg border border-border">
                <Table className="min-w-[760px]">
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      {['In stats', 'File', 'Map', 'Played', 'Source', 'Players', 'Rounds', 'Added', ''].map((label, index) => (
                        <TableHead key={label || index} className="bg-muted/40">{label}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dayGroups.map(([key, group]) => {
                      const dayDemos = demos.filter((demo) => dayKey(demo) === key);
                      const enabledInDay = dayDemos.filter((demo) => demo.enabled).length;
                      const allEnabled = enabledInDay === dayDemos.length;
                      return (
                        <Fragment key={key}>
                          <TableRow className="bg-muted/40 hover:bg-muted/40">
                            <TableCell>
                              <input
                                type="checkbox"
                                className="size-4 accent-primary"
                                checked={allEnabled}
                                ref={(el) => { if (el) el.indeterminate = enabledInDay > 0 && !allEnabled; }}
                                disabled={pending === key}
                                title={allEnabled ? 'Exclude the whole day' : 'Include the whole day'}
                                onChange={() => void toggleDay(key, !allEnabled)}
                              />
                            </TableCell>
                            <TableCell colSpan={8} className="text-xs font-medium text-muted-foreground">
                              {key === 'unknown' ? 'Unknown date' : new Date(key).toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
                              {' · '}{enabledInDay}/{dayDemos.length} in stats
                            </TableCell>
                          </TableRow>
                          {group.map((demo) => (
                            <TableRow key={demo.checksum} className={cn(!demo.enabled && 'opacity-50')}>
                              <TableCell>
                                <input
                                  type="checkbox"
                                  className="size-4 accent-primary"
                                  checked={demo.enabled}
                                  disabled={pending === demo.checksum}
                                  onChange={() => void toggle(demo)}
                                />
                              </TableCell>
                              <TableCell className="font-medium">{demo.fileName}</TableCell>
                              <TableCell>{demo.mapName}</TableCell>
                              <TableCell className="tabular-nums">{demo.date ? new Date(demo.date).toLocaleDateString() : '—'}</TableCell>
                              <TableCell>{demo.source}</TableCell>
                              <TableCell className="tabular-nums">{demo.players}</TableCell>
                              <TableCell className="tabular-nums">{demo.rounds}</TableCell>
                              <TableCell className="tabular-nums">{demo.importedAt ? new Date(demo.importedAt).toLocaleDateString() : '—'}</TableCell>
                              <TableCell className="w-[1%] px-2">
                                <button
                                  className="text-muted-foreground transition-colors hover:text-destructive disabled:opacity-50"
                                  title="Delete demo and all of its stats"
                                  disabled={pending === demo.checksum}
                                  onClick={() => void remove(demo)}
                                >
                                  <Trash2 className="size-4" />
                                </button>
                              </TableCell>
                            </TableRow>
                          ))}
                        </Fragment>
                      );
                    })}
                  </TableBody>
                </Table>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-muted-foreground">{demos.length} demos</p>
                <div className="flex items-center gap-2">
                  {!showAll && pageCount > 1 ? (
                    <>
                      <Button variant="outline" size="sm" disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}>
                        <ChevronLeft className="size-4" />
                      </Button>
                      <span className="text-sm tabular-nums text-muted-foreground">{currentPage + 1} / {pageCount}</span>
                      <Button variant="outline" size="sm" disabled={currentPage >= pageCount - 1} onClick={() => setPage(currentPage + 1)}>
                        <ChevronRight className="size-4" />
                      </Button>
                    </>
                  ) : null}
                  {demos.length > pageSize ? (
                    <Button variant="ghost" size="sm" onClick={() => { setShowAll((value) => !value); setPage(0); }}>
                      {showAll ? 'Show pages' : 'Show all'}
                    </Button>
                  ) : null}
                </div>
              </div>
            </>
          ) : (
            <div className="rounded-lg border border-border py-10 text-center text-sm text-muted-foreground">No demos analyzed yet.</div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function GuideSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="space-y-2 border-b border-border pb-5 last:border-0">
      <h3 className="text-sm font-semibold text-foreground">{title}</h3>
      <div className="space-y-2 text-sm leading-5 text-muted-foreground">{children}</div>
    </section>
  );
}

function GuideItem({ term, children }: { term: string; children: ReactNode }) {
  return <p><span className="font-medium text-foreground">{term}</span> — {children}</p>;
}

function CheatSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  useEffect(() => {
    if (!open) return;
    const closeOnEscape = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose(); };
    window.addEventListener('keydown', closeOnEscape);
    return () => window.removeEventListener('keydown', closeOnEscape);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <>
      <button aria-label="Close guide" className="fixed inset-0 z-40 cursor-default bg-black/30" onClick={onClose} />
      <aside aria-label="Cheat sheet" className="fixed inset-y-0 right-0 z-50 flex w-full max-w-xl flex-col border-l border-border bg-background shadow-2xl">
        <header className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
          <div>
            <h2 className="text-base font-semibold">Cheat sheet: how to read the analysis</h2>
            <p className="mt-1 text-sm text-muted-foreground">Use signals to choose what to review in a demo — never as an automatic verdict.</p>
          </div>
          <Button variant="ghost" size="icon" aria-label="Close cheat sheet" onClick={onClose}><X className="size-4" /></Button>
        </header>
        <div className="cheat-sheet-scroll min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5">
          <GuideSection title="Start here">
            <GuideItem term="Status">A review priority, not proof. `Cheater` (red) marks stats that are not humanly reproducible over many games; `Watch` (yellow) is a grey-zone flag. Always expand the row and inspect the triggered signals before judging.</GuideItem>
            <GuideItem term="Samples">Check the number of demos, shots and sample count (`n=`). A small sample can make an ordinary streak look extreme.</GuideItem>
            <GuideItem term="Workflow">Use the table to find a player, open their row, note multiple independent signals, then review the relevant rounds in the demo.</GuideItem>
          </GuideSection>

          <GuideSection title="Basic combat context">
            <GuideItem term="Demos / shots">The amount of evidence available. More unique demos and weapon-fire events make a comparison more useful.</GuideItem>
            <GuideItem term="Kills / deaths">Impact context only. A high K/D is never, by itself, evidence of cheating.</GuideItem>
            <GuideItem term="Accuracy">Tracked hit shots divided by tracked weapon shots. Compare weapon choice and sample size; AWP, pistols and rifles behave differently.</GuideItem>
            <GuideItem term="Head hit / HS kills">Head-hit rate uses every damaging hit; HS-kill rate uses only finishing headshots. High values can be skill, weapon choice or close-range play.</GuideItem>
          </GuideSection>

          <GuideSection title="Exposure and response">
            <GuideItem term="TTD">Time from first spotted tick to first damage, as a round-weighted long-term average. Rough bands: 450+ ms healthy, 400–450 elite, 320–400 suspicious (yellow, red when the player also frags hard), under 320 ms not humanly reproducible over many games (red). `p10` in details is the fast 10% tail.</GuideItem>
            <GuideItem term="AWPer">Players with at least 5 AWP kills and at least 25% of all kills made with the AWP. Their details split AWP TTD from non-AWP TTD for a fairer comparison with riflers.</GuideItem>
            <GuideItem term="Reaction">Time from first spotted tick to first shot. It is a demo-derived estimate, not a laboratory reaction-time test; pre-aim, sound cues and prediction affect it.</GuideItem>
            <GuideItem term="Crosshair @ exposure">Median angular distance from crosshair to opponent at confirmed exposure. Lower means stronger crosshair placement, not cheating by itself.</GuideItem>
            <GuideItem term="First shot error">Median angular distance at the first shot. Read it together with TTD and reaction instead of treating it as a standalone verdict.</GuideItem>
          </GuideSection>

          <GuideSection title="Suspicion signals">
            <GuideItem term="Unspotted damage">Damage where the analyzer did not have a confirmed spotted state. Check the demo for sound, teammate information, wallbang lines, smokes and replay limitations before judging it.</GuideItem>
            <GuideItem term="First-bullet head / snap">Signals around unusually accurate first shots and fast aim reduction. They are strongest when repeated over many encounters and paired with unusual TTD or reactions.</GuideItem>
            <GuideItem term="Smoke / wall kills">Useful review context, but legitimate wallbangs and common angles are expected in Counter-Strike. Look for repetition and timing, not isolated kills.</GuideItem>
            <GuideItem term="Triggered signals">Coloured badges (red = cheater, yellow = watch) are the signals that set the status; grey badges below them are context only (smoke/wall kills, unspotted damage) and colour nothing. Each states its value and `n=` sample count.</GuideItem>
          </GuideSection>

          <GuideSection title="Saved players and demos">
            <GuideItem term="Bookmark">Use the bookmark in the Status column to pin a player to the top while reviewing them later.</GuideItem>
            <GuideItem term="Demos">The Demos button lets you enable or disable a demo from aggregates and export/import the saved statistics set.</GuideItem>
          </GuideSection>

          <GuideSection title="Decision rule">
            <p>One metric is a reason to look, not a conclusion. Prioritize players with enough samples and multiple independent signals, then validate them against the demo timeline.</p>
          </GuideSection>
        </div>
      </aside>
    </>
  );
}

export default function App() {
  const [report, setReport] = useState<Report>(EMPTY_REPORT);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [guideOpen, setGuideOpen] = useState(false);
  const completedRef = useRef('');

  const loadAll = useCallback(async () => {
    try { const [nextReport, nextJobs] = await Promise.all([getReport(), getJobs()]); setReport(nextReport); setJobs(nextJobs); setError(''); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load data'); }
    finally { setLoading(false); }
  }, []);

  const toggleSaved = useCallback(async (player: Player) => {
    // optimistic flip; reload on failure to restore server state
    setReport((current) => ({
      ...current,
      players: (current.players ?? []).map((p) => (p.steamId === player.steamId ? { ...p, saved: !player.saved } : p)),
    }));
    try { await setPlayerSaved(player.steamId, !player.saved); }
    catch { void loadAll(); }
  }, [loadAll]);

  const toggleBanned = useCallback(async (player: Player) => {
    setReport((current) => ({
      ...current,
      players: (current.players ?? []).map((p) => (p.steamId === player.steamId ? { ...p, banned: !player.banned } : p)),
    }));
    try { await setPlayerBanned(player.steamId, !player.banned); }
    catch { void loadAll(); }
  }, [loadAll]);

  useEffect(() => { void loadAll(); }, [loadAll]);
  useEffect(() => {
    const timer = window.setInterval(async () => {
      try {
        const nextJobs = await getJobs(); setJobs(nextJobs);
        // Refetch the report whenever the set of finished jobs changes so the
        // table updates on its own once analysis completes (or partially fails).
        const signature = nextJobs.filter((job) => job.status === 'completed' || job.status === 'failed').map((job) => job.id).join(':');
        if (signature !== completedRef.current) { completedRef.current = signature; setReport(await getReport()); }
      } catch { /* the persistent banner from loadAll is sufficient */ }
    }, 1000);
    return () => window.clearInterval(timer);
  }, []);

  const players = report.players ?? [];

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6 px-4 py-8 sm:px-6 sm:py-10">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">CS2 demo analysis</h1>
          <p className="text-sm text-muted-foreground">Upload demos to build the player baseline and review flagged accounts.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => setGuideOpen(true)}><BookOpen className="size-4" /> Cheat sheet</Button>
          {!loading ? <DemosSection demos={report.importedDemos ?? []} onChanged={() => void loadAll()} /> : null}
        </div>
      </div>

      <Dropzone onQueued={(job) => setJobs((current) => [job, ...current])} />
      <Jobs jobs={jobs} />

      {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">API offline — {error}</div> : null}

      {loading ? (
        <div className="py-20 text-center text-sm text-muted-foreground">Loading…</div>
      ) : (
        <PlayerTable
          players={players}
          weapons={report.playersByWeapon ?? []}
          onToggleSaved={(player) => void toggleSaved(player)}
          onToggleBanned={(player) => void toggleBanned(player)}
        />
      )}
      <CheatSheet open={guideOpen} onClose={() => setGuideOpen(false)} />
    </div>
  );
}
