import { useEffect, useMemo, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { getPlayerEncounters, type PlayerEncounter } from '@/api';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { useT } from '@/lib/i18n';
import { cn } from '@/lib/utils';

const PAGE = 15;

type SortMode = 'ttd' | 'demo';
type Filter = 'all' | 'estimated' | 'excluded';

function TickButton({ tick, copied, onCopy }: { tick: number | null; copied: boolean; onCopy: (tick: number) => void }) {
  const t = useT();
  if (tick == null) return <span className="text-muted-foreground">—</span>;
  return (
    <button
      type="button"
      onClick={() => onCopy(tick)}
      title={t(`Copy "demo_gototick ${tick}"`, `Kopiuj „demo_gototick ${tick}”`)}
      className="inline-flex items-center gap-1 rounded px-1 tabular-nums transition-colors hover:bg-muted"
    >
      {tick}
      {copied ? <Check className="size-3 text-emerald-500" /> : <Copy className="size-3 text-muted-foreground" />}
    </button>
  );
}

/**
 * The encounters behind a player's aggregated timings, with the ticks needed
 * to check each one in the original demo.
 */
export function PlayerEncounters({ steamId }: { steamId: string }) {
  const t = useT();
  const [encounters, setEncounters] = useState<PlayerEncounter[] | null>(null);
  const [error, setError] = useState('');
  const [sort, setSort] = useState<SortMode>('ttd');
  const [filter, setFilter] = useState<Filter>('all');
  const [visible, setVisible] = useState(PAGE);
  const [copiedKey, setCopiedKey] = useState('');

  useEffect(() => {
    let cancelled = false;
    setEncounters(null);
    setError('');
    getPlayerEncounters(steamId)
      .then((body) => { if (!cancelled) setEncounters(body.encounters); })
      .catch((cause) => { if (!cancelled) setError(cause instanceof Error ? cause.message : 'Unable to load encounters'); });
    return () => { cancelled = true; };
  }, [steamId]);

  const rows = useMemo(() => {
    const filtered = (encounters ?? []).filter((e) => filter === 'all' || (filter === 'estimated' ? e.reactionEstimated : !e.counted));
    return sort === 'ttd' ? filtered.toSorted((a, b) => a.ttdMs - b.ttdMs) : filtered;
  }, [encounters, sort, filter]);

  const copyTick = (key: string, tick: number) => {
    void navigator.clipboard?.writeText(`demo_gototick ${tick}`).then(() => {
      setCopiedKey(key);
      setTimeout(() => setCopiedKey((current) => (current === key ? '' : current)), 1500);
    }).catch(() => { /* clipboard unavailable: the tick stays readable */ });
  };

  const filterButton = (value: Filter, label: string) => (
    <Button size="sm" variant={filter === value ? 'outline' : 'ghost'} onClick={() => { setFilter(value); setVisible(PAGE); }}>{label}</Button>
  );

  return (
    <Card className="space-y-3 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-semibold text-muted-foreground">
          {t('Encounters', 'Starcia')}{encounters ? ` · ${rows.length}${filter === 'all' ? '' : ` / ${encounters.length}`}` : ''}
        </span>
        <div className="flex flex-wrap items-center gap-1">
          {filterButton('all', t('All', 'Wszystkie'))}
          {filterButton('estimated', t('Reaction estimated', 'Reakcja estymowana'))}
          {filterButton('excluded', t('Outside 0–1000 ms', 'Poza 0–1000 ms'))}
          <span className="mx-1 h-4 w-px bg-border" />
          <Button size="sm" variant={sort === 'ttd' ? 'outline' : 'ghost'} onClick={() => setSort('ttd')}>{t('Fastest TTD', 'Najszybsze TTD')}</Button>
          <Button size="sm" variant={sort === 'demo' ? 'outline' : 'ghost'} onClick={() => setSort('demo')}>{t('By demo and round', 'Wg dema i rundy')}</Button>
        </div>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}
      {!error && !encounters ? <p className="text-sm text-muted-foreground">{t('Loading…', 'Ładowanie…')}</p> : null}
      {encounters && !encounters.length ? (
        <p className="text-sm text-muted-foreground">{t('No encounters recorded for this player in the enabled demos.', 'Brak zarejestrowanych starć tego gracza w włączonych demach.')}</p>
      ) : null}
      {encounters?.length && !rows.length ? <p className="text-sm text-muted-foreground">{t('No encounters match this filter.', 'Żadne starcie nie pasuje do filtra.')}</p> : null}

      {rows.length ? (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Demo', 'Demo')}</TableHead>
                <TableHead>{t('Round', 'Runda')}</TableHead>
                <TableHead>{t('Victim', 'Ofiara')}</TableHead>
                <TableHead title={t('First tick the target was visible', 'Pierwszy tick widoczności celu')}>{t('Spotted tick', 'Tick odsłonięcia')}</TableHead>
                <TableHead>{t('Shot tick', 'Tick strzału')}</TableHead>
                <TableHead>{t('Damage tick', 'Tick obrażeń')}</TableHead>
                <TableHead>TTD</TableHead>
                <TableHead>{t('Reaction', 'Reakcja')}</TableHead>
                <TableHead>{t('Angle', 'Kąt')}</TableHead>
                <TableHead>{t('Weapon', 'Broń')}</TableHead>
                <TableHead>{t('Dist.', 'Dyst.')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.slice(0, visible).map((e) => {
                const key = `${e.demoChecksum}:${e.roundNumber}:${e.damageTick}:${e.victimSteamId}`;
                return (
                  <TableRow key={key} className={cn(!e.counted && 'text-muted-foreground')}>
                    <TableCell>
                      <span className="block max-w-44 truncate" title={`${e.demoFileName} · ${e.demoChecksum}`}>{e.demoFileName}</span>
                      <span className="text-xs text-muted-foreground">{e.mapName}</span>
                    </TableCell>
                    <TableCell className="tabular-nums">{e.roundNumber}</TableCell>
                    <TableCell><span className="block max-w-32 truncate" title={e.victimSteamId}>{e.victimName || e.victimSteamId}</span></TableCell>
                    <TableCell><TickButton tick={e.firstSpottedTick} copied={copiedKey === `${key}:s`} onCopy={(tick) => copyTick(`${key}:s`, tick)} /></TableCell>
                    <TableCell><TickButton tick={e.firstShotTick} copied={copiedKey === `${key}:f`} onCopy={(tick) => copyTick(`${key}:f`, tick)} /></TableCell>
                    <TableCell><TickButton tick={e.damageTick} copied={copiedKey === `${key}:d`} onCopy={(tick) => copyTick(`${key}:d`, tick)} /></TableCell>
                    <TableCell className="tabular-nums">
                      {Math.round(e.ttdMs)} ms{!e.counted ? <span className="ml-1 text-xs" title={t('Outside 0–1000 ms: not in the aggregates', 'Poza 0–1000 ms: nie wchodzi do agregatów')}>⊘</span> : null}
                    </TableCell>
                    <TableCell className="tabular-nums">
                      {e.reactionMs == null ? '—' : (
                        <>
                          {e.reactionEstimated ? '~' : ''}{Math.round(e.reactionMs)} ms
                          {e.reactionEstimated ? <span className="ml-1 text-xs text-muted-foreground" title={t('No shot recorded: the reaction equals the time to damage', 'Brak zarejestrowanego strzału: reakcja równa czasowi do obrażeń')}>est.</span> : null}
                        </>
                      )}
                    </TableCell>
                    <TableCell className="tabular-nums" title={e.firstShotAngle == null ? undefined : t(`First shot ${e.firstShotAngle.toFixed(1)}°`, `Pierwszy strzał ${e.firstShotAngle.toFixed(1)}°`)}>
                      {e.confirmedAngle.toFixed(1)}°{e.snap ? <span className="ml-1 text-xs text-amber-600 dark:text-amber-400">snap</span> : null}
                    </TableCell>
                    <TableCell>{e.weaponName}</TableCell>
                    <TableCell className="tabular-nums">{e.distanceMeters.toFixed(1)} m</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </div>
      ) : null}

      {rows.length > visible ? (
        <Button size="sm" variant="outline" onClick={() => setVisible((value) => value + PAGE)}>
          {t(`Show ${Math.min(PAGE, rows.length - visible)} more`, `Pokaż ${Math.min(PAGE, rows.length - visible)} więcej`)}
        </Button>
      ) : null}
      <p className="text-xs text-muted-foreground">
        {t('Ticks are in-game server ticks. Open the original demo and use "demo_gototick <tick>" (click a tick to copy it) to check an encounter.', 'Ticki to ticki serwera. Otwórz oryginalne demo i użyj „demo_gototick <tick>” (kliknij tick, aby go skopiować), aby sprawdzić starcie.')}
      </p>
    </Card>
  );
}
