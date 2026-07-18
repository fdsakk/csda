import { Player } from '@/api';

export type StatusFilter = 'all' | 'flagged' | Player['status'];

const FLAGGED = ['watch', 'cheater'];

export type TableFilters = {
  status: StatusFilter;
  savedOnly: boolean;
  bannedOnly: boolean;
  hideBanned: boolean;
  minDemos: number;
  minShots: number;
  minAccuracy: number;
  minHeadHit: number;
  minHsKill: number;
  maxTtdMs: number;
  maxReactionMs: number;
  minKd: number;
};

export const DEFAULT_FILTERS: TableFilters = {
  status: 'all',
  savedOnly: false,
  bannedOnly: false,
  hideBanned: false,
  minDemos: 0,
  minShots: 0,
  minAccuracy: 0,
  minHeadHit: 0,
  minHsKill: 0,
  maxTtdMs: 0,
  maxReactionMs: 0,
  minKd: 0,
};

export function countActiveFilters(filters: TableFilters) {
  return (Object.keys(DEFAULT_FILTERS) as (keyof TableFilters)[]).filter((key) => filters[key] !== DEFAULT_FILTERS[key]).length;
}

function playerKd(player: Player) {
  return player.deaths ? player.kills / player.deaths : player.kills;
}

export function matchesFilters(player: Player, filters: TableFilters) {
  if (filters.status !== 'all' && !(filters.status === 'flagged' ? FLAGGED.includes(player.status) : player.status === filters.status)) return false;
  if (filters.savedOnly && !player.saved) return false;
  if (filters.bannedOnly && !player.banned) return false;
  if (filters.hideBanned && player.banned) return false;
  if (player.demoCount < filters.minDemos) return false;
  if (player.shots < filters.minShots) return false;
  if (player.accuracy < filters.minAccuracy) return false;
  if (player.headHitRate < filters.minHeadHit) return false;
  if (player.headshotKillRate < filters.minHsKill) return false;
  if (filters.maxTtdMs && !(player.nonAwpTtdSamples && player.nonAwpTtdWeightedMs <= filters.maxTtdMs)) return false;
  if (filters.maxReactionMs && !(player.nonAwpReactionSamples && player.nonAwpReactionWeightedMs <= filters.maxReactionMs)) return false;
  if (filters.minKd && playerKd(player) < filters.minKd) return false;
  return true;
}
