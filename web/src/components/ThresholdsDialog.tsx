import { useEffect, useState } from 'react';
import { RotateCcw, SlidersHorizontal } from 'lucide-react';
import { getThresholds, setThresholds, SuspicionConfig } from '@/api';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

type ConfigKey = keyof SuspicionConfig;

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

const GROUPS: { title: string; description: string; fields: Field[] }[] = [
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
    fields: [
      { key: 'scoreWatchThreshold', label: 'Watch score', description: 'Minimum final score for Watch.', suffix: '/100', min: 0, max: 100 },
      { key: 'scoreCheaterThreshold', label: 'Cheater score', description: 'Minimum final score for Cheater.', suffix: '/100', min: 0, max: 100 },
      { key: 'scoreCurveExponent', label: 'Score curve', description: 'Below 1 expands meaningful evidence toward the top of the 0–100 scale.', step: 0.05, min: 0.05, max: 2 },
    ],
  },
  {
    title: 'Evidence mapping and confidence',
    description: 'Metric anchors become soft evidence rather than immediate verdicts. Sample confidence prevents a small sample from carrying full strength.',
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
    fields: [
      { key: 'awpTtdWatchMs', label: 'Evidence start', description: 'AWP TTD begins producing evidence below this value.', suffix: 'ms' },
      { key: 'awpTtdCheaterMs', label: 'Cheater anchor', description: 'AWP evidence reaches full strength at this value.', suffix: 'ms' },
      { key: 'awpEvidenceExponent', label: 'Evidence curve', description: 'Above 1 suppresses ordinary AWP timings while preserving truly extreme values.', step: 0.1, min: 1, max: 5 },
    ],
  },
  {
    title: 'Precision anchors',
    description: 'Head-hit and accuracy share one support group. They can strengthen suspicious timing but can never create a flag by themselves.',
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
    fields: [
      { key: 'eliteKd', label: 'K/D watch anchor', description: 'Start of K/D supporting evidence.', step: 0.1, min: 0.1 },
      { key: 'eliteKdCheater', label: 'K/D cheater anchor', description: 'High K/D supporting evidence.', step: 0.1, min: 0.1 },
    ],
  },
  {
    title: 'Evidence fusion',
    description: 'Timing is required for every flag. Precision or K/D can add only a bounded support bonus after correlated metrics are collapsed.',
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
    <label className="space-y-1.5">
      <span className="text-sm font-medium text-foreground">{field.label}</span>
      <span className="relative block">
        <Input
          type="number"
          min={field.min ?? (field.percent ? 0 : field.step === 0.1 ? 0.1 : 1)}
          max={field.max ?? (field.percent ? 100 : undefined)}
          step={field.step ?? 1}
          value={shown}
          className={cn(field.suffix && 'pr-11')}
          onChange={(event) => {
            const parsed = Number(event.target.value);
            onChange(field.key, field.percent ? parsed / 100 : parsed);
          }}
        />
        {field.suffix ? <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">{field.suffix}</span> : null}
      </span>
      <span className="block text-xs leading-4 text-muted-foreground">{field.description}</span>
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
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger render={<Button variant="outline" size="sm"><SlidersHorizontal className="size-4" /> Thresholds</Button>} />
      <DialogContent className="max-h-[calc(100dvh-3rem)] max-w-5xl overflow-hidden p-0">
        <div className="flex max-h-[calc(100dvh-3rem)] flex-col">
          <header className="border-b border-border px-6 py-4 pr-14">
            <DialogTitle className="text-xl">Watch and cheater thresholds</DialogTitle>
            <p className="mt-1 text-sm text-muted-foreground">Tune the complete aggregate evidence score. Changes apply immediately to the current report.</p>
          </header>

          <div className="cheat-sheet-scroll min-h-0 flex-1 overflow-y-auto px-6 py-5">
            {loading ? <p className="py-16 text-center text-sm text-muted-foreground">Loading thresholds…</p> : null}
            {!loading && config ? (
              <div className="space-y-6">
                <div className="rounded-lg border border-border bg-muted/40 p-4 text-sm leading-5 text-muted-foreground">
                  <p className="font-medium text-foreground">How the score works</p>
                  <p className="mt-1">
                    Every aggregate metric becomes soft evidence from 0–100 and is reduced by sample confidence. The strongest TTD/reaction result forms the required timing core. AWP timing is down-weighted because held angles and one-shot kills naturally look faster. Head-hit, accuracy and K/D describe skill as well as cheating, so they can only add a bounded support bonus to timing and can never flag a player alone. The final curve maps the result to Normal, Watch and Cheater review bands.
                  </p>
                </div>
                {GROUPS.map((group) => (
                  <section key={group.title} className="space-y-4 border-b border-border pb-6 last:border-0 last:pb-0">
                    <div>
                      <h3 className="text-sm font-semibold">{group.title}</h3>
                      <p className="mt-1 text-xs text-muted-foreground">{group.description}</p>
                    </div>
                    <div className="grid gap-x-5 gap-y-4 sm:grid-cols-2 lg:grid-cols-3">
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

          <footer className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-6 py-4">
            <div>
              {message ? <p className={cn('text-sm', failed ? 'text-destructive' : 'text-muted-foreground')}>{message}</p> : null}
            </div>
            <div className="ml-auto flex gap-2">
              <Button variant="outline" disabled={!defaults || loading || saving} onClick={() => { if (defaults) { setConfig({ ...defaults }); setMessage(''); } }}>
                <RotateCcw className="size-4" /> Reset to defaults
              </Button>
              <Button disabled={!config || loading || saving} onClick={() => void save()}>{saving ? 'Saving…' : 'Save thresholds'}</Button>
            </div>
          </footer>
        </div>
      </DialogContent>
    </Dialog>
  );
}
