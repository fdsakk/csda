import { DragEvent, useCallback, useRef, useState } from 'react';
import { Upload, X } from 'lucide-react';
import { Job, uploadDemos } from '@/api';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

export function Dropzone({ onQueued }: { onQueued: (job: Job) => void }) {
  const input = useRef<HTMLInputElement>(null);
  const [files, setFiles] = useState<File[]>([]);
  const [dragging, setDragging] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);
  const [error, setError] = useState('');

  const accept = useCallback((incoming: File[]) => {
    const demos = incoming.filter((file) => /\.dem$/i.test(file.name));
    setError(demos.length === incoming.length ? '' : 'Only .dem files were added.');
    setFiles((current) => {
      const known = new Set(current.map((file) => `${file.name}:${file.size}`));
      return [...current, ...demos.filter((file) => !known.has(`${file.name}:${file.size}`))];
    });
  }, []);

  const drop = (event: DragEvent) => { event.preventDefault(); setDragging(false); accept([...event.dataTransfer.files]); };
  const submit = async () => {
    if (!files.length || progress !== null) return;
    setError(''); setProgress(0);
    try { const job = await uploadDemos(files, setProgress); setFiles([]); onQueued(job); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Upload failed'); }
    finally { setProgress(null); }
  };

  return (
    <div className="rounded-lg border border-border bg-card">
      <div
        className={cn(
          'flex flex-col items-center justify-center gap-2 rounded-t-lg border-b border-dashed border-border px-6 py-10 text-center transition-colors',
          dragging && 'border-ring bg-accent/50'
        )}
        onDragOver={(event) => { event.preventDefault(); setDragging(true); }}
        onDragLeave={() => setDragging(false)}
        onDrop={drop}
      >
        <input ref={input} type="file" accept=".dem" multiple hidden onChange={(event) => accept([...(event.target.files ?? [])])} />
        <div className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
          <Upload className="size-5" />
        </div>
        <p className="text-sm font-medium">Drop CS2 demo files here</p>
        <p className="text-sm text-muted-foreground">.dem recordings — drag &amp; drop or</p>
        <Button variant="outline" size="sm" onClick={() => input.current?.click()}>Browse files</Button>
      </div>

      <div className="flex flex-col gap-3 border-t border-border p-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 flex-wrap gap-2">
          {files.length ? files.map((file) => (
            <span key={`${file.name}:${file.size}`} className="flex max-w-full items-center gap-1.5 rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
              <span className="truncate">{file.name}</span>
              <button className="shrink-0 text-muted-foreground hover:text-foreground" onClick={() => setFiles((all) => all.filter((item) => item !== file))}>
                <X className="size-3" />
              </button>
            </span>
          )) : <span className="text-sm text-muted-foreground">Source detected automatically</span>}
        </div>
        <Button className="shrink-0 self-end sm:self-auto" disabled={!files.length || progress !== null} onClick={submit}>
          {progress === null
            ? `Analyze ${files.length || ''} file${files.length === 1 ? '' : 's'}`.replace('  ', ' ')
            : `Uploading ${progress}%`}
        </Button>
      </div>

      {progress !== null ? (
        <div className="mx-4 mb-4 h-1 overflow-hidden rounded-full bg-muted">
          <div className="h-full bg-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
      ) : null}
      {error ? <p className="px-4 pb-4 text-sm text-destructive">{error}</p> : null}
    </div>
  );
}
