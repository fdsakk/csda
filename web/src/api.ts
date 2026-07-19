export type Rule = { name: string; value: number; sample: number; tier: 'watch' | 'cheater'; score: number };

export type SuspicionConfig = {
  flagMode: 'score' | 'manual';
  minimumDemos: number;
  minimumShots: number;
  ttdMinimumSamples: number;
  ttdCheaterMs: number;
  ttdSuspiciousMs: number;
  reactionCheaterMs: number;
  reactionWatchMs: number;
  awpTtdCheaterMs: number;
  awpTtdWatchMs: number;
  eliteKd: number;
  eliteHeadHitRate: number;
  eliteAccuracy: number;
  eliteKdCheater: number;
  accuracyCheater: number;
  headHitMinimumEvents: number;
  headHitWatchThreshold: number;
  headHitCheaterThreshold: number;
  scoreWatchThreshold: number;
  scoreCheaterThreshold: number;
  metricWatchEvidence: number;
  metricCheaterEvidence: number;
  timingWeight: number;
  awpTimingWeight: number;
  awpEvidenceExponent: number;
  precisionWeight: number;
  performanceWeight: number;
  synergyWeight: number;
  sampleConfidenceFloor: number;
  sampleConfidenceK: number;
  scoreCurveExponent: number;
};

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
  suspicionScore: number;
  timingScore: number;
  precisionScore: number;
  performanceScore: number;
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
  /** 'analyzed' = parsed from a .dem here, 'imported' = merged from a stats export */
  origin: 'analyzed' | 'imported';
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
  importedDemos: Demo[] | null;
};

export type Job = {
  id: string;
  status: 'queued' | 'running' | 'completed' | 'failed';
  files: string[];
  createdAt: string;
  startedAt?: string;
  endedAt?: string;
  result?: { imported: number; skipped: number; failed: number; errors?: { path: string; error: string }[] };
  error?: string;
  processed: number;
  total: number;
  /** overall job progress in percent, including partial demo parse progress */
  progress: number;
};

// Shared fetch wrapper: throws the server's {error} message on failure,
// parses JSON on success (204 responses return undefined).
async function request<T = void>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, init);
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: undefined }));
    throw new Error(body.error ?? `Request failed: ${response.status}`);
  }
  return response.status === 204 ? (undefined as T) : response.json();
}

function patchJSON(body: unknown): RequestInit {
  return { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
}

export function getReport(): Promise<Report> {
  return request<Report>('/api/report');
}

export function getThresholds(): Promise<{ current: SuspicionConfig; defaults: SuspicionConfig }> {
  return request('/api/thresholds');
}

export function setThresholds(config: SuspicionConfig): Promise<SuspicionConfig> {
  return request('/api/thresholds', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(config) });
}

export function getJobs(): Promise<Job[]> {
  return request<Job[]>('/api/jobs');
}

export function deleteJob(id: string): Promise<void> {
  return request(`/api/jobs/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export function setDemoEnabled(checksum: string, enabled: boolean): Promise<void> {
  return request(`/api/demos/${encodeURIComponent(checksum)}`, patchJSON({ enabled }));
}

export function setAllDemosEnabled(enabled: boolean): Promise<void> {
  return request('/api/demos', patchJSON({ enabled }));
}

export function clearUploads(): Promise<void> {
  return request('/api/uploads', { method: 'DELETE' });
}

export function deleteDemo(checksum: string): Promise<void> {
  return request(`/api/demos/${encodeURIComponent(checksum)}`, { method: 'DELETE' });
}

export function setPlayerSaved(steamId: string, saved: boolean): Promise<void> {
  return request(`/api/players/${encodeURIComponent(steamId)}`, patchJSON({ saved }));
}

export function setPlayerBanned(steamId: string, banned: boolean): Promise<void> {
  return request(`/api/players/${encodeURIComponent(steamId)}`, patchJSON({ banned }));
}

export function importStats(file: File): Promise<{ imported: number; skipped: number }> {
  return request('/api/import', { method: 'POST', body: file });
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
