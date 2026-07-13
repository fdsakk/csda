export type Rule = { name: string; value: number; sample: number; points: number };

export type Player = {
  steamId: string;
  name: string;
  names: string[];
  demoCount: number;
  rounds: number;
  shots: number;
  hitShots: number;
  accuracy: number;
  damageEvents: number;
  headHitEvents: number;
  headHitRate: number;
  kills: number;
  headshotKills: number;
  headshotKillRate: number;
  smokeKills: number;
  wallKills: number;
  unspottedDamageRate: number;
  firstBulletHeadRate: number;
  snapRate: number;
  ttdSamples: number;
  ttdMedianMs: number;
  ttdWeightedMs: number;
  ttdP10Ms: number;
  reactionSamples: number;
  reactionMedianMs: number;
  reactionWeightedMs: number;
  reactionP10Ms: number;
  crosshairMedianAngle: number;
  firstShotMedianAngle: number;
  eligible: boolean;
  suspicionScore: number;
  status: 'normal' | 'watch' | 'review' | 'critical' | 'insufficient_sample';
  triggeredRules: Rule[] | null;
};

export type Demo = {
  checksum: string;
  path: string;
  fileName: string;
  mapName: string;
  date: string;
  tickRate: number;
  buildNumber: number;
  source: string;
  analysisVersion: number;
  importedAt: string;
  enabled: boolean;
  players: number;
  rounds: number;
};

export type Report = {
  players: Player[] | null;
  playersByWeapon: unknown[] | null;
  evidence: unknown[] | null;
  importedDemos: Demo[] | null;
};

export type Job = {
  id: string;
  status: 'queued' | 'running' | 'completed' | 'failed';
  files: string[];
  createdAt: string;
  startedAt?: string;
  endedAt?: string;
  result?: { imported: number; skipped: number; failed: number };
  error?: string;
  processed: number;
  total: number;
};

async function readJSON<T>(response: Response): Promise<T> {
  const body = await response.json();
  if (!response.ok) throw new Error(body.error ?? `Request failed: ${response.status}`);
  return body as T;
}

export async function getReport(): Promise<Report> {
  return readJSON<Report>(await fetch('/api/report'));
}

export async function getJobs(): Promise<Job[]> {
  return readJSON<Job[]>(await fetch('/api/jobs'));
}

export async function setDemoEnabled(checksum: string, enabled: boolean): Promise<void> {
  const response = await fetch(`/api/demos/${encodeURIComponent(checksum)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ enabled }),
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: undefined }));
    throw new Error(body.error ?? `Request failed: ${response.status}`);
  }
}

export async function importStats(file: File): Promise<{ imported: number; skipped: number }> {
  return readJSON(await fetch('/api/import', { method: 'POST', body: file }));
}

export function uploadDemos(files: File[], source: string, onProgress: (progress: number) => void): Promise<Job> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    files.forEach((file) => form.append('demos', file));
    const request = new XMLHttpRequest();
    request.open('POST', `/api/uploads?source=${encodeURIComponent(source)}`);
    request.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    });
    request.addEventListener('load', () => {
      try {
        const body = JSON.parse(request.responseText);
        if (request.status < 200 || request.status >= 300) throw new Error(body.error ?? 'Upload failed');
        resolve(body as Job);
      } catch (error) { reject(error); }
    });
    request.addEventListener('error', () => reject(new Error('Network error during upload')));
    request.send(form);
  });
}
