export type Rule = { name: string; value: number; sample: number; tier: 'watch' | 'cheater' | 'info' };

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
  deaths: number;
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
  awpKills: number;
  awpKillRate: number;
  isAwper: boolean;
  awpTtdSamples: number;
  awpTtdMedianMs: number;
  awpTtdWeightedMs: number;
  nonAwpTtdSamples: number;
  nonAwpTtdMedianMs: number;
  nonAwpTtdWeightedMs: number;
  nonAwpReactionSamples: number;
  nonAwpReactionWeightedMs: number;
  /** 20 bins of 50ms across 0–1000ms */
  ttdHistogram: number[] | null;
  reactionHistogram: number[] | null;
  movingShots: number;
  movingHitRate: number;
  airborneShots: number;
  airborneHitRate: number;
  flashedShots: number;
  flashedHitRate: number;
  scopedShots: number;
  scopedHitRate: number;
  reactionSamples: number;
  reactionMedianMs: number;
  reactionWeightedMs: number;
  reactionP10Ms: number;
  crosshairMedianAngle: number;
  firstShotMedianAngle: number;
  saved: boolean;
  banned: boolean;
  eligible: boolean;
  status: 'normal' | 'watch' | 'cheater' | 'insufficient_sample';
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
  qualityStatus: 'ok' | 'warning' | 'not_checked';
  qualityReason: 'systemic_low_timing' | string;
  players: number;
  rounds: number;
};

export type PlayerWeapon = {
  steamId: string;
  name: string;
  weaponName: string;
  shots: number;
  hitShots: number;
  accuracy: number;
  damageEvents: number;
  headHitEvents: number;
  headHitRate: number;
  kills: number;
};

export type Report = {
  players: Player[] | null;
  playersByWeapon: PlayerWeapon[] | null;
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
  /** overall job progress in percent, including partial demo parse progress */
  progress: number;
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

export async function deleteDemo(checksum: string): Promise<void> {
  const response = await fetch(`/api/demos/${encodeURIComponent(checksum)}`, { method: 'DELETE' });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: undefined }));
    throw new Error(body.error ?? `Request failed: ${response.status}`);
  }
}

export async function setPlayerSaved(steamId: string, saved: boolean): Promise<void> {
  return patchPlayer(steamId, { saved });
}

export async function setPlayerBanned(steamId: string, banned: boolean): Promise<void> {
  return patchPlayer(steamId, { banned });
}

async function patchPlayer(steamId: string, body: { saved?: boolean; banned?: boolean }): Promise<void> {
  const response = await fetch(`/api/players/${encodeURIComponent(steamId)}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: undefined }));
    throw new Error(payload.error ?? `Request failed: ${response.status}`);
  }
}

export async function importStats(file: File): Promise<{ imported: number; skipped: number }> {
  return readJSON(await fetch('/api/import', { method: 'POST', body: file }));
}

export function uploadDemos(files: File[], onProgress: (progress: number) => void): Promise<Job> {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    files.forEach((file) => form.append('demos', file));
    const request = new XMLHttpRequest();
    request.open('POST', '/api/uploads');
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
