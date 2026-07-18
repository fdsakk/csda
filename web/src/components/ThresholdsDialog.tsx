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
    title: 'Rifle timing',
    description: 'Non-AWP weighted averages. Lower values are more suspicious.',
    fields: [
      { key: 'ttdCheaterMs', label: 'Cheater TTD', description: 'Below this value sets Cheater.', suffix: 'ms' },
      { key: 'ttdSuspiciousMs', label: 'Watch TTD', description: 'Below this value sets Watch, or Cheater with elite stats.', suffix: 'ms' },
      { key: 'reactionCheaterMs', label: 'Cheater reaction', description: 'Average reaction below this value sets Cheater.', suffix: 'ms' },
    ],
  },
  {
    title: 'AWP timing',
    description: 'AWP has separate bands because one-shot flicks naturally produce lower TTD.',
    fields: [
      { key: 'awpTtdCheaterMs', label: 'Cheater TTD', description: 'AWP TTD below this value sets Cheater.', suffix: 'ms' },
      { key: 'awpTtdWatchMs', label: 'Watch TTD', description: 'AWP TTD below this value sets Watch.', suffix: 'ms' },
    ],
  },
  {
    title: 'Elite supporting stats',
    description: 'Any one of these promotes rifle TTD in the watch band to Cheater.',
    fields: [
      { key: 'eliteKd', label: 'K/D', description: 'Kills divided by deaths.', step: 0.1 },
      { key: 'eliteHeadHitRate', label: 'Head-hit rate', description: 'Share of damaging hits that hit the head.', step: 1, suffix: '%', percent: true },
      { key: 'eliteAccuracy', label: 'Accuracy', description: 'Share of tracked shots that hit.', step: 1, suffix: '%', percent: true },
    ],
  },
  {
    title: 'Standalone head-hit flags',
    description: 'Applied independently after the minimum head-hit sample is reached.',
    fields: [
      { key: 'headHitWatchThreshold', label: 'Watch threshold', description: 'Head-hit rate at or above this value sets Watch.', step: 1, suffix: '%', percent: true },
      { key: 'headHitCheaterThreshold', label: 'Cheater threshold', description: 'Head-hit rate at or above this value sets Cheater.', step: 1, suffix: '%', percent: true },
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
          min={field.percent ? 0 : field.step === 0.1 ? 0.1 : 1}
          max={field.percent ? 100 : undefined}
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
            <p className="mt-1 text-sm text-muted-foreground">Changes apply immediately to the current report. Lower timing thresholds are stricter.</p>
          </header>

          <div className="cheat-sheet-scroll min-h-0 flex-1 overflow-y-auto px-6 py-5">
            {loading ? <p className="py-16 text-center text-sm text-muted-foreground">Loading thresholds…</p> : null}
            {!loading && config ? (
              <div className="space-y-6">
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
