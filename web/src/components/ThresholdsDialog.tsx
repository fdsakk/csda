import { useEffect, useState } from 'react';
import { Info, RotateCcw, SlidersHorizontal } from 'lucide-react';
import { getThresholds, setThresholds, SuspicionConfig } from '@/api';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Slider } from '@/components/ui/slider';
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
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
  slider?: boolean;
  advanced?: boolean;
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
    title: 'Sample confidence',
    description: 'Timing can enter review at the hard minimum, but needs more encounters before it can carry a Cheater verdict without precision support.',
    modes: ['score'],
    fields: [
      { key: 'sampleConfidenceK', label: 'Samples to stabilize', description: 'Larger values require more encounters before timing evidence reaches full strength.', step: 1, min: 1 },
    ],
  },
  {
    title: 'Final score bands',
    description: 'The fused score is mapped to Normal, Watch or Cheater only after every evidence group has been combined.',
    modes: ['score'],
    fields: [
      { key: 'scoreWatchThreshold', label: 'Watch score', description: 'Minimum final score for Watch.', suffix: '/100', min: 0, max: 100, slider: true },
      { key: 'scoreCheaterThreshold', label: 'Cheater score', description: 'Minimum final score for Cheater.', suffix: '/100', min: 0, max: 100, slider: true },
      { key: 'scoreCurveExponent', label: 'Score curve', description: 'Below 1 expands meaningful evidence toward the top of the 0–100 scale.', step: 0.05, min: 0.05, max: 2, slider: true, advanced: true },
    ],
  },
  {
    title: 'Evidence mapping and confidence',
    description: 'Metric anchors become soft evidence rather than immediate verdicts. Sample confidence prevents a small sample from carrying full strength.',
    modes: ['score'],
    fields: [
      { key: 'metricWatchEvidence', label: 'Watch-anchor evidence', description: 'Evidence assigned at a metric’s watch anchor.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'metricCheaterEvidence', label: 'Cheater-anchor evidence', description: 'Evidence assigned at a metric’s cheater anchor.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'sampleConfidenceFloor', label: 'Minimum confidence', description: 'Evidence retained at the hard sample gate. The conservative default prevents a small sample from deciding Cheater on its own.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
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
      { key: 'awpEvidenceExponent', label: 'Evidence curve', description: 'Above 1 suppresses ordinary AWP timings while preserving truly extreme values.', step: 0.1, min: 1, max: 5, slider: true, advanced: true },
    ],
  },
  {
    title: 'Precision anchors',
    description: 'Head-hit and accuracy share one support group. They can strengthen suspicious timing but can never create a flag by themselves.',
    modes: ['score'],
    fields: [
      { key: 'eliteHeadHitRate', label: 'Head-hit evidence start', description: 'First soft head-hit evidence anchor.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'headHitWatchThreshold', label: 'Head-hit watch anchor', description: 'Middle head-hit evidence anchor.', step: 1, suffix: '%', percent: true, slider: true },
      { key: 'headHitCheaterThreshold', label: 'Head-hit cheater anchor', description: 'High head-hit evidence anchor.', step: 1, suffix: '%', percent: true, slider: true },
      { key: 'eliteAccuracy', label: 'Accuracy watch anchor', description: 'Accuracy at this value receives watch-anchor evidence.', step: 1, suffix: '%', percent: true, slider: true },
      { key: 'accuracyCheater', label: 'Accuracy cheater anchor', description: 'Accuracy at this value receives cheater-anchor evidence.', step: 1, suffix: '%', percent: true, slider: true },
    ],
  },
  {
    title: 'Performance anchors',
    description: 'K/D cannot flag a player by itself. It only supports timing evidence, and does not stack with precision support.',
    modes: ['score'],
    fields: [
      { key: 'eliteKd', label: 'K/D watch anchor', description: 'Start of K/D supporting evidence.', step: 0.1, min: 0.1, advanced: true },
      { key: 'eliteKdCheater', label: 'K/D cheater anchor', description: 'High K/D supporting evidence.', step: 0.1, min: 0.1, advanced: true },
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
      { key: 'headHitWatchThreshold', label: 'Head-hit watch bound', description: 'Head-hit rate at or above this flags Watch.', step: 1, suffix: '%', percent: true, slider: true },
      { key: 'headHitCheaterThreshold', label: 'Head-hit cheater bound', description: 'Head-hit rate at or above this flags Cheater.', step: 1, suffix: '%', percent: true, slider: true },
    ],
  },
  {
    title: 'Accuracy thresholds',
    description: 'Overall accuracy across all tracked shots.',
    modes: ['manual'],
    fields: [
      { key: 'eliteAccuracy', label: 'Accuracy watch bound', description: 'Accuracy at or above this flags Watch.', step: 1, suffix: '%', percent: true, slider: true },
      { key: 'accuracyCheater', label: 'Accuracy cheater bound', description: 'Accuracy at or above this flags Cheater.', step: 1, suffix: '%', percent: true, slider: true },
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
      { key: 'timingWeight', label: 'Timing weight', description: 'Strength of the timing group.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'awpTimingWeight', label: 'AWP timing weight', description: 'Reduces AWP evidence for one-shot kills and held angles.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'precisionWeight', label: 'Precision support weight', description: 'Strength of head-hit or accuracy support.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'performanceWeight', label: 'K/D support weight', description: 'Maximum strength of the gated K/D amplifier.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
      { key: 'synergyWeight', label: 'Maximum support bonus', description: 'Caps how strongly precision or K/D can raise timing evidence.', step: 1, suffix: '%', percent: true, slider: true, advanced: true },
    ],
  },
];

function InfoTip({ text }: { text: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" aria-label="More info" className="text-muted-foreground hover:text-foreground">
          <Info className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  );
}

function ThresholdField({ field, config, onChange }: { field: Field; config: SuspicionConfig; onChange: (key: ConfigKey, value: number) => void }) {
  const shown = field.percent ? config[field.key] * 100 : config[field.key];
  const min = field.min ?? (field.percent ? 0 : field.step === 0.1 ? 0.1 : 1);
  const max = field.max ?? (field.percent ? 100 : undefined);
  const step = field.step ?? 1;
  const commit = (raw: number) => onChange(field.key, field.percent ? raw / 100 : raw);

  const numberBox = (
    <span className="relative block w-20 flex-none">
      <Input
        type="number"
        min={min}
        max={max}
        step={step}
        value={shown}
        className={cn(
          'h-8 px-2 text-right text-sm tabular-nums [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none',
          field.suffix && 'pr-8',
        )}
        onChange={(event) => commit(Number(event.target.value))}
      />
      {field.suffix ? <span className="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{field.suffix}</span> : null}
    </span>
  );

  if (field.slider && max !== undefined) {
    return (
      <div className="col-span-2 flex flex-col gap-1.5" title={field.description}>
        <label className="text-sm font-medium text-foreground">{field.label}</label>
        <div className="flex items-center gap-4">
          <Slider
            value={[shown]}
            min={min}
            max={max}
            step={step}
            className="flex-1"
            onValueChange={(value) => commit(typeof value === 'number' ? value : value[0])}
          />
          {numberBox}
        </div>
      </div>
    );
  }

  return (
    <label className="flex flex-col gap-2" title={field.description}>
      <span className="text-sm font-medium text-foreground">{field.label}</span>
      {numberBox}
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

  const renderGroups = (advanced: boolean) => config
    ? GROUPS
      .filter((group) => !group.modes || group.modes.includes(config.flagMode))
      .map((group) => ({ ...group, fields: group.fields.filter((field) => Boolean(field.advanced) === advanced) }))
      .filter((group) => group.fields.length > 0)
      .map((group) => (
        <section key={`${advanced ? 'advanced' : 'primary'}-${group.title}`}>
          <div className="mb-4 flex items-center gap-1.5">
            <h3 className="text-base font-semibold text-muted-foreground">{group.title}</h3>
            <InfoTip text={group.description} />
          </div>
          <div className="grid grid-cols-2 gap-x-6 gap-y-4">
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
      ))
    : null;

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
      setMessage('Saved.');
      onChanged();
    } catch (cause) {
      setFailed(true);
      setMessage(cause instanceof Error ? cause.message : 'Unable to save thresholds');
    } finally {
      setSaving(false);
    }
  };

  return (
    <TooltipProvider>
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="outline" size="sm"><SlidersHorizontal className="size-4" /> Thresholds</Button>
      </SheetTrigger>
      <SheetContent className="w-full gap-0 p-0 sm:max-w-xl" aria-label="Threshold settings">
        <SheetHeader className="border-b border-border px-6 py-5 pr-12 text-left">
          <div className="flex flex-wrap items-center gap-3">
            <SheetTitle className="text-lg">Watch and cheater thresholds</SheetTitle>
            {config ? (
              <div className="flex items-center gap-2">
                <div className="inline-flex flex-none rounded-full bg-muted p-0.5">
                  {(['score', 'manual'] as const).map((mode) => (
                    <button
                      key={mode}
                      className={cn(
                        'rounded-full px-3 py-0.5 text-xs capitalize transition-colors',
                        config.flagMode === mode ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
                      )}
                      onClick={() => setConfig((current) => (current ? { ...current, flagMode: mode } : current))}
                    >
                      {mode}
                    </button>
                  ))}
                </div>
                <InfoTip
                  text={config.flagMode === 'manual'
                    ? 'Each stat is checked against its own watch and cheater bound; the worst tier wins. No fused score.'
                    : 'Evidence is fused into a 0–100 score. Timing is required; accuracy, head-hit and K/D only add bounded support.'}
                />
              </div>
            ) : null}
          </div>
          <SheetDescription>Tune the thresholds for the selected mode. Changes apply immediately.</SheetDescription>
        </SheetHeader>

        <div className="cheat-sheet-scroll min-h-0 flex-1 overflow-y-auto px-6 pt-3 pb-6">
              {loading ? <p className="py-16 text-center text-sm text-muted-foreground">Loading thresholds…</p> : null}
              {!loading && config ? (
                <div className="flex flex-col gap-8">
                  {renderGroups(false)}
                  {config.flagMode === 'score' ? (
                    <details className="rounded-lg border border-border p-4">
                      <summary className="cursor-pointer text-sm font-medium text-foreground">Advanced model tuning</summary>
                      <p className="mt-2 text-sm text-muted-foreground">Low-impact calibration controls. The defaults are intentionally conservative for small samples.</p>
                      <div className="flex flex-col gap-8 pt-6">{renderGroups(true)}</div>
                    </details>
                  ) : null}
                </div>
              ) : null}
        </div>

        <SheetFooter className="flex-row flex-wrap items-center justify-between gap-2 border-t border-border px-6 py-4">
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
    </TooltipProvider>
  );
}
