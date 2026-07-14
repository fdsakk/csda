import { DragEvent, Fragment, type ReactNode, useCallback, useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowDown, ArrowUp, BookOpen, Bookmark, ChevronDown, ChevronLeft, ChevronRight, ExternalLink, FileDown, FileUp, Film, Search, Upload, X } from 'lucide-react';
import { Demo, getJobs, getReport, importStats, Job, Player, Report, setDemoEnabled, setPlayerSaved, uploadDemos } from './api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { cn } from '@/lib/utils';

type SortKey = 'suspicionScore' | 'name' | 'demoCount' | 'shots' | 'kills' | 'deaths' | 'accuracy' | 'headHitRate' | 'headshotKillRate' | 'ttdWeightedMs' | 'reactionWeightedMs';
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

const PAGE_SIZES = [12, 25, 50, 100];

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
  { key: 'suspicionScore', label: 'Status' },
];

function PlayerTable({ players, onToggleSaved }: { players: Player[]; onToggleSaved: (player: Player) => void }) {
  const [query, setQuery] = useState('');
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const [status, setStatus] = useState<StatusFilter>('all');
  const [pageSize, setPageSize] = useState(12);
  const [sortKey, setSortKey] = useState<SortKey>('suspicionScore');
  const [ascending, setAscending] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [page, setPage] = useState(0);
  const [showAll, setShowAll] = useState(false);

  const shown = useMemo(() => {
    return players.filter((player) => {
      const matchesText = !deferredQuery || player.name.toLowerCase().includes(deferredQuery) || String(player.steamId).includes(deferredQuery);
      const matchesStatus = status === 'all' || (status === 'flagged' ? FLAGGED.includes(player.status) : player.status === status);
      return matchesText && matchesStatus;
    }).toSorted((a, b) => {
      // saved players pin to the top only under the default (status) sort
      if (sortKey === 'suspicionScore' && a.saved !== b.saved) return a.saved ? -1 : 1;
      const av = a[sortKey], bv = b[sortKey];
      const order = typeof av === 'string' ? av.localeCompare(String(bv)) : Number(av) - Number(bv);
      return ascending ? order : -order;
    });
  }, [players, deferredQuery, status, sortKey, ascending]);

  const sort = (key: SortKey) => {
    if (sortKey === key) setAscending((value) => !value);
    else { setSortKey(key); setAscending(key === 'name'); }
  };

  useEffect(() => { setPage(0); }, [deferredQuery, status, sortKey, ascending, pageSize]);

  const hasFilters = query || status !== 'all';
  const reset = () => { setQuery(''); setStatus('all'); setPage(0); };

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
        <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setShowAll(false); }}>
          <SelectTrigger className="w-32"><SelectValue /></SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((size) => (
              <SelectItem key={size} value={String(size)}>{size} / page</SelectItem>
            ))}
          </SelectContent>
        </Select>
        {hasFilters ? <Button variant="ghost" size="sm" onClick={reset}>Clear</Button> : null}
      </div>

      <div className="overflow-x-auto rounded-lg border border-border">
        <Table className="min-w-[1100px]">
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
            {visible.map((player) => {
              const open = expanded === player.steamId;
              return (
                <Fragment key={player.steamId}>
                  <TableRow
                    className="cursor-pointer"
                    onClick={() => setExpanded((value) => (value === player.steamId ? null : player.steamId))}
                  >
                    <TableCell>
                      <div className="flex items-center gap-1.5 font-medium">
                        {player.name}
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
                      <div className="flex items-center gap-1.5">
                        <button
                          className={cn('text-muted-foreground hover:text-foreground', player.saved && 'text-primary hover:text-primary')}
                          title={player.saved ? 'Unsave player' : 'Save player'}
                          onClick={(event) => { event.stopPropagation(); onToggleSaved(player); }}
                        >
                          <Bookmark className={cn('size-4', player.saved && 'fill-current')} />
                        </button>
                        <Badge variant={statusVariant(player.status)}>{STATUS_LABEL[player.status]}</Badge>
                      </div>
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

  const pageCount = Math.max(1, Math.ceil(demos.length / pageSize));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = showAll ? demos : demos.slice(currentPage * pageSize, (currentPage + 1) * pageSize);

  const report = (text: string, isError: boolean) => { setMessage(text); setFailed(isError); };

  const toggle = async (demo: Demo) => {
    setPending(demo.checksum);
    try { await setDemoEnabled(demo.checksum, !demo.enabled); setMessage(''); onChanged(); }
    catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
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
      <DialogContent className="max-w-6xl px-16 py-12">
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
                      {['In stats', 'File', 'Map', 'Played', 'Source', 'Players', 'Rounds', 'Added'].map((label) => (
                        <TableHead key={label} className="bg-muted/40">{label}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visible.map((demo) => (
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
                      </TableRow>
                    ))}
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
            <GuideItem term="Status">A review priority, not proof. Start with `Watch`, `Review` or `Critical`, then expand the row and inspect the triggered signals.</GuideItem>
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
            <GuideItem term="TTD">Time from first spotted tick to first damage. The table shows a round-weighted median; `p10` in details is the fast 10% tail. Repeated low TTD is more relevant than one fast duel.</GuideItem>
            <GuideItem term="Reaction">Time from first spotted tick to first shot. It is a demo-derived estimate, not a laboratory reaction-time test; pre-aim, sound cues and prediction affect it.</GuideItem>
            <GuideItem term="Crosshair @ exposure">Median angular distance from crosshair to opponent at confirmed exposure. Lower means stronger crosshair placement, not cheating by itself.</GuideItem>
            <GuideItem term="First shot error">Median angular distance at the first shot. Read it together with TTD and reaction instead of treating it as a standalone verdict.</GuideItem>
          </GuideSection>

          <GuideSection title="Suspicion signals">
            <GuideItem term="Unspotted damage">Damage where the analyzer did not have a confirmed spotted state. Check the demo for sound, teammate information, wallbang lines, smokes and replay limitations before judging it.</GuideItem>
            <GuideItem term="First-bullet head / snap">Signals around unusually accurate first shots and fast aim reduction. They are strongest when repeated over many encounters and paired with unusual TTD or reactions.</GuideItem>
            <GuideItem term="Smoke / wall kills">Useful review context, but legitimate wallbangs and common angles are expected in Counter-Strike. Look for repetition and timing, not isolated kills.</GuideItem>
            <GuideItem term="Triggered signals">Each badge in the expanded row states the rule, points and `n=` sample count that contributed to the status.</GuideItem>
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
        <PlayerTable players={players} onToggleSaved={(player) => void toggleSaved(player)} />
      )}
      <CheatSheet open={guideOpen} onClose={() => setGuideOpen(false)} />
    </div>
  );
}
