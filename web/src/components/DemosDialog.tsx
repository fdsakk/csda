import { Fragment, useMemo, useRef, useState } from 'react';
import { FileDown, FileUp, Film, ShieldAlert, Trash2 } from 'lucide-react';
import { clearUploads, deleteDemo, Demo, importStats, setAllDemosEnabled, setDemoEnabled } from '@/api';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

const QUALITY_REASON_LABEL: Record<string, string> = {
  systemic_low_timing: 'Systemically low TTD and reaction across players',
};

export function DemosDialog({ demos, onChanged }: { demos: Demo[]; onChanged: () => void }) {
  const fileInput = useRef<HTMLInputElement>(null);
  const [pending, setPending] = useState<string | null>(null);
  const [message, setMessage] = useState('');
  const [failed, setFailed] = useState(false);
  const enabledCount = demos.filter((demo) => demo.enabled).length;
  const warningCount = demos.filter((demo) => demo.qualityStatus === 'warning').length;

  const sorted = useMemo(() => demos.toSorted((a, b) => (b.date || '').localeCompare(a.date || '')), [demos]);
  const dayKey = (demo: Demo) => (demo.date ? demo.date.slice(0, 10) : 'unknown');
  const dayGroups = useMemo(() => {
    const groups = new Map<string, Demo[]>();
    for (const demo of sorted) {
      const key = dayKey(demo);
      const group = groups.get(key);
      if (group) group.push(demo);
      else groups.set(key, [demo]);
    }
    return [...groups.entries()];
  }, [sorted]);

  const report = (text: string, isError: boolean) => { setMessage(text); setFailed(isError); };

  const toggle = async (demo: Demo) => {
    setPending(demo.checksum);
    try { await setDemoEnabled(demo.checksum, !demo.enabled); setMessage(''); onChanged(); }
    catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
    finally { setPending(null); }
  };

  const remove = async (demo: Demo) => {
    if (!window.confirm(`Permanently delete ${demo.fileName} and all of its stats?`)) return;
    setPending(demo.checksum);
    try { await deleteDemo(demo.checksum); setMessage(''); onChanged(); }
    catch (cause) { report(cause instanceof Error ? cause.message : 'Delete failed', true); }
    finally { setPending(null); }
  };

  const toggleDay = async (key: string, target: boolean) => {
    setPending(key);
    try {
      const changed = demos.filter((demo) => dayKey(demo) === key && demo.enabled !== target);
      await Promise.all(changed.map((demo) => setDemoEnabled(demo.checksum, target)));
      setMessage('');
      onChanged();
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
    finally { setPending(null); }
  };

  const toggleAll = async (enabled: boolean) => {
    setPending('all');
    try {
      await setAllDemosEnabled(enabled);
      setMessage('');
      onChanged();
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Toggle failed', true); }
    finally { setPending(null); }
  };

  const clearUploadStorage = async () => {
    if (!window.confirm('Delete all uploaded demo files? Analyzed statistics will remain in the database.')) return;
    setPending('uploads');
    try {
      await clearUploads();
      report('Upload storage cleared. Analyzed statistics were kept.', false);
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Unable to clear uploads', true); }
    finally { setPending(null); }
  };

  const importFile = async (file: File) => {
    try {
      const result = await importStats(file);
      report(`Imported ${result.imported} demo${result.imported === 1 ? '' : 's'}, skipped ${result.skipped} already known.`, false);
      onChanged();
    } catch (cause) { report(cause instanceof Error ? cause.message : 'Import failed', true); }
  };

  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button variant="outline" size="sm">
          <Film className="size-4" /> Demos ({enabledCount}/{demos.length})
        </Button>
      </SheetTrigger>
      <SheetContent className="w-full gap-0 overflow-hidden p-0 sm:max-w-6xl">
        <TooltipProvider>
        <div className="flex h-full min-h-0 flex-col">
          <SheetHeader className="flex-row flex-wrap items-center justify-between gap-4 border-b border-border px-6 py-4 pr-14 text-left">
            <div>
              <SheetTitle className="text-2xl">Demos</SheetTitle>
              <SheetDescription>
                {demos.length} demo{demos.length === 1 ? '' : 's'} · {enabledCount} included in stats
                {warningCount ? ` · ${warningCount} quality warning${warningCount === 1 ? '' : 's'}` : ''}
              </SheetDescription>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" disabled={!demos.length || enabledCount === demos.length || pending !== null} onClick={() => void toggleAll(true)}>
                Select all
              </Button>
              <Button variant="outline" size="sm" disabled={!demos.length || enabledCount === 0 || pending !== null} onClick={() => void toggleAll(false)}>
                Deselect all
              </Button>
              <input ref={fileInput} type="file" accept=".json,application/json" hidden onChange={(event) => { const file = event.target.files?.[0]; if (file) void importFile(file); event.target.value = ''; }} />
              <Button variant="outline" size="sm" onClick={() => fileInput.current?.click()}>
                <FileUp className="size-4" /> Import
              </Button>
              <Button variant="outline" size="sm" onClick={() => { window.location.href = '/api/export'; }}>
                <FileDown className="size-4" /> Export
              </Button>
              <Button variant="outline" size="sm" className="text-destructive hover:text-destructive" disabled={pending !== null} onClick={() => void clearUploadStorage()}>
                <Trash2 className="size-4" /> Clear uploads
              </Button>
            </div>
          </SheetHeader>

          {message ? <p className={cn('border-b border-border px-6 py-2 text-sm', failed ? 'text-destructive' : 'text-muted-foreground')}>{message}</p> : null}

          {demos.length ? (
            <div className="cheat-sheet-scroll min-h-0 flex-1 overflow-auto">
              <Table className="min-w-[920px] [&_td:first-child]:pl-6 [&_th:first-child]:pl-6">
                  <TableHeader>
                    <TableRow className="hover:bg-transparent">
                      {['In stats', 'Quality', 'File', 'Map', 'Played', 'Source', 'Origin', 'Players', 'Rounds', 'Added', ''].map((label, index) => (
                        <TableHead key={label || index} className="sticky top-0 z-20 bg-background">{label}</TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {dayGroups.map(([key, group]) => {
                      const dayDemos = demos.filter((demo) => dayKey(demo) === key);
                      const enabledInDay = dayDemos.filter((demo) => demo.enabled).length;
                      const allEnabled = enabledInDay === dayDemos.length;
                      return (
                        <Fragment key={key}>
                          <TableRow className="bg-muted/40 hover:bg-muted/40">
                            <TableCell>
                              <input
                                type="checkbox"
                                className="size-4 accent-primary"
                                checked={allEnabled}
                                ref={(el) => { if (el) el.indeterminate = enabledInDay > 0 && !allEnabled; }}
                                disabled={pending !== null}
                                title={allEnabled ? 'Exclude the whole day' : 'Include the whole day'}
                                onChange={() => void toggleDay(key, !allEnabled)}
                              />
                            </TableCell>
                            <TableCell />
                            <TableCell colSpan={9} className="text-xs font-medium text-muted-foreground">
                              {key === 'unknown' ? 'Unknown date' : new Date(key).toLocaleDateString(undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' })}
                              {' · '}{enabledInDay}/{dayDemos.length} in stats
                            </TableCell>
                          </TableRow>
                          {group.map((demo) => (
                            <TableRow key={demo.checksum} className={cn(!demo.enabled && 'opacity-50')}>
                              <TableCell>
                                <input
                                  type="checkbox"
                                  className="size-4 accent-primary"
                                  checked={demo.enabled}
                                  disabled={pending !== null}
                                  title={demo.qualityStatus === 'warning' && !demo.enabled ? 'Automatically excluded after a demo quality warning. Check to include manually.' : undefined}
                                  onChange={() => void toggle(demo)}
                                />
                              </TableCell>
                              <TableCell>
                                {demo.qualityStatus === 'warning' ? (
                                  <Badge
                                    variant="warning"
                                    className="gap-1 whitespace-nowrap"
                                    title={QUALITY_REASON_LABEL[demo.qualityReason] ?? demo.qualityReason}
                                  >
                                    <ShieldAlert className="size-3" /> Warning
                                  </Badge>
                                ) : demo.qualityStatus === 'not_checked' ? (
                                  <Badge title="Imported with an older analysis version; re-analyze to run the quality check.">Not checked</Badge>
                                ) : (
                                  <span className="text-xs text-muted-foreground">OK</span>
                                )}
                              </TableCell>
                              <TableCell className="font-medium">{demo.fileName}</TableCell>
                              <TableCell>{demo.mapName}</TableCell>
                              <TableCell className="tabular-nums">{demo.date ? new Date(demo.date).toLocaleDateString() : '—'}</TableCell>
                              <TableCell>{demo.source}</TableCell>
                              <TableCell>
                                {demo.origin === 'imported' ? (
                                  <Badge variant="outline" title="Merged from a stats export; the .dem file was never on this server.">Imported</Badge>
                                ) : (
                                  <span className="text-xs text-muted-foreground">Analyzed</span>
                                )}
                              </TableCell>
                              <TableCell className="tabular-nums">
                                {demo.playerNames?.length ? (
                                  <Tooltip>
                                    <TooltipTrigger asChild>
                                      <span className="cursor-default underline decoration-dotted underline-offset-2">{demo.players}</span>
                                    </TooltipTrigger>
                                    <TooltipContent>
                                      <ul className="space-y-0.5 text-xs">
                                        {demo.playerNames.map((name, index) => (
                                          <li key={`${name}-${index}`}>{name}</li>
                                        ))}
                                      </ul>
                                    </TooltipContent>
                                  </Tooltip>
                                ) : (
                                  demo.players
                                )}
                              </TableCell>
                              <TableCell className="tabular-nums">{demo.rounds}</TableCell>
                              <TableCell className="tabular-nums">{demo.importedAt ? new Date(demo.importedAt).toLocaleDateString() : '—'}</TableCell>
                              <TableCell className="w-[1%] px-2">
                                <button
                                  className="text-muted-foreground transition-colors hover:text-destructive disabled:opacity-50"
                                  title="Delete demo and all of its stats"
                                  disabled={pending !== null}
                                  onClick={() => void remove(demo)}
                                >
                                  <Trash2 className="size-4" />
                                </button>
                              </TableCell>
                            </TableRow>
                          ))}
                        </Fragment>
                      );
                    })}
                  </TableBody>
              </Table>
            </div>
          ) : (
            <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">No demos analyzed yet.</div>
          )}
        </div>
        </TooltipProvider>
      </SheetContent>
    </Sheet>
  );
}
