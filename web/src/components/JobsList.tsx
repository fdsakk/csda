import { CheckCircle2, X, XCircle } from 'lucide-react';
import { Job } from '@/api';
import { cn } from '@/lib/utils';

export function JobsList({ jobs, onDismiss }: { jobs: Job[]; onDismiss: (job: Job) => void }) {
  const active = jobs.filter((job) => job.status === 'queued' || job.status === 'running');
  const finished = jobs.filter((job) => job.status === 'completed' || job.status === 'failed');
  if (!active.length && !finished.length) return null;
  return (
    <div className="space-y-3">
      {active.map((job) => {
        const total = job.total || job.files.length || 1;
        const running = job.status === 'running';
        const percent = running ? Math.round(job.progress || (job.processed / total) * 100) : 0;
        return (
          <div key={job.id} className="rounded-lg border border-border bg-card p-4">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="flex min-w-0 items-center gap-2">
                <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-warning" />
                <span className="truncate text-foreground">{job.files.join(', ')}</span>
              </span>
              <span className="shrink-0 tabular-nums text-muted-foreground">
                {running ? `Analyzing ${job.processed}/${total} · ${percent}%` : 'Queued'}
              </span>
            </div>
            <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-muted">
              <div
                className={cn('h-full bg-primary transition-all duration-300', !running && 'animate-pulse w-1/3')}
                style={running ? { width: `${percent}%` } : undefined}
              />
            </div>
          </div>
        );
      })}

      {finished.map((job) => {
        const failed = job.status === 'failed';
        const result = job.result;
        return (
          <div
            key={job.id}
            className={cn(
              'flex items-start justify-between gap-3 rounded-lg border p-4',
              failed ? 'border-destructive/40 bg-destructive/5' : 'border-border bg-card'
            )}
          >
            <div className="min-w-0 space-y-1 text-sm">
              <div className="flex items-center gap-2">
                {failed ? (
                  <XCircle className="size-4 shrink-0 text-destructive" />
                ) : (
                  <CheckCircle2 className="size-4 shrink-0 text-primary" />
                )}
                <span className="truncate font-medium">{job.files.join(', ')}</span>
              </div>
              {result ? (
                <p className="text-muted-foreground">
                  Imported {result.imported}, skipped {result.skipped}
                  {result.failed ? `, failed ${result.failed}` : ''}
                </p>
              ) : null}
              {failed && job.error ? <p className="text-destructive">{job.error}</p> : null}
              {result?.errors?.map((item) => (
                <p key={item.path} className="break-words text-xs text-destructive">
                  {item.path.split('/').pop()}: {item.error}
                </p>
              ))}
            </div>
            <button
              className="shrink-0 text-muted-foreground hover:text-foreground"
              title="Dismiss"
              onClick={() => onDismiss(job)}
            >
              <X className="size-4" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
