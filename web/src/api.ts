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

export type Report = {
  players: Player[] | null;
  playersByWeapon: unknown[] | null;
  evidence: unknown[] | null;
  importedDemos: unknown[] | null;
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
