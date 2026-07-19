import { ExternalLink } from 'lucide-react';
import { Player, PlayerWeapon } from '@/api';
import { ms, number, pct } from '@/lib/format';
import { cn } from '@/lib/utils';

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
  const stats: [string, string][] = [
    ...(scoreMode
      ? ([
          ['Review score', player.eligible ? `${Math.round(player.suspicionScore)} / 100` : 'low sample'],
          ['Evidence groups', `T ${Math.round(player.timingScore)} · P ${Math.round(player.precisionScore)} · K/D ${Math.round(player.performanceScore)}`],
        ] as [string, string][])
      : []),
    ['Crosshair @ exposure', `${player.crosshairMedianAngle.toFixed(1)}°`],
    ['First shot error', `${player.firstShotMedianAngle.toFixed(1)}°`],
    ['Unspotted damage', pct(player.unspottedDamageRate)],
    ['TTD (rifle)', ms(player.nonAwpTtdWeightedMs, player.nonAwpTtdSamples)],
    ['TTD (AWP)', ms(player.awpTtdWeightedMs, player.awpTtdSamples)],
    ['Reaction (rifle)', ms(player.nonAwpReactionWeightedMs, player.nonAwpReactionSamples)],
    ['TTD p10 (all)', ms(player.ttdP10Ms, player.ttdSamples)],
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
          {flagSignals.length ? (
            <div className="col-span-2 flex flex-wrap gap-2 border-t border-border pt-3">
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
        </div>
      </div>

    </div>
  );
}
