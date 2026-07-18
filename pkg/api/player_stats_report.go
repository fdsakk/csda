package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
)

// PlayerSuspicionRule is a single informational signal shown in the expanded
// row (e.g. lots of smoke kills). Signals no longer carry points — the status
// tier comes from the TTD heuristic in flagPlayer, not from summing these.
// Tier is "watch" or "cheater" when the signal itself decides the status,
// or "info" for context-only signals (smoke/wall kills, unspotted damage).
type PlayerSuspicionRule struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Sample int     `json:"sample"`
	Tier   string  `json:"tier"`
}

type PlayerStatsReportRow struct {
	SteamID64                uint64   `json:"steamId"`
	Name                     string   `json:"name"`
	Names                    []string `json:"names"`
	DemoCount                int      `json:"demoCount"`
	Rounds                   int      `json:"rounds"`
	Shots                    int      `json:"shots"`
	HitShots                 int      `json:"hitShots"`
	Accuracy                 float64  `json:"accuracy"`
	DamageEvents             int      `json:"damageEvents"`
	HeadHitEvents            int      `json:"headHitEvents"`
	HeadHitRate              float64  `json:"headHitRate"`
	Kills                    int      `json:"kills"`
	Deaths                   int      `json:"deaths"`
	HeadshotKills            int      `json:"headshotKills"`
	HeadshotKillRate         float64  `json:"headshotKillRate"`
	SmokeKills               int      `json:"smokeKills"`
	WallKills                int      `json:"wallKills"`
	UnspottedDamageEvents    int      `json:"unspottedDamageEvents"`
	UnspottedDamageRate      float64  `json:"unspottedDamageRate"`
	FirstBulletEncounters    int      `json:"firstBulletEncounters"`
	FirstBulletHeadHits      int      `json:"firstBulletHeadHits"`
	FirstBulletHeadRate      float64  `json:"firstBulletHeadRate"`
	SnapEvents               int      `json:"snapEvents"`
	SnapRate                 float64  `json:"snapRate"`
	TTDSamples               int      `json:"ttdSamples"`
	TTDMeanMS                float64  `json:"ttdMeanMs"`
	TTDMedianMS              float64  `json:"ttdMedianMs"`
	TTDWeightedMS            float64  `json:"ttdWeightedMs"`
	TTDP10MS                 float64  `json:"ttdP10Ms"`
	TTDUnder190Rate          float64  `json:"ttdUnder190Rate"`
	AWPKills                 int      `json:"awpKills"`
	AWPKillRate              float64  `json:"awpKillRate"`
	IsAWPer                  bool     `json:"isAwper"`
	AWPTTDSamples            int      `json:"awpTtdSamples"`
	AWPTTDMedianMS           float64  `json:"awpTtdMedianMs"`
	AWPTTDWeightedMS         float64  `json:"awpTtdWeightedMs"`
	NonAWPTTDSamples         int      `json:"nonAwpTtdSamples"`
	NonAWPTTDMedianMS        float64  `json:"nonAwpTtdMedianMs"`
	NonAWPTTDWeightedMS      float64  `json:"nonAwpTtdWeightedMs"`
	NonAWPReactionSamples    int      `json:"nonAwpReactionSamples"`
	NonAWPReactionWeightedMS float64  `json:"nonAwpReactionWeightedMs"`
	// 20 bins of 50ms across 0–1000ms, for the UI distribution charts.
	TTDHistogram         []int                 `json:"ttdHistogram"`
	ReactionHistogram    []int                 `json:"reactionHistogram"`
	ReactionSamples      int                   `json:"reactionSamples"`
	ReactionMedianMS     float64               `json:"reactionMedianMs"`
	ReactionWeightedMS   float64               `json:"reactionWeightedMs"`
	ReactionP10MS        float64               `json:"reactionP10Ms"`
	CrosshairMedianAngle float64               `json:"crosshairMedianAngle"`
	FirstShotMedianAngle float64               `json:"firstShotMedianAngle"`
	MovingShots          int                   `json:"movingShots"`
	MovingHitRate        float64               `json:"movingHitRate"`
	AirborneShots        int                   `json:"airborneShots"`
	AirborneHitRate      float64               `json:"airborneHitRate"`
	FlashedShots         int                   `json:"flashedShots"`
	FlashedHitRate       float64               `json:"flashedHitRate"`
	ScopedShots          int                   `json:"scopedShots"`
	ScopedHitRate        float64               `json:"scopedHitRate"`
	Saved                bool                  `json:"saved"`
	Banned               bool                  `json:"banned"`
	Eligible             bool                  `json:"eligible"`
	Status               string                `json:"status"`
	TriggeredRules       []PlayerSuspicionRule `json:"triggeredRules"`
}

func (row PlayerStatsReportRow) MarshalJSON() ([]byte, error) {
	type alias PlayerStatsReportRow
	return json.Marshal(struct {
		*alias
		SteamID string `json:"steamId"`
	}{alias: (*alias)(&row), SteamID: strconv.FormatUint(row.SteamID64, 10)})
}

type PlayerWeaponReportRow struct {
	SteamID64     uint64  `json:"steamId"`
	Name          string  `json:"name"`
	WeaponName    string  `json:"weaponName"`
	Shots         int     `json:"shots"`
	HitShots      int     `json:"hitShots"`
	Accuracy      float64 `json:"accuracy"`
	DamageEvents  int     `json:"damageEvents"`
	HeadHitEvents int     `json:"headHitEvents"`
	HeadHitRate   float64 `json:"headHitRate"`
	Kills         int     `json:"kills"`
}

func (row PlayerWeaponReportRow) MarshalJSON() ([]byte, error) {
	type alias PlayerWeaponReportRow
	return json.Marshal(struct {
		*alias
		SteamID string `json:"steamId"`
	}{alias: (*alias)(&row), SteamID: strconv.FormatUint(row.SteamID64, 10)})
}

type PlayerEvidenceReportRow struct {
	DemoChecksum string  `json:"demoChecksum"`
	DemoPath     string  `json:"demoPath"`
	RoundNumber  int     `json:"roundNumber"`
	Tick         int     `json:"tick"`
	SteamID64    uint64  `json:"steamId"`
	PlayerName   string  `json:"playerName"`
	VictimID     uint64  `json:"victimSteamId"`
	Kind         string  `json:"kind"`
	Value        float64 `json:"value"`
	Details      string  `json:"details"`
}

func (row PlayerEvidenceReportRow) MarshalJSON() ([]byte, error) {
	type alias PlayerEvidenceReportRow
	return json.Marshal(struct {
		*alias
		SteamID  string `json:"steamId"`
		VictimID string `json:"victimSteamId"`
	}{alias: (*alias)(&row), SteamID: strconv.FormatUint(row.SteamID64, 10), VictimID: strconv.FormatUint(row.VictimID, 10)})
}

type ImportedDemoReportRow struct {
	Checksum        string  `json:"checksum"`
	Path            string  `json:"path"`
	FileName        string  `json:"fileName"`
	MapName         string  `json:"mapName"`
	Date            string  `json:"date"`
	TickRate        float64 `json:"tickRate"`
	BuildNumber     int     `json:"buildNumber"`
	Source          string  `json:"source"`
	AnalysisVersion int     `json:"analysisVersion"`
	ImportedAt      string  `json:"importedAt"`
	Enabled         bool    `json:"enabled"`
	QualityStatus   string  `json:"qualityStatus"`
	QualityReason   string  `json:"qualityReason"`
	// Origin is "analyzed" for demos parsed from a .dem file on this instance
	// and "imported" for demos merged in from a stats export.
	Origin  string `json:"origin"`
	Players int    `json:"players"`
	Rounds  int    `json:"rounds"`
}

type PlayerStatsReport struct {
	Players  []PlayerStatsReportRow    `json:"players"`
	Weapons  []PlayerWeaponReportRow   `json:"playersByWeapon"`
	Evidence []PlayerEvidenceReportRow `json:"evidence"`
	Demos    []ImportedDemoReportRow   `json:"importedDemos"`
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// histogramMS buckets millisecond samples into 20 bins of 50ms (0–1000ms).
// Samples are already clamped to that range by the caller.
func histogramMS(values []float64) []int {
	bins := make([]int, 20)
	for _, value := range values {
		bins[min(int(value/50), 19)]++
	}
	return bins
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	values = append([]float64(nil), values...)
	sort.Float64s(values)
	if len(values) == 1 {
		return values[0]
	}
	position := p * float64(len(values)-1)
	lo := int(position)
	hi := lo + 1
	if hi >= len(values) {
		return values[lo]
	}
	weight := position - float64(lo)
	return values[lo]*(1-weight) + values[hi]*weight
}

type demoSamples struct {
	rounds int
	values []float64
}

func appendDemoSample(groups map[int64]*demoSamples, demoID int64, rounds int, value float64) {
	group := groups[demoID]
	if group == nil {
		group = &demoSamples{rounds: rounds}
		groups[demoID] = group
	}
	group.values = append(group.values, value)
}

func roundWeightedDemoMedian(groups map[int64]*demoSamples) float64 {
	weighted, rounds := 0.0, 0
	for _, group := range groups {
		if len(group.values) == 0 || group.rounds <= 0 {
			continue
		}
		weighted += percentile(group.values, .5) * float64(group.rounds)
		rounds += group.rounds
	}
	if rounds == 0 {
		return 0
	}
	return weighted / float64(rounds)
}

// playerEncounterSamples accumulates one player's encounter timings so the
// report can compute all encounter-derived stats from a single query pass.
type playerEncounterSamples struct {
	ttd, reaction                    []float64
	awpTTD, nonAWPTTD                []float64
	nonAWPReaction                   []float64
	crosshairAngles, firstShotAngles []float64
	ttdByDemo, reactionByDemo        map[int64]*demoSamples
	awpTTDByDemo, nonAWPTTDByDemo    map[int64]*demoSamples
	nonAWPReactionByDemo             map[int64]*demoSamples
	under190                         int
}

func newPlayerEncounterSamples() *playerEncounterSamples {
	return &playerEncounterSamples{
		ttdByDemo:            make(map[int64]*demoSamples),
		reactionByDemo:       make(map[int64]*demoSamples),
		awpTTDByDemo:         make(map[int64]*demoSamples),
		nonAWPTTDByDemo:      make(map[int64]*demoSamples),
		nonAWPReactionByDemo: make(map[int64]*demoSamples),
	}
}

func (s *playerEncounterSamples) add(demoID int64, rounds int, ttd, reaction, crosshairAngle, firstShotAngle float64, isAWP bool) {
	if ttd >= 0 && ttd <= 1000 {
		s.ttd = append(s.ttd, ttd)
		appendDemoSample(s.ttdByDemo, demoID, rounds, ttd)
		s.crosshairAngles = append(s.crosshairAngles, crosshairAngle)
		if firstShotAngle > 0 {
			s.firstShotAngles = append(s.firstShotAngles, firstShotAngle)
		}
		if ttd <= 190 {
			s.under190++
		}
		if isAWP {
			s.awpTTD = append(s.awpTTD, ttd)
			appendDemoSample(s.awpTTDByDemo, demoID, rounds, ttd)
		} else {
			s.nonAWPTTD = append(s.nonAWPTTD, ttd)
			appendDemoSample(s.nonAWPTTDByDemo, demoID, rounds, ttd)
		}
	}
	// reaction_time_ms is -1 on rows stored before the column existed
	if reaction >= 0 && reaction <= 1000 {
		s.reaction = append(s.reaction, reaction)
		appendDemoSample(s.reactionByDemo, demoID, rounds, reaction)
		if !isAWP {
			s.nonAWPReaction = append(s.nonAWPReaction, reaction)
			appendDemoSample(s.nonAWPReactionByDemo, demoID, rounds, reaction)
		}
	}
}

func (s *playerEncounterSamples) apply(row *PlayerStatsReportRow) {
	row.TTDHistogram = histogramMS(s.ttd)
	row.ReactionHistogram = histogramMS(s.reaction)
	row.TTDSamples = len(s.ttd)
	row.TTDMedianMS = percentile(s.ttd, .5)
	row.TTDWeightedMS = roundWeightedDemoMedian(s.ttdByDemo)
	row.TTDP10MS = percentile(s.ttd, .1)
	row.TTDUnder190Rate = ratio(s.under190, len(s.ttd))
	row.AWPTTDSamples = len(s.awpTTD)
	row.AWPTTDMedianMS = percentile(s.awpTTD, .5)
	row.AWPTTDWeightedMS = roundWeightedDemoMedian(s.awpTTDByDemo)
	if row.AWPTTDWeightedMS == 0 {
		row.AWPTTDWeightedMS = row.AWPTTDMedianMS
	}
	row.NonAWPTTDSamples = len(s.nonAWPTTD)
	row.NonAWPTTDMedianMS = percentile(s.nonAWPTTD, .5)
	row.NonAWPTTDWeightedMS = roundWeightedDemoMedian(s.nonAWPTTDByDemo)
	if row.NonAWPTTDWeightedMS == 0 {
		row.NonAWPTTDWeightedMS = row.NonAWPTTDMedianMS
	}
	row.NonAWPReactionSamples = len(s.nonAWPReaction)
	row.NonAWPReactionWeightedMS = roundWeightedDemoMedian(s.nonAWPReactionByDemo)
	if row.NonAWPReactionWeightedMS == 0 {
		row.NonAWPReactionWeightedMS = percentile(s.nonAWPReaction, .5)
	}
	for _, value := range s.ttd {
		row.TTDMeanMS += value
	}
	if row.TTDSamples > 0 {
		row.TTDMeanMS /= float64(row.TTDSamples)
	}
	row.CrosshairMedianAngle = percentile(s.crosshairAngles, .5)
	row.FirstShotMedianAngle = percentile(s.firstShotAngles, .5)

	row.ReactionSamples = len(s.reaction)
	row.ReactionMedianMS = percentile(s.reaction, .5)
	row.ReactionWeightedMS = roundWeightedDemoMedian(s.reactionByDemo)
	row.ReactionP10MS = percentile(s.reaction, .1)
	if row.TTDWeightedMS == 0 {
		row.TTDWeightedMS = row.TTDMedianMS
	}
	if row.ReactionWeightedMS == 0 {
		row.ReactionWeightedMS = row.ReactionMedianMS
	}
}

// collectEncounterSamples reads every enabled demo's encounters once and
// groups them per attacker, instead of one query per player.
func collectEncounterSamples(ctx context.Context, db *sql.DB) (map[uint64]*playerEncounterSamples, error) {
	rows, err := db.QueryContext(ctx, `SELECT e.attacker_steam_id,e.demo_id,s.rounds,e.ttd_ms,e.reaction_time_ms,e.confirmed_angle,e.first_shot_angle,e.weapon_name FROM encounters e JOIN player_demo_stats s ON s.demo_id=e.demo_id AND s.steam_id=e.attacker_steam_id JOIN demos d ON d.id=e.demo_id AND d.enabled=1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	awpName := constants.WeaponAWP.String()
	byPlayer := make(map[uint64]*playerEncounterSamples)
	for rows.Next() {
		var steamID uint64
		var demoID int64
		var rounds int
		var ttd, reaction, crosshairAngle, firstShotAngle float64
		var weaponName string
		if err := rows.Scan(&steamID, &demoID, &rounds, &ttd, &reaction, &crosshairAngle, &firstShotAngle, &weaponName); err != nil {
			return nil, err
		}
		samples := byPlayer[steamID]
		if samples == nil {
			samples = newPlayerEncounterSamples()
			byPlayer[steamID] = samples
		}
		samples.add(demoID, rounds, ttd, reaction, crosshairAngle, firstShotAngle, weaponName == awpName)
	}
	return byPlayer, rows.Err()
}

// promote keeps the worst tier seen so far. cheater > watch > normal.
func promote(current, next string) string {
	rank := map[string]int{"normal": 0, "watch": 1, "cheater": 2}
	if rank[next] > rank[current] {
		return next
	}
	return current
}

// flagPlayer assigns a two-tier status (watch = yellow, cheater = red) from a
// small set of hard heuristics, and records the signals it saw for the UI.
//
// Rifle/pistol (non-AWP) time-to-damage, read as a long-term average:
//
//	360+ ms   normal
//	320-360   suspicious | watch, or cheater if the player is also fragging
//	                       hard (K/D, head-accuracy or accuracy elite)
//	< 320     not humanly reproducible over many games | cheater
//
// AWP TTD is excluded from the above (one flick + one-shot kill makes it
// naturally lower). It gets its own, lower tier for anyone with enough AWP
// encounters: < 210 ms cheater, < 280 ms watch. The two are independent — a
// player can look clean on the rifle and get flagged on the AWP, or vice versa.
//
// Non-AWP reaction time (see→first shot) below ~200 ms as an average is a
// trigger/aimbot signal. Head-hit-rate flags on its own at very high,
// well-sampled values. Smoke/wall kills and unspotted damage are kept as
// context-only signals — they colour nothing by themselves.
func flagPlayer(row *PlayerStatsReportRow, config SuspicionConfig) {
	row.Eligible = row.DemoCount >= config.MinimumDemos && row.Shots >= config.MinimumShots
	if !row.Eligible {
		row.Status = "insufficient_sample"
		return
	}
	row.Status = "normal"
	signal := func(name string, value float64, sample int, tier string) {
		row.Status = promote(row.Status, tier)
		row.TriggeredRules = append(row.TriggeredRules, PlayerSuspicionRule{Name: name, Value: value, Sample: sample, Tier: tier})
	}

	kd := ratio(row.Kills, row.Deaths)
	if row.Deaths == 0 && row.Kills > 0 {
		kd = float64(row.Kills)
	}
	eliteStats := kd >= config.EliteKD || row.HeadHitRate >= config.EliteHeadHitRate || row.Accuracy >= config.EliteAccuracy

	// Non-AWP time-to-damage: the primary signal.
	if row.NonAWPTTDSamples >= config.TTDMinimumSamples && row.NonAWPTTDWeightedMS > 0 {
		switch {
		case row.NonAWPTTDWeightedMS < config.TTDCheaterMS:
			signal("ttd_impossible", row.NonAWPTTDWeightedMS, row.NonAWPTTDSamples, "cheater")
		case row.NonAWPTTDWeightedMS < config.TTDSuspiciousMS:
			if eliteStats {
				signal("ttd_low_elite_stats", row.NonAWPTTDWeightedMS, row.NonAWPTTDSamples, "cheater")
			} else {
				signal("ttd_low", row.NonAWPTTDWeightedMS, row.NonAWPTTDSamples, "watch")
			}
		}
	}

	// AWP time-to-damage: its own tier, for anyone with enough AWP encounters —
	// a rifler who only triggers on the AWP still gets caught here.
	if row.AWPTTDSamples >= config.TTDMinimumSamples && row.AWPTTDWeightedMS > 0 {
		switch {
		case row.AWPTTDWeightedMS < config.AWPTTDCheaterMS:
			signal("awp_ttd_impossible", row.AWPTTDWeightedMS, row.AWPTTDSamples, "cheater")
		case row.AWPTTDWeightedMS < config.AWPTTDWatchMS:
			signal("awp_ttd_low", row.AWPTTDWeightedMS, row.AWPTTDSamples, "watch")
		}
	}

	// Non-AWP reaction time (see → first shot) below human floor as an average.
	if row.NonAWPReactionSamples >= config.TTDMinimumSamples && row.NonAWPReactionWeightedMS > 0 && row.NonAWPReactionWeightedMS < config.ReactionCheaterMS {
		signal("reaction_impossible", row.NonAWPReactionWeightedMS, row.NonAWPReactionSamples, "cheater")
	}

	// Head-hit-rate standalone.
	if row.DamageEvents >= config.HeadHitMinimumEvents {
		switch {
		case row.HeadHitRate >= config.HeadHitCheaterThreshold:
			signal("head_hit_rate", row.HeadHitRate, row.DamageEvents, "cheater")
		case row.HeadHitRate >= config.HeadHitWatchThreshold:
			signal("head_hit_rate", row.HeadHitRate, row.DamageEvents, "watch")
		}
	}

	// Context-only signals — surfaced in the expanded row, colour nothing.
	combinedSpecialKills := row.SmokeKills + row.WallKills
	if combinedSpecialKills > 0 {
		row.TriggeredRules = append(row.TriggeredRules, PlayerSuspicionRule{Name: "smoke_wall_kills", Value: float64(combinedSpecialKills), Sample: row.Kills, Tier: "info"})
	}
	if row.DamageEvents > 0 && row.UnspottedDamageRate > 0 {
		row.TriggeredRules = append(row.TriggeredRules, PlayerSuspicionRule{Name: "unspotted_damage", Value: row.UnspottedDamageRate, Sample: row.DamageEvents, Tier: "info"})
	}
}

func buildPlayerStatsReport(ctx context.Context, options PlayerStatsReportOptions) (*PlayerStatsReport, error) {
	db, err := openPlayerStatsDB(options.DatabasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	config := options.Config
	if config.MinimumDemos == 0 {
		config = DefaultSuspicionConfig()
	}
	report := &PlayerStatsReport{}

	rows, err := db.QueryContext(ctx, `SELECT p.steam_id,p.latest_name,p.names,p.saved,p.banned,COUNT(s.demo_id),COALESCE(SUM(s.rounds),0),COALESCE(SUM(s.shots),0),COALESCE(SUM(s.hit_shots),0),COALESCE(SUM(s.damage_events),0),COALESCE(SUM(s.head_hit_events),0),COALESCE(SUM(s.kills),0),COALESCE(SUM(s.deaths),0),COALESCE(SUM(s.headshot_kills),0),COALESCE(SUM(s.smoke_kills),0),COALESCE(SUM(s.wall_kills),0),COALESCE(SUM(s.unspotted_damage_events),0),COALESCE(SUM(s.first_bullet_encounters),0),COALESCE(SUM(s.first_bullet_head_hits),0),COALESCE(SUM(s.snap_events),0),COALESCE(SUM(s.ttd_samples),0),COALESCE(SUM(s.ttd_sum_ms),0),COALESCE(SUM(s.moving_shots),0),COALESCE(SUM(s.moving_hit_shots),0),COALESCE(SUM(s.airborne_shots),0),COALESCE(SUM(s.airborne_hit_shots),0),COALESCE(SUM(s.flashed_shots),0),COALESCE(SUM(s.flashed_hit_shots),0),COALESCE(SUM(s.scoped_shots),0),COALESCE(SUM(s.scoped_hit_shots),0) FROM players p JOIN player_demo_stats s ON s.steam_id=p.steam_id JOIN demos d ON d.id=s.demo_id AND d.enabled=1 GROUP BY p.steam_id,p.latest_name,p.names,p.saved,p.banned ORDER BY p.steam_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row PlayerStatsReportRow
		var names string
		var storedTTDSamples int
		var storedTTDSum float64
		var movingHits, airborneHits, flashedHits, scopedHits int
		if err := rows.Scan(&row.SteamID64, &row.Name, &names, &row.Saved, &row.Banned, &row.DemoCount, &row.Rounds, &row.Shots, &row.HitShots, &row.DamageEvents, &row.HeadHitEvents, &row.Kills, &row.Deaths, &row.HeadshotKills, &row.SmokeKills, &row.WallKills, &row.UnspottedDamageEvents, &row.FirstBulletEncounters, &row.FirstBulletHeadHits, &row.SnapEvents, &storedTTDSamples, &storedTTDSum, &row.MovingShots, &movingHits, &row.AirborneShots, &airborneHits, &row.FlashedShots, &flashedHits, &row.ScopedShots, &scopedHits); err != nil {
			rows.Close()
			return nil, err
		}
		row.Names = strings.Split(names, "\n")
		row.Accuracy = ratio(row.HitShots, row.Shots)
		row.HeadHitRate = ratio(row.HeadHitEvents, row.DamageEvents)
		row.HeadshotKillRate = ratio(row.HeadshotKills, row.Kills)
		row.UnspottedDamageRate = ratio(row.UnspottedDamageEvents, row.DamageEvents)
		row.FirstBulletHeadRate = ratio(row.FirstBulletHeadHits, row.FirstBulletEncounters)
		row.SnapRate = ratio(row.SnapEvents, row.FirstBulletEncounters)
		row.MovingHitRate = ratio(movingHits, row.MovingShots)
		row.AirborneHitRate = ratio(airborneHits, row.AirborneShots)
		row.FlashedHitRate = ratio(flashedHits, row.FlashedShots)
		row.ScopedHitRate = ratio(scopedHits, row.ScopedShots)
		report.Players = append(report.Players, row)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	samplesByPlayer, err := collectEncounterSamples(ctx, db)
	if err != nil {
		return nil, err
	}
	for index := range report.Players {
		row := &report.Players[index]
		samples := samplesByPlayer[row.SteamID64]
		if samples == nil {
			samples = &playerEncounterSamples{}
		}
		samples.apply(row)
	}

	weaponRows, err := db.QueryContext(ctx, `SELECT w.steam_id,p.latest_name,w.weapon_name,SUM(w.shots),SUM(w.hit_shots),SUM(w.damage_events),SUM(w.head_hit_events),SUM(w.kills) FROM player_demo_weapon_stats w JOIN players p ON p.steam_id=w.steam_id JOIN demos d ON d.id=w.demo_id AND d.enabled=1 GROUP BY w.steam_id,p.latest_name,w.weapon_name ORDER BY w.steam_id,w.weapon_name`)
	if err != nil {
		return nil, err
	}
	for weaponRows.Next() {
		var w PlayerWeaponReportRow
		if err := weaponRows.Scan(&w.SteamID64, &w.Name, &w.WeaponName, &w.Shots, &w.HitShots, &w.DamageEvents, &w.HeadHitEvents, &w.Kills); err != nil {
			weaponRows.Close()
			return nil, err
		}
		w.Accuracy = ratio(w.HitShots, w.Shots)
		w.HeadHitRate = ratio(w.HeadHitEvents, w.DamageEvents)
		report.Weapons = append(report.Weapons, w)
		if w.WeaponName == constants.WeaponAWP.String() {
			for index := range report.Players {
				if report.Players[index].SteamID64 == w.SteamID64 {
					report.Players[index].AWPKills = w.Kills
					break
				}
			}
		}
	}
	weaponRows.Close()
	for index := range report.Players {
		row := &report.Players[index]
		row.AWPKillRate = ratio(row.AWPKills, row.Kills)
		row.IsAWPer = row.AWPKills >= 5 && row.AWPKillRate >= .25
		flagPlayer(row, config)
	}

	if options.IncludeEvidence {
		evidenceRows, err := db.QueryContext(ctx, `SELECT d.checksum,d.path,e.round_number,e.tick,e.steam_id,p.latest_name,e.victim_steam_id,e.kind,e.value,e.details FROM evidence e JOIN demos d ON d.id=e.demo_id LEFT JOIN players p ON p.steam_id=e.steam_id WHERE d.enabled=1 ORDER BY e.steam_id,d.demo_date,e.round_number,e.tick`)
		if err != nil {
			return nil, err
		}
		for evidenceRows.Next() {
			var e PlayerEvidenceReportRow
			if err := evidenceRows.Scan(&e.DemoChecksum, &e.DemoPath, &e.RoundNumber, &e.Tick, &e.SteamID64, &e.PlayerName, &e.VictimID, &e.Kind, &e.Value, &e.Details); err != nil {
				evidenceRows.Close()
				return nil, err
			}
			report.Evidence = append(report.Evidence, e)
		}
		evidenceRows.Close()
	}

	demoRows, err := db.QueryContext(ctx, `SELECT d.checksum,d.path,d.file_name,d.map_name,d.demo_date,d.tick_rate,d.build_number,d.source,d.analysis_version,d.imported_at,d.enabled,d.quality_status,d.quality_reason,d.origin,COUNT(s.steam_id),COALESCE(MAX(s.rounds),0) FROM demos d LEFT JOIN player_demo_stats s ON s.demo_id=d.id GROUP BY d.id ORDER BY d.demo_date,d.checksum`)
	if err != nil {
		return nil, err
	}
	for demoRows.Next() {
		var d ImportedDemoReportRow
		if err := demoRows.Scan(&d.Checksum, &d.Path, &d.FileName, &d.MapName, &d.Date, &d.TickRate, &d.BuildNumber, &d.Source, &d.AnalysisVersion, &d.ImportedAt, &d.Enabled, &d.QualityStatus, &d.QualityReason, &d.Origin, &d.Players, &d.Rounds); err != nil {
			demoRows.Close()
			return nil, err
		}
		report.Demos = append(report.Demos, d)
	}
	demoRows.Close()
	return report, nil
}

// GetPlayerStatsReport reads the current aggregate report without writing files.
func GetPlayerStatsReport(ctx context.Context, options PlayerStatsReportOptions) (*PlayerStatsReport, error) {
	return buildPlayerStatsReport(ctx, options)
}

func ExportPlayerStatsReport(ctx context.Context, options PlayerStatsReportOptions) error {
	if options.OutputPath == "" {
		return errors.New("report output path is required")
	}
	if options.Format == "" {
		options.Format = constants.ExportFormatCSV
	}
	if options.Format != constants.ExportFormatCSV && options.Format != constants.ExportFormatJSON {
		return fmt.Errorf("player stats report supports csv and json formats")
	}
	if err := os.MkdirAll(options.OutputPath, 0o755); err != nil {
		return err
	}
	options.IncludeEvidence = true
	report, err := buildPlayerStatsReport(ctx, options)
	if err != nil {
		return err
	}
	if options.Format == constants.ExportFormatJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(options.OutputPath, "player_stats.json"), data, 0o644)
	}
	if err := writePlayersCSV(filepath.Join(options.OutputPath, "players.csv"), report.Players); err != nil {
		return err
	}
	if err := writeWeaponsCSV(filepath.Join(options.OutputPath, "players_by_weapon.csv"), report.Weapons); err != nil {
		return err
	}
	if err := writeEvidenceCSV(filepath.Join(options.OutputPath, "evidence.csv"), report.Evidence); err != nil {
		return err
	}
	return writeDemosCSV(filepath.Join(options.OutputPath, "imported_demos.csv"), report.Demos)
}

func f(value float64) string { return strconv.FormatFloat(value, 'f', 4, 64) }
func i(value int) string     { return strconv.Itoa(value) }
func u(value uint64) string  { return strconv.FormatUint(value, 10) }
func writeCSV(path string, lines [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := csv.NewWriter(file)
	err = w.WriteAll(lines)
	if err == nil {
		err = w.Error()
	}
	return err
}
func writePlayersCSV(path string, rows []PlayerStatsReportRow) error {
	lines := [][]string{{"steamid", "name", "aliases", "demos", "rounds", "shots", "hit shots", "accuracy", "damage events", "head hits", "head hit rate", "kills", "headshot kills", "headshot kill rate", "smoke kills", "wall kills", "unspotted damage rate", "first bullet head rate", "snap rate", "ttd samples", "ttd mean ms", "ttd pooled median ms", "ttd weighted ms", "ttd p10 ms", "ttd under 190 rate", "reaction samples", "reaction median ms", "reaction weighted ms", "reaction p10 ms", "crosshair median angle", "first shot median angle", "moving shots", "moving hit rate", "airborne shots", "airborne hit rate", "flashed shots", "flashed hit rate", "scoped shots", "scoped hit rate", "eligible", "status", "triggered rules"}}
	for _, r := range rows {
		rules := make([]string, len(r.TriggeredRules))
		for x, rule := range r.TriggeredRules {
			rules[x] = fmt.Sprintf("%s:%s(value=%.2f,sample=%d)", rule.Name, rule.Tier, rule.Value, rule.Sample)
		}
		lines = append(lines, []string{u(r.SteamID64), r.Name, strings.Join(r.Names, " | "), i(r.DemoCount), i(r.Rounds), i(r.Shots), i(r.HitShots), f(r.Accuracy), i(r.DamageEvents), i(r.HeadHitEvents), f(r.HeadHitRate), i(r.Kills), i(r.HeadshotKills), f(r.HeadshotKillRate), i(r.SmokeKills), i(r.WallKills), f(r.UnspottedDamageRate), f(r.FirstBulletHeadRate), f(r.SnapRate), i(r.TTDSamples), f(r.TTDMeanMS), f(r.TTDMedianMS), f(r.TTDWeightedMS), f(r.TTDP10MS), f(r.TTDUnder190Rate), i(r.ReactionSamples), f(r.ReactionMedianMS), f(r.ReactionWeightedMS), f(r.ReactionP10MS), f(r.CrosshairMedianAngle), f(r.FirstShotMedianAngle), i(r.MovingShots), f(r.MovingHitRate), i(r.AirborneShots), f(r.AirborneHitRate), i(r.FlashedShots), f(r.FlashedHitRate), i(r.ScopedShots), f(r.ScopedHitRate), strconv.FormatBool(r.Eligible), r.Status, strings.Join(rules, " | ")})
	}
	return writeCSV(path, lines)
}
func writeWeaponsCSV(path string, rows []PlayerWeaponReportRow) error {
	lines := [][]string{{"steamid", "name", "weapon", "shots", "hit shots", "accuracy", "damage events", "head hits", "head hit rate", "kills"}}
	for _, r := range rows {
		lines = append(lines, []string{u(r.SteamID64), r.Name, r.WeaponName, i(r.Shots), i(r.HitShots), f(r.Accuracy), i(r.DamageEvents), i(r.HeadHitEvents), f(r.HeadHitRate), i(r.Kills)})
	}
	return writeCSV(path, lines)
}
func writeEvidenceCSV(path string, rows []PlayerEvidenceReportRow) error {
	lines := [][]string{{"demo checksum", "demo path", "round", "tick", "steamid", "player", "victim steamid", "kind", "value", "details"}}
	for _, r := range rows {
		lines = append(lines, []string{r.DemoChecksum, r.DemoPath, i(r.RoundNumber), i(r.Tick), u(r.SteamID64), r.PlayerName, u(r.VictimID), r.Kind, f(r.Value), r.Details})
	}
	return writeCSV(path, lines)
}
func writeDemosCSV(path string, rows []ImportedDemoReportRow) error {
	lines := [][]string{{"checksum", "path", "file name", "map", "date", "tickrate", "build number", "source", "analysis version", "imported at", "enabled", "quality status", "quality reason", "origin", "players", "rounds"}}
	for _, r := range rows {
		lines = append(lines, []string{r.Checksum, r.Path, r.FileName, r.MapName, r.Date, f(r.TickRate), i(r.BuildNumber), r.Source, i(r.AnalysisVersion), r.ImportedAt, strconv.FormatBool(r.Enabled), r.QualityStatus, r.QualityReason, r.Origin, i(r.Players), i(r.Rounds)})
	}
	return writeCSV(path, lines)
}
