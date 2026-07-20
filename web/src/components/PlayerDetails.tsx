import { Gamepad2, Info } from 'lucide-react';
import { Player, PlayerWeapon } from '@/api';
import { Card } from '@/components/ui/card';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { ms, number, pct } from '@/lib/format';
import { useT } from '@/lib/i18n';
import { cn } from '@/lib/utils';

const HIST_BIN_MS = 50;

function InfoTip({ text, label }: { text: string; label?: string }) {
  const t = useT();
  const resolvedLabel = label ?? t('More info', 'Więcej informacji');
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" aria-label={resolvedLabel} className="text-muted-foreground transition-colors hover:text-foreground">
          <Info className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  );
}

function Histogram({
  title,
  bins,
  samples,
  medianMs,
  p10Ms,
  color,
  className,
  help,
}: {
  title: string;
  bins: number[] | null;
  samples: number;
  medianMs: number;
  p10Ms: number;
  color: string;
  className?: string;
  help: string;
}) {
  const data = bins && bins.length ? bins : [];
  const max = Math.max(1, ...data);
  const width = 240;
  const height = 56;
  const barW = width / 20;
  const x = (msValue: number) => Math.min(width, (msValue / 1000) * width);
  return (
    <Card className={cn('h-full p-3', className)}>
      <div className="flex items-baseline justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs font-medium text-foreground">
          <span className="size-2 rounded-full" style={{ background: color }} />
          {title}
          <InfoTip text={help} label={`About ${title}`} />
        </span>
        <span className="text-xs tabular-nums text-muted-foreground">
          {samples ? <>median <span className="font-medium text-foreground">{Math.round(medianMs)} ms</span></> : 'no samples'}
        </span>
      </div>
      <svg viewBox={`0 0 ${width} ${height + 14}`} className="mt-auto w-full pt-6" role="img" aria-label={`${title} distribution`}>
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
            <line x1={x(p10Ms)} y1="2" x2={x(p10Ms)} y2={height} stroke="var(--foreground)" strokeOpacity="0.75" strokeWidth="1.5" strokeDasharray="3 2" />
            <line x1={x(medianMs)} y1="0" x2={x(medianMs)} y2={height} stroke="var(--foreground)" strokeWidth="1.5" />
          </>
        ) : null}
        {[0, 250, 500, 750, 1000].map((tick) => (
          <text key={tick} x={tick === 0 ? 1 : tick === 1000 ? width - 1 : x(tick)} y={height + 11} fontSize="8.5" fill="var(--muted-foreground)" textAnchor={tick === 0 ? 'start' : tick === 1000 ? 'end' : 'middle'}>
            {tick}
          </text>
        ))}
      </svg>
    </Card>
  );
}

function BarRow({ label, detail, value, color, reference }: { label: string; detail: string; value: number; color: string; reference?: number }) {
  return (
    <div className="grid grid-cols-[7.5rem_1fr_3.5rem] items-center gap-2 text-sm">
      <span className="truncate font-medium text-foreground" title={label}>{label}</span>
      <div className="relative h-3 overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full" style={{ width: `${Math.min(100, value * 100)}%`, background: color }} />
        {reference !== undefined ? (
          <div
            className="absolute inset-y-0 w-px bg-foreground/60"
            style={{ left: `${Math.min(100, reference * 100)}%` }}
            title={`overall ${pct(reference)}`}
          />
        ) : null}
      </div>
      <span className="text-right text-[13px] tabular-nums text-muted-foreground" title={detail}>{pct(value)}</span>
    </div>
  );
}

const RULE_LABEL: Record<string, string> = {
  ttd_score: 'Rifle TTD evidence',
  awp_ttd_score: 'AWP TTD evidence',
  reaction_score: 'Rifle reaction evidence',
  head_hit_score: 'Head-hit evidence',
  accuracy_score: 'Accuracy evidence',
  kd_score: 'K/D support',
  ttd: 'Rifle TTD',
  awp_ttd: 'AWP TTD',
  reaction: 'Rifle reaction',
  head_hit_rate: 'Head-hit rate',
  accuracy: 'Accuracy',
  kd: 'K/D',
};

export function PlayerDetails({ player, weapons, scoreMode }: { player: Player; weapons: PlayerWeapon[]; scoreMode: boolean }) {
  const t = useT();
  const rules = player.triggeredRules ?? [];
  const flagSignals = rules.filter((rule) => rule.tier === 'watch' || rule.tier === 'cheater');
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
  const scoreStats: [string, string][] = scoreMode
    ? [
        ['Review score', player.eligible ? `${Math.round(player.suspicionScore)} / 100` : 'low sample'],
        ['Evidence groups', `T ${Math.round(player.timingScore)} · P ${Math.round(player.precisionScore)} · K/D ${Math.round(player.performanceScore)}`],
      ]
    : [];
  const aimStats: [string, string][] = [
    ['Crosshair @ exposure', `${player.crosshairMedianAngle.toFixed(1)}°`],
    ['First shot error', `${player.firstShotMedianAngle.toFixed(1)}°`],
    ['Unspotted damage', pct(player.unspottedDamageRate)],
    ['Smoke / wall kills', `${player.smokeKills} / ${player.wallKills}`],
  ];
  const timingStats: [string, string][] = [
    ['TTD (rifle)', ms(player.nonAwpTtdWeightedMs, player.nonAwpTtdSamples)],
    ['TTD (AWP)', ms(player.awpTtdWeightedMs, player.awpTtdSamples)],
    ['TTD p10 (all)', ms(player.ttdP10Ms, player.ttdSamples)],
    ['Reaction (rifle)', ms(player.nonAwpReactionWeightedMs, player.nonAwpReactionSamples)],
  ];
  return (
    <TooltipProvider>
    <div className="space-y-3 border-l-2 border-primary/40 bg-muted/40 p-4">
      <div className="grid gap-3 lg:grid-cols-3">
        <Histogram
          title="Time to damage"
          bins={player.ttdHistogram}
          samples={player.ttdSamples}
          medianMs={player.ttdWeightedMs}
          p10Ms={player.ttdP10Ms}
          color="var(--chart-1)"
          className="lg:col-start-1 lg:row-start-1"
          help={t('The solid line marks the median time. The brighter dashed line marks p10: the threshold reached by the fastest 10% of encounters.', 'Ciągła linia oznacza medianę czasu. Jaśniejsza przerywana linia oznacza p10: próg osiągany przez najszybsze 10% starć.')}
        />
        <Histogram
          title="Reaction time (first shot)"
          bins={player.reactionHistogram}
          samples={player.reactionSamples}
          medianMs={player.reactionWeightedMs}
          p10Ms={player.reactionP10Ms}
          color="var(--chart-2)"
          className="lg:col-start-2 lg:row-start-1"
          help={t('The solid line marks the median reaction time. The brighter dashed line marks p10: the threshold reached by the fastest 10% of reactions.', 'Ciągła linia oznacza medianę czasu reakcji. Jaśniejsza przerywana linia oznacza p10: próg osiągany przez najszybsze 10% reakcji.')}
        />
        <Card className="space-y-2 p-3 lg:col-start-1 lg:row-start-2">
          <span className="mb-3 block text-sm font-semibold text-muted-foreground">Kills by weapon</span>
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
        </Card>

        <Card className="space-y-2 p-3 lg:col-start-2 lg:row-start-2">
          <span className="mb-3 flex items-center gap-1.5 text-sm font-semibold text-muted-foreground">
            Accuracy
            <InfoTip text={t(`The vertical marker on every bar shows the player's overall accuracy (${pct(player.accuracy)}).`, `Pionowy znacznik na każdym pasku pokazuje ogólne accuracy gracza (${pct(player.accuracy)}).`)} label="About accuracy markers" />
          </span>
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
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">Not enough weapon data.</p>
          )}

          <div className="border-t border-border pt-2">
            <span className="text-sm font-semibold text-muted-foreground">Situational</span>
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
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No situational shots tracked.</p>
          )}
        </Card>

        <Card className="content-start space-y-4 p-3 lg:col-start-3 lg:row-span-2 lg:row-start-1">
          <div className="flex flex-wrap gap-2 border-b border-border pb-3">
            <a
              href={`https://steamcommunity.com/profiles/${player.steamId}`}
              target="_blank"
              rel="noreferrer"
              title={`Open ${player.steamId} on Steam`}
              aria-label={`Open ${player.steamId} on Steam`}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-muted/50 px-3 text-sm font-semibold text-foreground transition-colors hover:bg-muted"
            >
              <Gamepad2 className="size-4 text-[#66c0f4]" />
              Steam
            </a>
            <a
              href={`https://csrep.gg/player/${encodeURIComponent(player.steamId)}`}
              target="_blank"
              rel="noreferrer"
              title={`Open ${player.steamId} on CSRep`}
              aria-label={`Open ${player.steamId} on CSRep`}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-muted/50 px-3 text-sm font-semibold text-foreground transition-colors hover:bg-muted"
            >
              <img src="/csrep-icon.png" alt="" className="size-5 rounded-sm" />
              CSRep
            </a>
            <a
              href={`https://rank.uwujka.pl/?steam=${encodeURIComponent(player.steamId)}`}
              target="_blank"
              rel="noreferrer"
              title={`Open ${player.steamId} on uwujka rank`}
              aria-label={`Open ${player.steamId} on uwujka rank`}
              className="inline-flex h-9 items-center gap-2 rounded-md border border-border bg-muted/50 px-3 text-sm font-semibold text-foreground transition-colors hover:bg-muted"
            >
              <img src="/favicon.ico" alt="" className="size-4" />
              uwujka
            </a>
          </div>

          {scoreStats.length ? (
            <div className="grid grid-cols-2 gap-x-4 gap-y-3">
              {scoreStats.map(([label, value]) => (
                <div key={label} className="flex flex-col gap-0.5">
                  <span className="text-sm text-muted-foreground">{label}</span>
                  <span className="text-base font-semibold tabular-nums">{value}</span>
                </div>
              ))}
            </div>
          ) : null}

          <div className="grid grid-cols-2 gap-x-4 gap-y-3">
            {aimStats.map(([label, value]) => (
              <div key={label} className="flex flex-col gap-0.5">
                <span className="text-sm text-muted-foreground">{label}</span>
                <span className="text-base font-semibold tabular-nums">{value}</span>
              </div>
            ))}
          </div>

          <div className="grid grid-cols-2 gap-x-4 gap-y-3 border-t border-border pt-3">
            {timingStats.map(([label, value]) => (
              <div key={label} className="flex flex-col gap-0.5">
                <span className="text-sm text-muted-foreground">{label}</span>
                <span className="text-base font-semibold tabular-nums">{value}</span>
              </div>
            ))}
          </div>

          {flagSignals.length ? (
            <div className="flex flex-wrap gap-2 border-t border-border pt-3">
              {flagSignals.map((rule) => (
                <span
                  key={rule.name}
                  title={rule.score ? `${Math.round(rule.score)} evidence · n=${number.format(rule.sample)}` : `n=${number.format(rule.sample)}`}
                  className={cn(
                    'flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs',
                    rule.tier === 'cheater'
                      ? 'border-destructive/40 bg-destructive/10 text-destructive'
                      : 'border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400',
                  )}
                >
                  <span className="font-medium">{RULE_LABEL[rule.name] ?? rule.name.replaceAll('_', ' ')}</span>
                </span>
              ))}
            </div>
          ) : null}
        </Card>
      </div>

    </div>
    </TooltipProvider>
  );
}
