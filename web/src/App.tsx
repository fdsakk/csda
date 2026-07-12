import { DragEvent, Fragment, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowDown, ArrowUp, ChevronDown, ExternalLink, Search, Upload, X } from 'lucide-react';
import { getJobs, getReport, Job, Player, Report, uploadDemos } from './api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';

type SortKey = 'suspicionScore' | 'name' | 'demoCount' | 'shots' | 'accuracy' | 'headHitRate' | 'headshotKillRate' | 'ttdWeightedMs' | 'reactionWeightedMs';
type StatusFilter = 'all' | 'flagged' | Player['status'];

const EMPTY_REPORT: Report = { players: [], playersByWeapon: [], evidence: [], importedDemos: [] };
const number = new Intl.NumberFormat('en-US');
const FLAGGED = ['watch', 'review', 'critical'];

const STATUS_LABEL: Record<Player['status'], string> = {
  normal: 'Normal',
  watch: 'Watch',
  review: 'Review',
  critical: 'Critical',
  insufficient_sample: 'Low sample',
};

function pct(value: number) { return `${(value * 100).toFixed(1)}%`; }
function ms(value: number, samples: number) { return samples ? `${Math.round(value)} ms` : '—'; }

function statusVariant(status: Player['status']) {
  if (status === 'review' || status === 'critical') return 'destructive' as const;
  if (status === 'watch') return 'warning' as const;
  if (status === 'insufficient_sample') return 'outline' as const;
  return 'default' as const;
}

function Dropzone({ onQueued }: { onQueued: (job: Job) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  const [source, setSource] = useState('valve');
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState('');

  const accept = useCallback((incoming: File[]) => {
    const demos = incoming.filter((file) => file.name.toLowerCase().endsWith('.dem'));
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
    try { const job = await uploadDemos(files, source, setProgress); setFiles([]); onQueued(job); }
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

      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-muted-foreground">Demo source</label>
          <Select value={source} onValueChange={(value) => setSource(value as string)}>
            <SelectTrigger className="w-52"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="valve">Valve / generic CSTV</SelectItem>
              <SelectItem value="matchzy">MatchZy</SelectItem>
              <SelectItem value="pracc">PRACC</SelectItem>
              <SelectItem value="faceit">FACEIT</SelectItem>
              <SelectItem value="esportal">Esportal</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button disabled={!files.length || progress !== null} onClick={submit}>
          {progress === null
            ? `Analyze ${files.length || ''} demo${files.length === 1 ? '' : 's'}`.replace('  ', ' ')
            : `Uploading ${progress}%`}
        </Button>
      </div>

      {files.length ? (
        <div className="flex flex-wrap gap-2 px-4 pb-4">
          {files.map((file) => (
            <span key={`${file.name}:${file.size}`} className="flex items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
              {file.name}
              <button className="text-muted-foreground hover:text-foreground" onClick={() => setFiles((all) => all.filter((item) => item !== file))}>
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
      ) : null}

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
        const percent = running ? Math.round((job.processed / total) * 100) : 0;
        return (
          <div key={job.id} className="rounded-lg border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="flex min-w-0 items-center gap-2">
                <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-warning" />
                <span className="truncate text-foreground">{job.files.join(', ')}</span>
              </span>
              <span className="shrink-0 text-muted-foreground">
                {running ? `Analyzing ${job.processed}/${total}` : 'Queued'}
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

function PlayerDetails({ player }: { player: Player }) {
  const rules = player.triggeredRules ?? [];
  const stats: [string, string][] = [
    ['Crosshair @ exposure', `${player.crosshairMedianAngle.toFixed(1)}°`],
    ['First shot error', `${player.firstShotMedianAngle.toFixed(1)}°`],
    ['Unspotted damage', pct(player.unspottedDamageRate)],
    ['TTD p10', ms(player.ttdP10Ms, player.ttdSamples)],
    ['Reaction p10', ms(player.reactionP10Ms, player.reactionSamples)],
  ];
  return (
    <div className="space-y-4 border-l-2 border-primary/40 bg-muted/40 px-6 py-4">
      <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <div className="flex flex-col gap-0.5">
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
            <span className="text-sm font-medium">{value}</span>
          </div>
        ))}
      </div>
      <div className="flex flex-col gap-2">
        <span className="text-xs text-muted-foreground">Triggered signals</span>
        {rules.length ? (
          <div className="flex flex-wrap gap-2">
            {rules.map((rule) => (
              <span key={rule.name} className="flex items-center gap-1.5 rounded-md bg-card px-2 py-1 text-xs">
                {rule.name.replaceAll('_', ' ')}
                <span className="text-destructive">+{rule.points}</span>
                <span className="text-muted-foreground">{rule.value.toFixed(2)} · n={rule.sample}</span>
              </span>
            ))}
          </div>
        ) : (
          <span className="text-sm text-muted-foreground">No scoring rules triggered.</span>
        )}
      </div>
    </div>
  );
}

const COLUMNS: { key: SortKey | null; label: string }[] = [
  { key: 'suspicionScore', label: 'Risk' },
  { key: 'name', label: 'Player' },
  { key: 'demoCount', label: 'Demos' },
  { key: 'shots', label: 'Shots' },
  { key: 'accuracy', label: 'Accuracy' },
  { key: 'headHitRate', label: 'Head hit' },
  { key: 'headshotKillRate', label: 'HS kills' },
  { key: 'ttdWeightedMs', label: 'TTD' },
  { key: 'reactionWeightedMs', label: 'Reaction' },
  { key: null, label: 'Status' },
];

function PlayerTable({ players }: { players: Player[] }) {
  const [query, setQuery] = useState('');
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const [status, setStatus] = useState<StatusFilter>('all');
  const [minDemos, setMinDemos] = useState('');
  const [minAccuracy, setMinAccuracy] = useState('');
  const [sortKey, setSortKey] = useState<SortKey>('suspicionScore');
  const [ascending, setAscending] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);

  const shown = useMemo(() => {
    const demoFloor = Number(minDemos) || 0;
    const accFloor = (Number(minAccuracy) || 0) / 100;
    return players.filter((player) => {
      const matchesText = !deferredQuery || player.name.toLowerCase().includes(deferredQuery) || String(player.steamId).includes(deferredQuery);
      const matchesStatus = status === 'all' || (status === 'flagged' ? FLAGGED.includes(player.status) : player.status === status);
      return matchesText && matchesStatus && player.demoCount >= demoFloor && player.accuracy >= accFloor;
    }).toSorted((a, b) => {
      const av = a[sortKey], bv = b[sortKey];
      const order = typeof av === 'string' ? av.localeCompare(String(bv)) : Number(av) - Number(bv);
      return ascending ? order : -order;
    });
  }, [players, deferredQuery, status, minDemos, minAccuracy, sortKey, ascending]);

  const sort = (key: SortKey) => {
    if (sortKey === key) setAscending((value) => !value);
    else { setSortKey(key); setAscending(key === 'name'); }
  };

  const hasFilters = query || status !== 'all' || minDemos || minAccuracy;
  const reset = () => { setQuery(''); setStatus('all'); setMinDemos(''); setMinAccuracy(''); };

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div className="relative flex-1 sm:min-w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search name or Steam ID" className="pl-9" />
        </div>
        <Select value={status} onValueChange={(value) => setStatus(value as StatusFilter)}>
          <SelectTrigger className="w-40"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="flagged">Flagged only</SelectItem>
            <SelectItem value="critical">Critical</SelectItem>
            <SelectItem value="review">Review</SelectItem>
            <SelectItem value="watch">Watch</SelectItem>
            <SelectItem value="normal">Normal</SelectItem>
            <SelectItem value="insufficient_sample">Low sample</SelectItem>
          </SelectContent>
        </Select>
        <Input type="number" min={0} value={minDemos} onChange={(event) => setMinDemos(event.target.value)} placeholder="Min demos" className="w-28" />
        <Input type="number" min={0} max={100} value={minAccuracy} onChange={(event) => setMinAccuracy(event.target.value)} placeholder="Min acc %" className="w-28" />
        {hasFilters ? <Button variant="ghost" size="sm" onClick={reset}>Clear</Button> : null}
      </div>

      <div className="overflow-x-auto rounded-lg border border-border">
        <Table className="min-w-[960px]">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              {COLUMNS.map((col) => (
                <TableHead key={col.label} className="bg-muted/40">
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
            {shown.map((player) => {
              const open = expanded === player.steamId;
              return (
                <Fragment key={player.steamId}>
                  <TableRow
                    className="cursor-pointer"
                    onClick={() => setExpanded((value) => (value === player.steamId ? null : player.steamId))}
                  >
                    <TableCell className="font-medium tabular-nums">{player.suspicionScore}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5 font-medium">
                        {player.name}
                        <ChevronDown className={cn('size-3.5 text-muted-foreground transition-transform', open && 'rotate-180')} />
                      </div>
                    </TableCell>
                    <TableCell className="tabular-nums">{player.demoCount}</TableCell>
                    <TableCell className="tabular-nums">{number.format(player.shots)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.accuracy)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.headHitRate)}</TableCell>
                    <TableCell className="tabular-nums">{pct(player.headshotKillRate)}</TableCell>
                    <TableCell className="tabular-nums">{ms(player.ttdWeightedMs, player.ttdSamples)}</TableCell>
                    <TableCell className="tabular-nums">{ms(player.reactionWeightedMs, player.reactionSamples)}</TableCell>
                    <TableCell>
                      <Badge variant={statusVariant(player.status)}>{STATUS_LABEL[player.status]}</Badge>
                    </TableCell>
                  </TableRow>
                  {open ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={COLUMNS.length} className="p-0">
                        <PlayerDetails player={player} />
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

      <p className="text-sm text-muted-foreground">
        {shown.length} of {players.length} players · click a row for signal detail
      </p>
    </div>
  );
}

export default function App() {
  const [report, setReport] = useState<Report>(EMPTY_REPORT);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const completedRef = useRef('');

  const loadAll = useCallback(async () => {
    try { const [nextReport, nextJobs] = await Promise.all([getReport(), getJobs()]); setReport(nextReport); setJobs(nextJobs); setError(''); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load data'); }
    finally { setLoading(false); }
  }, []);

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
    <div className="mx-auto w-full max-w-6xl space-y-6 px-4 py-8 sm:px-6 sm:py-10">
      <div>
        <h1 className="text-xl font-semibold">CS2 demo analysis</h1>
        <p className="text-sm text-muted-foreground">Upload demos to build the player baseline and review flagged accounts.</p>
      </div>

      <Dropzone onQueued={(job) => setJobs((current) => [job, ...current])} />
      <Jobs jobs={jobs} />

      {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">API offline — {error}</div> : null}

      {loading ? (
        <div className="py-20 text-center text-sm text-muted-foreground">Loading…</div>
      ) : (
        <PlayerTable players={players} />
      )}
    </div>
  );
}
