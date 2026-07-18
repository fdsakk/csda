import { useCallback, useEffect, useRef, useState } from 'react';
import { BookOpen } from 'lucide-react';
import { deleteJob, getJobs, getReport, Job, Player, Report, setPlayerBanned, setPlayerSaved } from './api';
import { Button } from '@/components/ui/button';
import { CheatSheet } from '@/components/CheatSheet';
import { DemosDialog } from '@/components/DemosDialog';
import { Dropzone } from '@/components/Dropzone';
import { JobsList } from '@/components/JobsList';
import { PlayerTable } from '@/components/PlayerTable';
import { ThresholdsDialog } from '@/components/ThresholdsDialog';

const EMPTY_REPORT: Report = { players: [], playersByWeapon: [], importedDemos: [] };

function finishedSignature(jobs: Job[]) {
  return jobs.filter((job) => job.status === 'completed' || job.status === 'failed').map((job) => job.id).join(':');
}

export default function App() {
  const [report, setReport] = useState<Report>(EMPTY_REPORT);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [guideOpen, setGuideOpen] = useState(false);
  const completedRef = useRef('');

  const loadAll = useCallback(async () => {
    try {
      const [nextReport, nextJobs] = await Promise.all([getReport(), getJobs()]);
      setReport(nextReport); setJobs(nextJobs); setError('');
      completedRef.current = finishedSignature(nextJobs);
    }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Unable to load data'); }
    finally { setLoading(false); }
  }, []);

  // optimistic flip; reload on failure to restore server state
  const patchPlayer = useCallback(async (player: Player, patch: Partial<Player>, save: () => Promise<void>) => {
    setReport((current) => ({
      ...current,
      players: (current.players ?? []).map((p) => (p.steamId === player.steamId ? { ...p, ...patch } : p)),
    }));
    try { await save(); }
    catch { void loadAll(); }
  }, [loadAll]);

  const toggleSaved = (player: Player) =>
    patchPlayer(player, { saved: !player.saved }, () => setPlayerSaved(player.steamId, !player.saved));
  const toggleBanned = (player: Player) =>
    patchPlayer(player, { banned: !player.banned }, () => setPlayerBanned(player.steamId, !player.banned));

  const dismissJob = useCallback(async (job: Job) => {
    setJobs((current) => current.filter((item) => item.id !== job.id));
    try { await deleteJob(job.id); } catch { /* SSE restores it if the delete failed */ }
  }, []);

  useEffect(() => { void loadAll(); }, [loadAll]);
  useEffect(() => {
    // The server pushes the job list over SSE whenever it changes; the browser
    // reconnects automatically. Refetch the report when the set of finished
    // jobs changes so the table updates once analysis completes.
    const source = new EventSource('/api/events');
    source.onmessage = (event) => {
      const nextJobs: Job[] = JSON.parse(event.data);
      setJobs(nextJobs);
      const signature = finishedSignature(nextJobs);
      if (signature !== completedRef.current) {
        completedRef.current = signature;
        getReport().then(setReport).catch(() => { /* banner from loadAll covers outages */ });
      }
    };
    return () => source.close();
  }, []);

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6 px-4 py-8 sm:px-6 sm:py-10">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">CS2 demo analysis</h1>
          <p className="text-sm text-muted-foreground">Upload demos to build the player baseline and review flagged accounts.</p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => setGuideOpen(true)}><BookOpen className="size-4" /> Cheat sheet</Button>
          {!loading ? <ThresholdsDialog onChanged={() => void loadAll()} /> : null}
          {!loading ? <DemosDialog demos={report.importedDemos ?? []} onChanged={() => void loadAll()} /> : null}
        </div>
      </div>

      <Dropzone onQueued={(job) => setJobs((current) => [job, ...current])} />
      <JobsList jobs={jobs} onDismiss={(job) => void dismissJob(job)} />

      {error ? <div className="rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">API offline — {error}</div> : null}

      {loading ? (
        <div className="py-20 text-center text-sm text-muted-foreground">Loading…</div>
      ) : (
        <PlayerTable
          players={report.players ?? []}
          weapons={report.playersByWeapon ?? []}
          onToggleSaved={(player) => void toggleSaved(player)}
          onToggleBanned={(player) => void toggleBanned(player)}
        />
      )}
      <CheatSheet open={guideOpen} onClose={() => setGuideOpen(false)} />
    </div>
  );
}
