import { Fragment, type CSSProperties, useEffect, useMemo, useState, useDeferredValue } from 'react';
import { ArrowDown, ArrowUp, Ban, Bookmark, ChevronDown, ChevronLeft, ChevronRight, ListFilter, Search } from 'lucide-react';
import { Player, PlayerWeapon } from '@/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { PlayerDetails } from '@/components/PlayerDetails';
import { FiltersPanel } from '@/components/PlayerFilters';
import { countActiveFilters, DEFAULT_FILTERS, matchesFilters, TableFilters } from '@/lib/filters';
import { ms, number, pct, scoreColor } from '@/lib/format';
import { cn } from '@/lib/utils';

type SortKey = 'status' | 'suspicionScore' | 'name' | 'demoCount' | 'shots' | 'kills' | 'deaths' | 'accuracy' | 'headHitRate' | 'headshotKillRate' | 'nonAwpTtdWeightedMs' | 'nonAwpReactionWeightedMs';

const STATUS_RANK: Record<Player['status'], number> = { cheater: 3, watch: 2, normal: 1, insufficient_sample: 0 };

const STATUS_LABEL: Record<Player['status'], string> = {
  normal: 'Normal',
  watch: 'Watch',
  cheater: 'Cheater',
  insufficient_sample: 'Low sample',
};

function statusVariant(status: Player['status']) {
  if (status === 'cheater') return 'destructive' as const;
  if (status === 'watch') return 'warning' as const;
  if (status === 'insufficient_sample') return 'outline' as const;
  return 'default' as const;
}

const PAGE_SIZES = [12, 25, 50, 100];

// Table view state survives reloads (the page refreshes itself after analysis).
const STORAGE_KEY = 'csda.playerTable';

type StoredTableState = { filters?: Partial<TableFilters>; sortKey?: SortKey; ascending?: boolean; pageSize?: number | 'all' };

function loadStoredState(): StoredTableState {
  try {
    const state: StoredTableState = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '{}');
    if (!COLUMNS.some((column) => column.key === state.sortKey)) delete state.sortKey;
    if (state.pageSize !== 'all' && !PAGE_SIZES.includes(state.pageSize ?? 0)) delete state.pageSize;
    return state;
  } catch {
    return {};
  }
}

const COLUMNS: { key: SortKey | null; label: string }[] = [
  { key: 'suspicionScore', label: 'Score' },
  { key: 'name', label: 'Player' },
  { key: 'demoCount', label: 'Demos' },
  { key: 'shots', label: 'Shots' },
  { key: 'kills', label: 'Kills' },
  { key: 'deaths', label: 'Deaths' },
  { key: 'accuracy', label: 'Accuracy' },
  { key: 'headHitRate', label: 'Head hit' },
  { key: 'headshotKillRate', label: 'HS kills' },
  { key: 'nonAwpTtdWeightedMs', label: 'TTD' },
  { key: 'nonAwpReactionWeightedMs', label: 'Reaction' },
  { key: 'status', label: 'Status' },
  { key: null, label: 'Actions' },
];

export function PlayerTable({
  players,
  weapons,
  scoreMode,
  onToggleSaved,
  onToggleBanned,
}: {
  players: Player[];
  weapons: PlayerWeapon[];
  scoreMode: boolean;
  onToggleSaved: (player: Player) => void;
  onToggleBanned: (player: Player) => void;
}) {
  const [stored] = useState(loadStoredState);
  const [query, setQuery] = useState('');
  const deferredQuery = useDeferredValue(query.trim().toLowerCase());
  const [filters, setFilters] = useState<TableFilters>({ ...DEFAULT_FILTERS, ...stored.filters });
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [pageSize, setPageSize] = useState<number | 'all'>(stored.pageSize ?? 12);
  const [sortKey, setSortKey] = useState<SortKey>(stored.sortKey ?? 'status');
  const [ascending, setAscending] = useState(stored.ascending ?? false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [page, setPage] = useState(0);

  const columns = scoreMode ? COLUMNS : COLUMNS.filter((column) => column.key !== 'suspicionScore');

  useEffect(() => {
    if (!scoreMode && sortKey === 'suspicionScore') setSortKey('status');
  }, [scoreMode, sortKey]);

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ filters, sortKey, ascending, pageSize }));
  }, [filters, sortKey, ascending, pageSize]);

  const shown = useMemo(() => {
    return players.filter((player) => {
      const matchesText = !deferredQuery || player.name.toLowerCase().includes(deferredQuery) || String(player.steamId).includes(deferredQuery);
      return matchesText && matchesFilters(player, filters);
    }).toSorted((a, b) => {
      // saved players pin to the top only under the default (status) sort
      if (sortKey === 'status' && a.saved !== b.saved) return a.saved ? -1 : 1;
      const order = sortKey === 'status'
        ? STATUS_RANK[a.status] - STATUS_RANK[b.status] || a.suspicionScore - b.suspicionScore
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

  const pageCount = pageSize === 'all' ? 1 : Math.max(1, Math.ceil(shown.length / pageSize));
  const currentPage = Math.min(page, pageCount - 1);
  const visible = pageSize === 'all' ? shown : shown.slice(currentPage * pageSize, (currentPage + 1) * pageSize);

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
        <Select value={String(pageSize)} onValueChange={(value) => setPageSize(value === 'all' ? 'all' : Number(value))}>
          <SelectTrigger className="w-32"><SelectValue /></SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((size) => (
              <SelectItem key={size} value={String(size)}>{size} / page</SelectItem>
            ))}
            <SelectItem value="all">All</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filtersOpen ? <FiltersPanel filters={filters} onChange={setFilters} /> : null}

      <div className="overflow-x-auto rounded-lg border border-border">
        <Table className="min-w-[1100px]">
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              {columns.map((col) => (
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
                    className={cn('player-score-row cursor-pointer', player.banned && 'bg-destructive/10 text-muted-foreground hover:bg-destructive/15')}
                    style={{ '--score-color': scoreMode && player.eligible ? scoreColor(player.suspicionScore) : undefined } as CSSProperties}
                    onClick={() => setExpanded((value) => (value === player.steamId ? null : player.steamId))}
                  >
                    {scoreMode ? (
                      <TableCell className="font-medium tabular-nums">{player.eligible ? Math.round(player.suspicionScore) : '—'}</TableCell>
                    ) : null}
                    <TableCell>
                      <div className="flex items-center gap-1.5 font-medium">
                        <span className="max-w-56 truncate" title={player.name}>{player.name}</span>
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
                    <TableCell className="tabular-nums">{ms(player.nonAwpTtdWeightedMs, player.nonAwpTtdSamples)}</TableCell>
                    <TableCell className="tabular-nums">{ms(player.nonAwpReactionWeightedMs, player.nonAwpReactionSamples)}</TableCell>
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
                      <TableCell colSpan={columns.length} className="p-0">
                        <PlayerDetails player={player} weapons={weapons.filter((weapon) => weapon.steamId === player.steamId)} scoreMode={scoreMode} />
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
        {pageCount > 1 ? (
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={currentPage === 0} onClick={() => setPage(currentPage - 1)}>
              <ChevronLeft className="size-4" />
            </Button>
            <span className="text-sm tabular-nums text-muted-foreground">{currentPage + 1} / {pageCount}</span>
            <Button variant="outline" size="sm" disabled={currentPage >= pageCount - 1} onClick={() => setPage(currentPage + 1)}>
              <ChevronRight className="size-4" />
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
