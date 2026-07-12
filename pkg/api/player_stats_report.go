package api

import (
	"context"
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

type PlayerSuspicionRule struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Sample int     `json:"sample"`
	Points int     `json:"points"`
}

type PlayerStatsReportRow struct {
	SteamID64             uint64                `json:"steamId"`
	Name                  string                `json:"name"`
	Names                 []string              `json:"names"`
	DemoCount             int                   `json:"demoCount"`
	Rounds                int                   `json:"rounds"`
	Shots                 int                   `json:"shots"`
	HitShots              int                   `json:"hitShots"`
	Accuracy              float64               `json:"accuracy"`
	DamageEvents          int                   `json:"damageEvents"`
	HeadHitEvents         int                   `json:"headHitEvents"`
	HeadHitRate           float64               `json:"headHitRate"`
	Kills                 int                   `json:"kills"`
	HeadshotKills         int                   `json:"headshotKills"`
	HeadshotKillRate      float64               `json:"headshotKillRate"`
	SmokeKills            int                   `json:"smokeKills"`
	WallKills             int                   `json:"wallKills"`
	UnspottedDamageEvents int                   `json:"unspottedDamageEvents"`
	UnspottedDamageRate   float64               `json:"unspottedDamageRate"`
	FirstBulletEncounters int                   `json:"firstBulletEncounters"`
	FirstBulletHeadHits   int                   `json:"firstBulletHeadHits"`
	FirstBulletHeadRate   float64               `json:"firstBulletHeadRate"`
	SnapEvents            int                   `json:"snapEvents"`
	SnapRate              float64               `json:"snapRate"`
	TTDSamples            int                   `json:"ttdSamples"`
	TTDMeanMS             float64               `json:"ttdMeanMs"`
	TTDMedianMS           float64               `json:"ttdMedianMs"`
	TTDWeightedMS         float64               `json:"ttdWeightedMs"`
	TTDP10MS              float64               `json:"ttdP10Ms"`
	TTDUnder190Rate       float64               `json:"ttdUnder190Rate"`
	ReactionSamples       int                   `json:"reactionSamples"`
	ReactionMedianMS      float64               `json:"reactionMedianMs"`
	ReactionWeightedMS    float64               `json:"reactionWeightedMs"`
	ReactionP10MS         float64               `json:"reactionP10Ms"`
	CrosshairMedianAngle  float64               `json:"crosshairMedianAngle"`
	FirstShotMedianAngle  float64               `json:"firstShotMedianAngle"`
	MovingShots           int                   `json:"movingShots"`
	MovingHitRate         float64               `json:"movingHitRate"`
	AirborneShots         int                   `json:"airborneShots"`
	AirborneHitRate       float64               `json:"airborneHitRate"`
	FlashedShots          int                   `json:"flashedShots"`
	FlashedHitRate        float64               `json:"flashedHitRate"`
	ScopedShots           int                   `json:"scopedShots"`
	ScopedHitRate         float64               `json:"scopedHitRate"`
	Eligible              bool                  `json:"eligible"`
	SuspicionScore        int                   `json:"suspicionScore"`
	Status                string                `json:"status"`
	TriggeredRules        []PlayerSuspicionRule `json:"triggeredRules"`
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
	MapName         string  `json:"mapName"`
	Date            string  `json:"date"`
	TickRate        float64 `json:"tickRate"`
	BuildNumber     int     `json:"buildNumber"`
	Source          string  `json:"source"`
	AnalysisVersion int     `json:"analysisVersion"`
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

func scorePlayer(row *PlayerStatsReportRow, config SuspicionConfig) {
	row.Eligible = row.DemoCount >= config.MinimumDemos && row.Shots >= config.MinimumShots
	if !row.Eligible {
		row.Status = "insufficient_sample"
		return
	}
	add := func(name string, value float64, sample, points int) {
		row.SuspicionScore += points
		row.TriggeredRules = append(row.TriggeredRules, PlayerSuspicionRule{Name: name, Value: value, Sample: sample, Points: points})
	}
	if row.TTDSamples >= config.TTDMinimumSamples && row.TTDWeightedMS <= config.TTDThresholdMS {
		add("ttd_weighted", row.TTDWeightedMS, row.TTDSamples, config.TTDPoints)
	}
	if row.TTDSamples >= config.TTDMinimumSamples && row.TTDP10MS <= config.TTDP10ThresholdMS {
		add("ttd_p10", row.TTDP10MS, row.TTDSamples, config.TTDP10Points)
	}
	if row.DamageEvents >= config.HeadHitMinimumEvents && row.HeadHitRate >= config.HeadHitRateThreshold {
		add("head_hit_rate", row.HeadHitRate, row.DamageEvents, config.HeadHitRatePoints)
	}
	if row.FirstBulletEncounters >= config.FirstBulletMinimumEncounters && row.FirstBulletHeadRate >= config.FirstBulletHeadRateThreshold {
		add("first_bullet_head_rate", row.FirstBulletHeadRate, row.FirstBulletEncounters, config.FirstBulletHeadRatePoints)
	}
	if row.FirstBulletEncounters >= config.SnapMinimumEncounters && row.SnapRate >= config.SnapRateThreshold {
		add("snap_rate", row.SnapRate, row.FirstBulletEncounters, config.SnapRatePoints)
	}
	if row.DamageEvents >= config.UnspottedMinimumEvents && row.UnspottedDamageRate >= config.UnspottedDamageRateThreshold {
		add("damage_without_confirmed_spot", row.UnspottedDamageRate, row.DamageEvents, config.UnspottedDamageRatePoints)
	}
	combinedSpecialKills := row.SmokeKills + row.WallKills
	if combinedSpecialKills >= config.SmokeWallMinimumKills && ratio(combinedSpecialKills, row.Kills) >= config.SmokeWallKillRateThreshold {
		add("smoke_wall_kill_rate", ratio(combinedSpecialKills, row.Kills), row.Kills, config.SmokeWallKillRatePoints)
	}
	if row.SuspicionScore > 100 {
		row.SuspicionScore = 100
	}
	switch {
	case row.SuspicionScore >= 70:
		row.Status = "critical"
	case row.SuspicionScore >= 50:
		row.Status = "review"
	case row.SuspicionScore >= 30:
		row.Status = "watch"
	default:
		row.Status = "normal"
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

	rows, err := db.QueryContext(ctx, `SELECT p.steam_id,p.latest_name,p.names,COUNT(s.demo_id),COALESCE(SUM(s.rounds),0),COALESCE(SUM(s.shots),0),COALESCE(SUM(s.hit_shots),0),COALESCE(SUM(s.damage_events),0),COALESCE(SUM(s.head_hit_events),0),COALESCE(SUM(s.kills),0),COALESCE(SUM(s.headshot_kills),0),COALESCE(SUM(s.smoke_kills),0),COALESCE(SUM(s.wall_kills),0),COALESCE(SUM(s.unspotted_damage_events),0),COALESCE(SUM(s.first_bullet_encounters),0),COALESCE(SUM(s.first_bullet_head_hits),0),COALESCE(SUM(s.snap_events),0),COALESCE(SUM(s.ttd_samples),0),COALESCE(SUM(s.ttd_sum_ms),0),COALESCE(SUM(s.moving_shots),0),COALESCE(SUM(s.moving_hit_shots),0),COALESCE(SUM(s.airborne_shots),0),COALESCE(SUM(s.airborne_hit_shots),0),COALESCE(SUM(s.flashed_shots),0),COALESCE(SUM(s.flashed_hit_shots),0),COALESCE(SUM(s.scoped_shots),0),COALESCE(SUM(s.scoped_hit_shots),0) FROM players p JOIN player_demo_stats s ON s.steam_id=p.steam_id GROUP BY p.steam_id,p.latest_name,p.names ORDER BY p.steam_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row PlayerStatsReportRow
		var names string
		var storedTTDSamples int
		var storedTTDSum float64
		var movingHits, airborneHits, flashedHits, scopedHits int
		if err := rows.Scan(&row.SteamID64, &row.Name, &names, &row.DemoCount, &row.Rounds, &row.Shots, &row.HitShots, &row.DamageEvents, &row.HeadHitEvents, &row.Kills, &row.HeadshotKills, &row.SmokeKills, &row.WallKills, &row.UnspottedDamageEvents, &row.FirstBulletEncounters, &row.FirstBulletHeadHits, &row.SnapEvents, &storedTTDSamples, &storedTTDSum, &row.MovingShots, &movingHits, &row.AirborneShots, &airborneHits, &row.FlashedShots, &flashedHits, &row.ScopedShots, &scopedHits); err != nil {
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
	for index := range report.Players {
		row := &report.Players[index]
		ttdRows, qerr := db.QueryContext(ctx, `SELECT e.demo_id,s.rounds,e.ttd_ms,e.confirmed_angle,e.first_shot_angle FROM encounters e JOIN player_demo_stats s ON s.demo_id=e.demo_id AND s.steam_id=e.attacker_steam_id WHERE e.attacker_steam_id=? AND e.ttd_ms BETWEEN 0 AND 1000 ORDER BY e.ttd_ms`, row.SteamID64)
		if qerr != nil {
			return nil, qerr
		}
		var ttdSamples []float64
		var crosshairAngles, firstShotAngles []float64
		ttdByDemo := make(map[int64]*demoSamples)
		under190 := 0
		for ttdRows.Next() {
			var demoID int64
			var rounds int
			var value float64
			var crosshairAngle, firstShotAngle float64
			if err := ttdRows.Scan(&demoID, &rounds, &value, &crosshairAngle, &firstShotAngle); err != nil {
				ttdRows.Close()
				return nil, err
			}
			ttdSamples = append(ttdSamples, value)
			group := ttdByDemo[demoID]
			if group == nil {
				group = &demoSamples{rounds: rounds}
				ttdByDemo[demoID] = group
			}
			group.values = append(group.values, value)
			crosshairAngles = append(crosshairAngles, crosshairAngle)
			if firstShotAngle > 0 {
				firstShotAngles = append(firstShotAngles, firstShotAngle)
			}
			if value <= 190 {
				under190++
			}
		}
		ttdRows.Close()
		row.TTDSamples = len(ttdSamples)
		row.TTDMedianMS = percentile(ttdSamples, .5)
		row.TTDWeightedMS = roundWeightedDemoMedian(ttdByDemo)
		row.TTDP10MS = percentile(ttdSamples, .1)
		row.TTDUnder190Rate = ratio(under190, len(ttdSamples))
		for _, value := range ttdSamples {
			row.TTDMeanMS += value
		}
		if row.TTDSamples > 0 {
			row.TTDMeanMS /= float64(row.TTDSamples)
		}
		row.CrosshairMedianAngle = percentile(crosshairAngles, .5)
		row.FirstShotMedianAngle = percentile(firstShotAngles, .5)

		reactionRows, reactionErr := db.QueryContext(ctx, `SELECT r.demo_id,s.rounds,r.reaction_time_ms FROM reactions r JOIN player_demo_stats s ON s.demo_id=r.demo_id AND s.steam_id=r.attacker_steam_id WHERE r.attacker_steam_id=? AND r.reaction_time_ms BETWEEN 0 AND 1000 ORDER BY r.reaction_time_ms`, row.SteamID64)
		if reactionErr != nil {
			return nil, reactionErr
		}
		var reactionSamples []float64
		reactionByDemo := make(map[int64]*demoSamples)
		for reactionRows.Next() {
			var demoID int64
			var rounds int
			var value float64
			if err := reactionRows.Scan(&demoID, &rounds, &value); err != nil {
				reactionRows.Close()
				return nil, err
			}
			reactionSamples = append(reactionSamples, value)
			group := reactionByDemo[demoID]
			if group == nil {
				group = &demoSamples{rounds: rounds}
				reactionByDemo[demoID] = group
			}
			group.values = append(group.values, value)
		}
		reactionRows.Close()
		row.ReactionSamples = len(reactionSamples)
		row.ReactionMedianMS = percentile(reactionSamples, .5)
		row.ReactionWeightedMS = roundWeightedDemoMedian(reactionByDemo)
		row.ReactionP10MS = percentile(reactionSamples, .1)
		if row.TTDWeightedMS == 0 {
			row.TTDWeightedMS = row.TTDMedianMS
		}
		if row.ReactionWeightedMS == 0 {
			row.ReactionWeightedMS = row.ReactionMedianMS
		}
		scorePlayer(row, config)
	}

	weaponRows, err := db.QueryContext(ctx, `SELECT w.steam_id,p.latest_name,w.weapon_name,SUM(w.shots),SUM(w.hit_shots),SUM(w.damage_events),SUM(w.head_hit_events),SUM(w.kills) FROM player_demo_weapon_stats w JOIN players p ON p.steam_id=w.steam_id GROUP BY w.steam_id,p.latest_name,w.weapon_name ORDER BY w.steam_id,w.weapon_name`)
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
	}
	weaponRows.Close()

	evidenceRows, err := db.QueryContext(ctx, `SELECT d.checksum,d.path,e.round_number,e.tick,e.steam_id,p.latest_name,e.victim_steam_id,e.kind,e.value,e.details FROM evidence e JOIN demos d ON d.id=e.demo_id LEFT JOIN players p ON p.steam_id=e.steam_id ORDER BY e.steam_id,d.demo_date,e.round_number,e.tick`)
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

	demoRows, err := db.QueryContext(ctx, `SELECT checksum,path,map_name,demo_date,tick_rate,build_number,source,analysis_version FROM demos ORDER BY demo_date,checksum`)
	if err != nil {
		return nil, err
	}
	for demoRows.Next() {
		var d ImportedDemoReportRow
		if err := demoRows.Scan(&d.Checksum, &d.Path, &d.MapName, &d.Date, &d.TickRate, &d.BuildNumber, &d.Source, &d.AnalysisVersion); err != nil {
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
	lines := [][]string{{"steamid", "name", "aliases", "demos", "rounds", "shots", "hit shots", "accuracy", "damage events", "head hits", "head hit rate", "kills", "headshot kills", "headshot kill rate", "smoke kills", "wall kills", "unspotted damage rate", "first bullet head rate", "snap rate", "ttd samples", "ttd mean ms", "ttd pooled median ms", "ttd weighted ms", "ttd p10 ms", "ttd under 190 rate", "reaction samples", "reaction median ms", "reaction weighted ms", "reaction p10 ms", "crosshair median angle", "first shot median angle", "moving shots", "moving hit rate", "airborne shots", "airborne hit rate", "flashed shots", "flashed hit rate", "scoped shots", "scoped hit rate", "eligible", "suspicion score", "status", "triggered rules"}}
	for _, r := range rows {
		rules := make([]string, len(r.TriggeredRules))
		for x, rule := range r.TriggeredRules {
			rules[x] = fmt.Sprintf("%s:%d(sample=%d)", rule.Name, rule.Points, rule.Sample)
		}
		lines = append(lines, []string{u(r.SteamID64), r.Name, strings.Join(r.Names, " | "), i(r.DemoCount), i(r.Rounds), i(r.Shots), i(r.HitShots), f(r.Accuracy), i(r.DamageEvents), i(r.HeadHitEvents), f(r.HeadHitRate), i(r.Kills), i(r.HeadshotKills), f(r.HeadshotKillRate), i(r.SmokeKills), i(r.WallKills), f(r.UnspottedDamageRate), f(r.FirstBulletHeadRate), f(r.SnapRate), i(r.TTDSamples), f(r.TTDMeanMS), f(r.TTDMedianMS), f(r.TTDWeightedMS), f(r.TTDP10MS), f(r.TTDUnder190Rate), i(r.ReactionSamples), f(r.ReactionMedianMS), f(r.ReactionWeightedMS), f(r.ReactionP10MS), f(r.CrosshairMedianAngle), f(r.FirstShotMedianAngle), i(r.MovingShots), f(r.MovingHitRate), i(r.AirborneShots), f(r.AirborneHitRate), i(r.FlashedShots), f(r.FlashedHitRate), i(r.ScopedShots), f(r.ScopedHitRate), strconv.FormatBool(r.Eligible), i(r.SuspicionScore), r.Status, strings.Join(rules, " | ")})
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
	lines := [][]string{{"checksum", "path", "map", "date", "tickrate", "build number", "source", "analysis version"}}
	for _, r := range rows {
		lines = append(lines, []string{r.Checksum, r.Path, r.MapName, r.Date, f(r.TickRate), i(r.BuildNumber), r.Source, i(r.AnalysisVersion)})
	}
	return writeCSV(path, lines)
}
