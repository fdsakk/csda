import { type ReactNode, useEffect } from 'react';
import { X } from 'lucide-react';
import { Button } from '@/components/ui/button';

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

export function CheatSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
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
            <GuideItem term="Status">A review priority, not proof. `Cheater` (red) marks stats that are not humanly reproducible over many games; `Watch` (yellow) is a grey-zone flag. Always expand the row and inspect the reason badges below the player stats before judging.</GuideItem>
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
            <GuideItem term="TTD (rifle)">Time from first spotted tick to first damage on non-AWP weapons, as a round-weighted long-term average. This is what the table column and status use. Values below the watch threshold are suspicious; values below the cheater threshold set the red status. The current bands and elite supporting-stat thresholds can be changed in Thresholds. `p10 (all)` in details is the fast 10% tail across every weapon.</GuideItem>
            <GuideItem term="TTD (AWP)">AWP-only TTD, flagged separately for anyone with enough AWP encounters. The AWP is a one-flick one-shot weapon so it has its own, lower watch and cheater bands configured in Thresholds. A player can look clean on the rifle and still get flagged here (or the reverse).</GuideItem>
            <GuideItem term="Reaction (rifle)">Time from first spotted tick to first shot on non-AWP weapons. It is a demo-derived estimate, not a laboratory reaction-time test; pre-aim, sound cues and prediction affect it. The red cutoff is configured in Thresholds.</GuideItem>
            <GuideItem term="Crosshair @ exposure">Median angular distance from crosshair to opponent at confirmed exposure. Lower means stronger crosshair placement, not cheating by itself.</GuideItem>
            <GuideItem term="First shot error">Median angular distance at the first shot. Read it together with TTD and reaction instead of treating it as a standalone verdict.</GuideItem>
          </GuideSection>

          <GuideSection title="Suspicion signals">
            <GuideItem term="Unspotted damage">Damage where the analyzer did not have a confirmed spotted state. Check the demo for sound, teammate information, wallbang lines, smokes and replay limitations before judging it.</GuideItem>
            <GuideItem term="First-bullet head / snap">Signals around unusually accurate first shots and fast aim reduction. They are strongest when repeated over many encounters and paired with unusual TTD or reactions.</GuideItem>
            <GuideItem term="Smoke / wall kills">Useful review context, but legitimate wallbangs and common angles are expected in Counter-Strike. Look for repetition and timing, not isolated kills.</GuideItem>
            <GuideItem term="Reason badges">Coloured badges below the player stats show exactly which rules set the status: red for Cheater and yellow for Watch.</GuideItem>
          </GuideSection>

          <GuideSection title="Saved players and demos">
            <GuideItem term="Bookmark">Use the bookmark in the Status column to pin a player to the top while reviewing them later.</GuideItem>
            <GuideItem term="Demos">The Demos page lets you enable or disable demos from aggregates, manage imports and exports, and clear uploaded files after analysis. A quality warning automatically excludes a demo when low timing affects several players; you can still include it manually after review.</GuideItem>
          </GuideSection>

          <GuideSection title="Decision rule">
            <p>One metric is a reason to look, not a conclusion. Prioritize players with enough samples and multiple independent signals, then validate them against the demo timeline.</p>
          </GuideSection>
        </div>
      </aside>
    </>
  );
}
