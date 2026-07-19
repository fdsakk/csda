import { useEffect, useState } from 'react';
import { RotateCcw, SlidersHorizontal } from 'lucide-react';
import { getThresholds, setThresholds, SuspicionConfig } from '@/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { cn } from '@/lib/utils';

type ConfigKey = Exclude<keyof SuspicionConfig, 'flagMode'>;

type Field = {
  key: ConfigKey;
  label: string;
  description: string;
  step?: number;
  suffix?: string;
  percent?: boolean;
  min?: number;
  max?: number;
};

type Group = { title: string; description: string; modes?: SuspicionConfig['flagMode'][]; fields: Field[] };

const GROUPS: Group[] = [
  {
    title: 'Minimum evidence',
    description: 'A player stays in Low sample until both player-level minimums are met. Timing rules also require their own encounter sample.',
    fields: [
      { key: 'minimumDemos', label: 'Minimum demos', description: 'Unique enabled demos required.' },
      { key: 'minimumShots', label: 'Minimum shots', description: 'Tracked firearm shots required.' },
      { key: 'ttdMinimumSamples', label: 'Timing samples', description: 'Encounters required for each TTD or reaction rule.' },
      { key: 'headHitMinimumEvents', label: 'Head-hit samples', description: 'Damage events required for standalone head-hit flags.' },
    ],
  },
  {
    title: 'Final score bands',
    description: 'The fused score is mapped to Normal, Watch or Cheater only after every evidence group has been combined.',
    modes: ['score'],
    fields: [
      { key: 'scoreWatchThreshold', label: 'Watch score', description: 'Minimum final score for Watch.', suffix: '/100', min: 0, max: 100 },
      { key: 'scoreCheaterThreshold', label: 'Cheater score', description: 'Minimum final score for Cheater.', suffix: '/100', min: 0, max: 100 },
      { key: 'scoreCurveExponent', label: 'Score curve', description: 'Below 1 expands meaningful evidence toward the top of the 0–100 scale.', step: 0.05, min: 0.05, max: 2 },
    ],
  },
  {
    title: 'Evidence mapping and confidence',
    description: 'Metric anchors become soft evidence rather than immediate verdicts. Sample confidence prevents a small sample from carrying full strength.',
    modes: ['score'],
    fields: [
      { key: 'metricWatchEvidence', label: 'Watch-anchor evidence', description: 'Evidence assigned at a metric’s watch anchor.', step: 1, suffix: '%', percent: true },
      { key: 'metricCheaterEvidence', label: 'Cheater-anchor evidence', description: 'Evidence assigned at a metric’s cheater anchor.', step: 1, suffix: '%', percent: true },
      { key: 'sampleConfidenceFloor', label: 'Confidence floor', description: 'Minimum retained evidence once the hard sample gate is met.', step: 1, suffix: '%', percent: true },
      { key: 'sampleConfidenceK', label: 'Confidence K', description: 'Samples needed to recover half of the confidence above the floor.', step: 1, min: 1 },
    ],
  },
  {
    title: 'Rifle timing anchors',
    description: 'Non-AWP weighted averages. Lower values produce stronger timing evidence.',
    modes: ['score'],
    fields: [
      { key: 'ttdSuspiciousMs', label: 'TTD watch anchor', description: 'TTD at this value receives watch-anchor evidence.', suffix: 'ms' },
      { key: 'ttdCheaterMs', label: 'TTD cheater anchor', description: 'TTD at this value receives cheater-anchor evidence.', suffix: 'ms' },
      { key: 'reactionWatchMs', label: 'Reaction watch anchor', description: 'Reaction time at this value receives watch-anchor evidence.', suffix: 'ms' },
      { key: 'reactionCheaterMs', label: 'Reaction cheater anchor', description: 'Reaction time at this value receives cheater-anchor evidence.', suffix: 'ms' },
    ],
  },
  {
    title: 'AWP timing anchors',
    description: 'AWP has separate anchors because one-shot flicks naturally produce lower TTD.',
    modes: ['score'],
    fields: [
      { key: 'awpTtdWatchMs', label: 'Evidence start', description: 'AWP TTD begins producing evidence below this value.', suffix: 'ms' },
      { key: 'awpTtdCheaterMs', label: 'Cheater anchor', description: 'AWP evidence reaches full strength at this value.', suffix: 'ms' },
      { key: 'awpEvidenceExponent', label: 'Evidence curve', description: 'Above 1 suppresses ordinary AWP timings while preserving truly extreme values.', step: 0.1, min: 1, max: 5 },
    ],
  },
  {
    title: 'Precision anchors',
    description: 'Head-hit and accuracy share one support group. They can strengthen suspicious timing but can never create a flag by themselves.',
    modes: ['score'],
    fields: [
      { key: 'eliteHeadHitRate', label: 'Head-hit evidence start', description: 'First soft head-hit evidence anchor.', step: 1, suffix: '%', percent: true },
      { key: 'headHitWatchThreshold', label: 'Head-hit watch anchor', description: 'Middle head-hit evidence anchor.', step: 1, suffix: '%', percent: true },
      { key: 'headHitCheaterThreshold', label: 'Head-hit cheater anchor', description: 'High head-hit evidence anchor.', step: 1, suffix: '%', percent: true },
      { key: 'eliteAccuracy', label: 'Accuracy watch anchor', description: 'Accuracy at this value receives watch-anchor evidence.', step: 1, suffix: '%', percent: true },
      { key: 'accuracyCheater', label: 'Accuracy cheater anchor', description: 'Accuracy at this value receives cheater-anchor evidence.', step: 1, suffix: '%', percent: true },
    ],
  },
  {
    title: 'Performance anchors',
    description: 'K/D cannot flag a player by itself. It only supports timing evidence, and does not stack with precision support.',
    modes: ['score'],
    fields: [
      { key: 'eliteKd', label: 'K/D watch anchor', description: 'Start of K/D supporting evidence.', step: 0.1, min: 0.1 },
      { key: 'eliteKdCheater', label: 'K/D cheater anchor', description: 'High K/D supporting evidence.', step: 0.1, min: 0.1 },
    ],
  },
  {
    title: 'Rifle timing thresholds',
    description: 'Non-AWP weighted averages. Below the watch bound → Watch, below the cheater bound → Cheater.',
    modes: ['manual'],
    fields: [
      { key: 'ttdSuspiciousMs', label: 'TTD watch bound', description: 'TTD below this flags Watch.', suffix: 'ms' },
      { key: 'ttdCheaterMs', label: 'TTD cheater bound', description: 'TTD below this flags Cheater.', suffix: 'ms' },
      { key: 'reactionWatchMs', label: 'Reaction watch bound', description: 'Reaction below this flags Watch.', suffix: 'ms' },
      { key: 'reactionCheaterMs', label: 'Reaction cheater bound', description: 'Reaction below this flags Cheater.', suffix: 'ms' },
    ],
  },
  {
    title: 'AWP timing thresholds',
    description: 'AWP has separate, lower bounds because one-shot flicks naturally produce lower TTD.',
    modes: ['manual'],
    fields: [
      { key: 'awpTtdWatchMs', label: 'AWP watch bound', description: 'AWP TTD below this flags Watch.', suffix: 'ms' },
      { key: 'awpTtdCheaterMs', label: 'AWP cheater bound', description: 'AWP TTD below this flags Cheater.', suffix: 'ms' },
    ],
  },
  {
    title: 'Head-hit thresholds',
    description: 'Gated by the head-hit sample minimum. At or above a bound → flag.',
    modes: ['manual'],
    fields: [
      { key: 'headHitWatchThreshold', label: 'Head-hit watch bound', description: 'Head-hit rate at or above this flags Watch.', step: 1, suffix: '%', percent: true },
      { key: 'headHitCheaterThreshold', label: 'Head-hit cheater bound', description: 'Head-hit rate at or above this flags Cheater.', step: 1, suffix: '%', percent: true },
    ],
  },
  {
    title: 'Accuracy thresholds',
    description: 'Overall accuracy across all tracked shots.',
    modes: ['manual'],
    fields: [
      { key: 'eliteAccuracy', label: 'Accuracy watch bound', description: 'Accuracy at or above this flags Watch.', step: 1, suffix: '%', percent: true },
      { key: 'accuracyCheater', label: 'Accuracy cheater bound', description: 'Accuracy at or above this flags Cheater.', step: 1, suffix: '%', percent: true },
    ],
  },
  {
    title: 'K/D thresholds',
    description: 'Kill/death ratio across all enabled demos.',
    modes: ['manual'],
    fields: [
      { key: 'eliteKd', label: 'K/D watch bound', description: 'K/D at or above this flags Watch.', step: 0.1, min: 0.1 },
      { key: 'eliteKdCheater', label: 'K/D cheater bound', description: 'K/D at or above this flags Cheater.', step: 0.1, min: 0.1 },
    ],
  },
  {
    title: 'Evidence fusion',
    description: 'Timing is required for every flag. Precision or K/D can add only a bounded support bonus after correlated metrics are collapsed.',
    modes: ['score'],
    fields: [
      { key: 'timingWeight', label: 'Timing weight', description: 'Strength of the timing group.', step: 1, suffix: '%', percent: true },
      { key: 'awpTimingWeight', label: 'AWP timing weight', description: 'Reduces AWP evidence for one-shot kills and held angles.', step: 1, suffix: '%', percent: true },
      { key: 'precisionWeight', label: 'Precision support weight', description: 'Strength of head-hit or accuracy support.', step: 1, suffix: '%', percent: true },
      { key: 'performanceWeight', label: 'K/D support weight', description: 'Maximum strength of the gated K/D amplifier.', step: 1, suffix: '%', percent: true },
      { key: 'synergyWeight', label: 'Maximum support bonus', description: 'Caps how strongly precision or K/D can raise timing evidence.', step: 1, suffix: '%', percent: true },
    ],
  },
];

function ThresholdField({ field, config, onChange }: { field: Field; config: SuspicionConfig; onChange: (key: ConfigKey, value: number) => void }) {
  const shown = field.percent ? config[field.key] * 100 : config[field.key];
  return (
    <label className="w-28 max-w-full flex-none" title={field.description}>
      <span className="mb-1 flex min-h-10 items-end whitespace-nowrap text-[10px] font-medium leading-4 text-foreground">{field.label}</span>
      <span className="relative block w-full">
        <Input
          type="number"
          min={field.min ?? (field.percent ? 0 : field.step === 0.1 ? 0.1 : 1)}
          max={field.max ?? (field.percent ? 100 : undefined)}
          step={field.step ?? 1}
          value={shown}
          className={cn(
            'text-right tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none',
            field.suffix && 'pr-10',
          )}
          onChange={(event) => {
            const parsed = Number(event.target.value);
            onChange(field.key, field.percent ? parsed / 100 : parsed);
          }}
        />
        {field.suffix ? <span className="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{field.suffix}</span> : null}
      </span>
    </label>
  );
}

export function ThresholdsDialog({ onChanged }: { onChanged: () => void }) {
  const [open, setOpen] = useState(false);
  const [config, setConfig] = useState<SuspicionConfig | null>(null);
  const [defaults, setDefaults] = useState<SuspicionConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState('');
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!open) return;
    setLoading(true);
    setMessage('');
    getThresholds()
      .then((result) => { setConfig(result.current); setDefaults(result.defaults); })
      .catch((cause) => { setFailed(true); setMessage(cause instanceof Error ? cause.message : 'Unable to load thresholds'); })
      .finally(() => setLoading(false));
  }, [open]);

  const save = async () => {
    if (!config) return;
    setSaving(true);
    try {
      const saved = await setThresholds(config);
      setConfig(saved);
      setFailed(false);
      setMessage('Thresholds saved. Player statuses were recalculated.');
      onChanged();
    } catch (cause) {
      setFailed(true);
      setMessage(cause instanceof Error ? cause.message : 'Unable to save thresholds');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="outline" size="sm"><SlidersHorizontal className="size-4" /> Thresholds</Button>
      </SheetTrigger>
      <SheetContent className="w-full gap-0 p-0 sm:max-w-3xl" aria-label="Threshold settings">
        <SheetHeader className="border-b border-border px-5 py-4 pr-12 text-left">
          <SheetTitle className="text-base">Watch and cheater thresholds</SheetTitle>
          <SheetDescription>Choose the flagging mode and tune its thresholds. Changes apply immediately to the current report.</SheetDescription>
        </SheetHeader>

        <div className="cheat-sheet-scroll min-h-0 flex-1 overflow-y-auto px-5 py-4">
              {loading ? <p className="py-16 text-center text-sm text-muted-foreground">Loading thresholds…</p> : null}
              {!loading && config ? (
                <div>
                  <div className="flex items-center gap-3 border-b border-border pb-4">
                    <div className="inline-flex flex-none rounded-md border border-border p-0.5">
                      {(['score', 'manual'] as const).map((mode) => (
                        <button
                          key={mode}
                          className={cn(
                            'rounded-[5px] px-3 py-1 text-sm capitalize',
                            config.flagMode === mode ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
                          )}
                          onClick={() => setConfig((current) => (current ? { ...current, flagMode: mode } : current))}
                        >
                          {mode}
                        </button>
                      ))}
                    </div>
                    <p className="text-xs leading-4 text-muted-foreground">
                      {config.flagMode === 'manual'
                        ? 'Each stat is checked against its own watch and cheater bound; the worst tier wins. No fused score.'
                        : 'Evidence is fused into a 0–100 score. Timing is required; accuracy, head-hit and K/D only add bounded support.'}
                    </p>
                  </div>
                  {GROUPS.filter((group) => !group.modes || group.modes.includes(config.flagMode)).map((group) => (
                    <section key={group.title} className="border-b border-border py-4 last:border-0">
                      <div className="mb-3">
                        <h3 className="text-sm font-semibold text-foreground">{group.title}</h3>
                        <p className="mt-0.5 text-xs leading-4 text-muted-foreground">{group.description}</p>
                      </div>
                      <div className="flex flex-wrap gap-3">
                        {group.fields.map((field) => (
                          <ThresholdField
                            key={field.key}
                            field={field}
                            config={config}
                            onChange={(key, value) => setConfig((current) => current ? { ...current, [key]: value } : current)}
                          />
                        ))}
                      </div>
                    </section>
                  ))}
                </div>
              ) : null}
        </div>

        <SheetFooter className="flex-row flex-wrap items-center justify-between gap-2 border-t border-border px-5 py-3">
              <div>{message ? <p className={cn('text-sm', failed ? 'text-destructive' : 'text-muted-foreground')}>{message}</p> : null}</div>
              <div className="ml-auto flex gap-2">
                <Button size="sm" variant="outline" disabled={!defaults || loading || saving} onClick={() => { if (defaults) { setConfig((current) => ({ ...defaults, flagMode: current?.flagMode ?? defaults.flagMode })); setMessage(''); } }}>
                  <RotateCcw className="size-4" /> Reset to defaults
                </Button>
                <Button size="sm" disabled={!config || loading || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save thresholds'}</Button>
              </div>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
