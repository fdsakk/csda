package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	// PlayerStatsExportFormat identifies player stats export payloads.
	PlayerStatsExportFormat = "cs-demo-analyzer/player-stats"
	// PlayerStatsExportVersion is the current export payload version.
	// Version 2 added reactionTimeMs to encounters; version 1 payloads are
	// still importable with that field defaulting to -1. The legacy "reactions"
	// list in older payloads is ignored on import — nothing ever read it.
	PlayerStatsExportVersion = 2
)

type ExportedPlayer struct {
	SteamID    string   `json:"steamId"`
	LatestName string   `json:"latestName"`
	Names      []string `json:"names"`
}

type ExportedPlayerDemoStats struct {
	SteamID               string  `json:"steamId"`
	Rounds                int     `json:"rounds"`
	Shots                 int     `json:"shots"`
	HitShots              int     `json:"hitShots"`
	DamageEvents          int     `json:"damageEvents"`
	HeadHitEvents         int     `json:"headHitEvents"`
	Kills                 int     `json:"kills"`
	Deaths                int     `json:"deaths"`
	HeadshotKills         int     `json:"headshotKills"`
	SmokeKills            int     `json:"smokeKills"`
	WallKills             int     `json:"wallKills"`
	UnspottedDamageEvents int     `json:"unspottedDamageEvents"`
	FirstBulletEncounters int     `json:"firstBulletEncounters"`
	FirstBulletHeadHits   int     `json:"firstBulletHeadHits"`
	SnapEvents            int     `json:"snapEvents"`
	TTDSamples            int     `json:"ttdSamples"`
	TTDSumMS              float64 `json:"ttdSumMs"`
	MovingShots           int     `json:"movingShots"`
	MovingHitShots        int     `json:"movingHitShots"`
	AirborneShots         int     `json:"airborneShots"`
	AirborneHitShots      int     `json:"airborneHitShots"`
	FlashedShots          int     `json:"flashedShots"`
	FlashedHitShots       int     `json:"flashedHitShots"`
	ScopedShots           int     `json:"scopedShots"`
	ScopedHitShots        int     `json:"scopedHitShots"`
}

type ExportedEncounter struct {
	RoundNumber      int     `json:"roundNumber"`
	AttackerSteamID  string  `json:"attackerSteamId"`
	VictimSteamID    string  `json:"victimSteamId"`
	FirstSpottedTick int     `json:"firstSpottedTick"`
	ConfirmedTick    int     `json:"confirmedTick"`
	DamageTick       int     `json:"damageTick"`
	TTDMS            float64 `json:"ttdMs"`
	TTDConfirmedMS   float64 `json:"ttdConfirmedMs"`
	FirstShotTimeMS  float64 `json:"firstShotTimeMs"`
	ReactionTimeMS   float64 `json:"reactionTimeMs"`
	FirstAngle       float64 `json:"firstAngle"`
	ConfirmedAngle   float64 `json:"confirmedAngle"`
	FirstShotAngle   float64 `json:"firstShotAngle"`
	DistanceMeters   float64 `json:"distanceMeters"`
	WeaponName       string  `json:"weaponName"`
	Snap             bool    `json:"snap"`
}

type ExportedWeaponStats struct {
	SteamID       string `json:"steamId"`
	WeaponName    string `json:"weaponName"`
	Shots         int    `json:"shots"`
	HitShots      int    `json:"hitShots"`
	DamageEvents  int    `json:"damageEvents"`
	HeadHitEvents int    `json:"headHitEvents"`
	Kills         int    `json:"kills"`
}

type ExportedEvidence struct {
	RoundNumber   int     `json:"roundNumber"`
	Tick          int     `json:"tick"`
	SteamID       string  `json:"steamId"`
	VictimSteamID string  `json:"victimSteamId"`
	Kind          string  `json:"kind"`
	Value         float64 `json:"value"`
	Details       string  `json:"details"`
}

type ExportedDemo struct {
	Checksum        string                    `json:"checksum"`
	Path            string                    `json:"path"`
	FileName        string                    `json:"fileName"`
	MapName         string                    `json:"mapName"`
	DemoDate        string                    `json:"demoDate"`
	TickRate        float64                   `json:"tickRate"`
	BuildNumber     int                       `json:"buildNumber"`
	Source          string                    `json:"source"`
	AnalysisVersion int                       `json:"analysisVersion"`
	ImportedAt      string                    `json:"importedAt"`
	PlayerStats     []ExportedPlayerDemoStats `json:"playerStats"`
	Encounters      []ExportedEncounter       `json:"encounters"`
	WeaponStats     []ExportedWeaponStats     `json:"weaponStats"`
	Evidence        []ExportedEvidence        `json:"evidence"`
}

type PlayerStatsExport struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	ExportedAt string           `json:"exportedAt"`
	Players    []ExportedPlayer `json:"players"`
	Demos      []ExportedDemo   `json:"demos"`
}

type PlayerStatsImportResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ExportPlayerStatsData dumps all enabled demos and their raw stats as a portable payload.
func ExportPlayerStatsData(ctx context.Context, databasePath string) (*PlayerStatsExport, error) {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	export := &PlayerStatsExport{Format: PlayerStatsExportFormat, Version: PlayerStatsExportVersion, ExportedAt: time.Now().UTC().Format(time.RFC3339)}
	demoIDs := make(map[int64]*ExportedDemo)
	steamIDs := make(map[string]bool)

	rows, err := db.QueryContext(ctx, `SELECT id,checksum,path,file_name,map_name,demo_date,tick_rate,build_number,source,analysis_version,imported_at FROM demos WHERE enabled=1 ORDER BY demo_date,checksum`)
	if err != nil {
		return nil, err
	}
	var order []int64
	for rows.Next() {
		var id int64
		var d ExportedDemo
		if err := rows.Scan(&id, &d.Checksum, &d.Path, &d.FileName, &d.MapName, &d.DemoDate, &d.TickRate, &d.BuildNumber, &d.Source, &d.AnalysisVersion, &d.ImportedAt); err != nil {
			rows.Close()
			return nil, err
		}
		demoIDs[id] = &d
		order = append(order, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	statRows, err := db.QueryContext(ctx, `SELECT demo_id,steam_id,rounds,shots,hit_shots,damage_events,head_hit_events,kills,deaths,headshot_kills,smoke_kills,wall_kills,unspotted_damage_events,first_bullet_encounters,first_bullet_head_hits,snap_events,ttd_samples,ttd_sum_ms,moving_shots,moving_hit_shots,airborne_shots,airborne_hit_shots,flashed_shots,flashed_hit_shots,scoped_shots,scoped_hit_shots FROM player_demo_stats ORDER BY demo_id,steam_id`)
	if err != nil {
		return nil, err
	}
	for statRows.Next() {
		var demoID int64
		var steamID uint64
		var s ExportedPlayerDemoStats
		if err := statRows.Scan(&demoID, &steamID, &s.Rounds, &s.Shots, &s.HitShots, &s.DamageEvents, &s.HeadHitEvents, &s.Kills, &s.Deaths, &s.HeadshotKills, &s.SmokeKills, &s.WallKills, &s.UnspottedDamageEvents, &s.FirstBulletEncounters, &s.FirstBulletHeadHits, &s.SnapEvents, &s.TTDSamples, &s.TTDSumMS, &s.MovingShots, &s.MovingHitShots, &s.AirborneShots, &s.AirborneHitShots, &s.FlashedShots, &s.FlashedHitShots, &s.ScopedShots, &s.ScopedHitShots); err != nil {
			statRows.Close()
			return nil, err
		}
		if demo := demoIDs[demoID]; demo != nil {
			s.SteamID = strconv.FormatUint(steamID, 10)
			steamIDs[s.SteamID] = true
			demo.PlayerStats = append(demo.PlayerStats, s)
		}
	}
	if err := statRows.Close(); err != nil {
		return nil, err
	}

	encounterRows, err := db.QueryContext(ctx, `SELECT demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,damage_tick,ttd_ms,ttd_confirmed_ms,first_shot_time_ms,reaction_time_ms,first_angle,confirmed_angle,first_shot_angle,distance_meters,weapon_name,snap FROM encounters ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for encounterRows.Next() {
		var demoID int64
		var attacker, victim uint64
		var e ExportedEncounter
		if err := encounterRows.Scan(&demoID, &e.RoundNumber, &attacker, &victim, &e.FirstSpottedTick, &e.ConfirmedTick, &e.DamageTick, &e.TTDMS, &e.TTDConfirmedMS, &e.FirstShotTimeMS, &e.ReactionTimeMS, &e.FirstAngle, &e.ConfirmedAngle, &e.FirstShotAngle, &e.DistanceMeters, &e.WeaponName, &e.Snap); err != nil {
			encounterRows.Close()
			return nil, err
		}
		if demo := demoIDs[demoID]; demo != nil {
			e.AttackerSteamID = strconv.FormatUint(attacker, 10)
			e.VictimSteamID = strconv.FormatUint(victim, 10)
			demo.Encounters = append(demo.Encounters, e)
		}
	}
	if err := encounterRows.Close(); err != nil {
		return nil, err
	}

	weaponRows, err := db.QueryContext(ctx, `SELECT demo_id,steam_id,weapon_name,shots,hit_shots,damage_events,head_hit_events,kills FROM player_demo_weapon_stats ORDER BY demo_id,steam_id,weapon_name`)
	if err != nil {
		return nil, err
	}
	for weaponRows.Next() {
		var demoID int64
		var steamID uint64
		var w ExportedWeaponStats
		if err := weaponRows.Scan(&demoID, &steamID, &w.WeaponName, &w.Shots, &w.HitShots, &w.DamageEvents, &w.HeadHitEvents, &w.Kills); err != nil {
			weaponRows.Close()
			return nil, err
		}
		if demo := demoIDs[demoID]; demo != nil {
			w.SteamID = strconv.FormatUint(steamID, 10)
			demo.WeaponStats = append(demo.WeaponStats, w)
		}
	}
	if err := weaponRows.Close(); err != nil {
		return nil, err
	}

	evidenceRows, err := db.QueryContext(ctx, `SELECT demo_id,round_number,tick,steam_id,victim_steam_id,kind,value,details FROM evidence ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for evidenceRows.Next() {
		var demoID int64
		var steamID, victimID uint64
		var e ExportedEvidence
		if err := evidenceRows.Scan(&demoID, &e.RoundNumber, &e.Tick, &steamID, &victimID, &e.Kind, &e.Value, &e.Details); err != nil {
			evidenceRows.Close()
			return nil, err
		}
		if demo := demoIDs[demoID]; demo != nil {
			e.SteamID = strconv.FormatUint(steamID, 10)
			e.VictimSteamID = strconv.FormatUint(victimID, 10)
			demo.Evidence = append(demo.Evidence, e)
		}
	}
	if err := evidenceRows.Close(); err != nil {
		return nil, err
	}

	playerRows, err := db.QueryContext(ctx, `SELECT steam_id,latest_name,names FROM players ORDER BY steam_id`)
	if err != nil {
		return nil, err
	}
	for playerRows.Next() {
		var steamID uint64
		var latestName, names string
		if err := playerRows.Scan(&steamID, &latestName, &names); err != nil {
			playerRows.Close()
			return nil, err
		}
		id := strconv.FormatUint(steamID, 10)
		if !steamIDs[id] {
			continue
		}
		export.Players = append(export.Players, ExportedPlayer{SteamID: id, LatestName: latestName, Names: splitPlayerNames(names)})
	}
	if err := playerRows.Close(); err != nil {
		return nil, err
	}

	for _, id := range order {
		export.Demos = append(export.Demos, *demoIDs[id])
	}
	return export, nil
}

func splitPlayerNames(names string) []string {
	var result []string
	for _, name := range strings.Split(names, "\n") {
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func parseSteamID(value string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid steam id %q", value)
	}
	return id, nil
}

func assessExportedDemoQuality(demo ExportedDemo, version int) (demoQualityAssessment, error) {
	encounters := make([]DemoEncounter, 0, len(demo.Encounters))
	for _, exported := range demo.Encounters {
		attacker, err := parseSteamID(exported.AttackerSteamID)
		if err != nil {
			return demoQualityAssessment{}, err
		}
		reaction := exported.ReactionTimeMS
		if version == 1 {
			reaction = -1
		}
		encounters = append(encounters, DemoEncounter{
			AttackerSteamID64: attacker,
			TTDMS:             exported.TTDMS,
			ReactionTimeMS:    reaction,
			WeaponName:        exported.WeaponName,
		})
	}
	return assessDemoQuality(encounters), nil
}

// validateExportedDemo rejects numbers that no analysis can produce, so a
// hand-edited or corrupted file cannot skew the aggregates.
func validateExportedDemo(demo ExportedDemo) error {
	if demo.TickRate < 0 || math.IsNaN(demo.TickRate) || math.IsInf(demo.TickRate, 0) {
		return fmt.Errorf("invalid tick rate %g", demo.TickRate)
	}
	for _, s := range demo.PlayerStats {
		counts := map[string]int{
			"rounds": s.Rounds, "shots": s.Shots, "hitShots": s.HitShots, "damageEvents": s.DamageEvents,
			"headHitEvents": s.HeadHitEvents, "kills": s.Kills, "deaths": s.Deaths, "headshotKills": s.HeadshotKills,
			"smokeKills": s.SmokeKills, "wallKills": s.WallKills, "unspottedDamageEvents": s.UnspottedDamageEvents,
			"firstBulletEncounters": s.FirstBulletEncounters, "firstBulletHeadHits": s.FirstBulletHeadHits,
			"snapEvents": s.SnapEvents, "ttdSamples": s.TTDSamples, "movingShots": s.MovingShots,
			"movingHitShots": s.MovingHitShots, "airborneShots": s.AirborneShots, "airborneHitShots": s.AirborneHitShots,
			"flashedShots": s.FlashedShots, "flashedHitShots": s.FlashedHitShots, "scopedShots": s.ScopedShots,
			"scopedHitShots": s.ScopedHitShots,
		}
		for name, value := range counts {
			if value < 0 {
				return fmt.Errorf("player %s: negative %s (%d)", s.SteamID, name, value)
			}
		}
		if !validMeasurement(s.TTDSumMS) {
			return fmt.Errorf("player %s: invalid ttdSumMs (%g)", s.SteamID, s.TTDSumMS)
		}
		for _, pair := range [][3]any{
			{"hitShots", s.HitShots, s.Shots}, {"movingHitShots", s.MovingHitShots, s.MovingShots},
			{"airborneHitShots", s.AirborneHitShots, s.AirborneShots}, {"flashedHitShots", s.FlashedHitShots, s.FlashedShots},
			{"scopedHitShots", s.ScopedHitShots, s.ScopedShots}, {"headHitEvents", s.HeadHitEvents, s.DamageEvents},
			{"headshotKills", s.HeadshotKills, s.Kills}, {"firstBulletHeadHits", s.FirstBulletHeadHits, s.FirstBulletEncounters},
		} {
			if pair[1].(int) > pair[2].(int) {
				return fmt.Errorf("player %s: %s (%d) exceeds its total (%d)", s.SteamID, pair[0], pair[1], pair[2])
			}
		}
	}
	for _, w := range demo.WeaponStats {
		for name, value := range map[string]int{"shots": w.Shots, "hitShots": w.HitShots, "damageEvents": w.DamageEvents, "headHitEvents": w.HeadHitEvents, "kills": w.Kills} {
			if value < 0 {
				return fmt.Errorf("weapon %s of player %s: negative %s (%d)", w.WeaponName, w.SteamID, name, value)
			}
		}
		if w.HitShots > w.Shots {
			return fmt.Errorf("weapon %s of player %s: hitShots (%d) exceeds shots (%d)", w.WeaponName, w.SteamID, w.HitShots, w.Shots)
		}
	}
	for _, e := range demo.Encounters {
		// -1 marks "not measured" (no shot attributed, or a version 1 payload).
		for name, value := range map[string]float64{"firstShotTimeMs": e.FirstShotTimeMS, "reactionTimeMs": e.ReactionTimeMS, "firstShotAngle": e.FirstShotAngle} {
			if !validMeasurement(value) && value != -1 {
				return fmt.Errorf("encounter in round %d: invalid %s (%g)", e.RoundNumber, name, value)
			}
		}
		for name, value := range map[string]float64{
			"ttdMs": e.TTDMS, "ttdConfirmedMs": e.TTDConfirmedMS, "firstAngle": e.FirstAngle,
			"confirmedAngle": e.ConfirmedAngle, "distanceMeters": e.DistanceMeters,
		} {
			if !validMeasurement(value) {
				return fmt.Errorf("encounter in round %d: invalid %s (%g)", e.RoundNumber, name, value)
			}
		}
	}
	return nil
}

// validMeasurement accepts finite, non-negative numbers.
func validMeasurement(value float64) bool {
	return value >= 0 && !math.IsInf(value, 0)
}

// ImportPlayerStatsData merges an export payload into the database. Demos whose
// checksum already exists are skipped; everything runs in a single transaction.
func ImportPlayerStatsData(ctx context.Context, databasePath string, payload *PlayerStatsExport) (*PlayerStatsImportResult, error) {
	if payload == nil || payload.Format != PlayerStatsExportFormat {
		return nil, fmt.Errorf("unsupported export format, expected %q", PlayerStatsExportFormat)
	}
	if payload.Version != 1 && payload.Version != PlayerStatsExportVersion {
		return nil, fmt.Errorf("unsupported export version %d, expected %d", payload.Version, PlayerStatsExportVersion)
	}
	for _, demo := range payload.Demos {
		if demo.Checksum == "" {
			return nil, errors.New("export contains a demo without a checksum")
		}
		if err := validateExportedDemo(demo); err != nil {
			return nil, fmt.Errorf("demo %s: %w", demo.Checksum, err)
		}
	}

	playerNames := make(map[uint64]ExportedPlayer, len(payload.Players))
	for _, player := range payload.Players {
		id, err := parseSteamID(player.SteamID)
		if err != nil {
			return nil, err
		}
		playerNames[id] = player
	}

	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	result := &PlayerStatsImportResult{}
	upsertPlayer := func(steamID uint64) error {
		player := playerNames[steamID]
		latestName := player.LatestName
		importedNames := ""
		for _, name := range player.Names {
			importedNames = mergePlayerName(importedNames, name)
		}
		var existingName, existingNames string
		err := tx.QueryRowContext(ctx, `SELECT latest_name,names FROM players WHERE steam_id=?`, steamID).Scan(&existingName, &existingNames)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if latestName == "" {
				latestName = "unknown"
				importedNames = mergePlayerName(importedNames, latestName)
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO players(steam_id,latest_name,names,updated_at) VALUES(?,?,?,?)`, steamID, latestName, importedNames, now)
			return err
		case err != nil:
			return err
		default:
			// existing players keep their current name but gain imported aliases
			merged := existingNames
			for _, name := range player.Names {
				merged = mergePlayerName(merged, name)
			}
			if merged == existingNames {
				return nil
			}
			_, err = tx.ExecContext(ctx, `UPDATE players SET names=?, updated_at=? WHERE steam_id=?`, merged, now, steamID)
			return err
		}
	}

	for _, demo := range payload.Demos {
		// Skip demos already known by checksum. File name and map are not an
		// identity: different matches may share both.
		var existingID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM demos WHERE checksum=?`, demo.Checksum).Scan(&existingID)
		if err == nil {
			result.Skipped++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		quality, err := assessExportedDemoQuality(demo, payload.Version)
		if err != nil {
			return nil, err
		}
		enabled := quality.Status != demoQualityStatusWarning
		res, err := tx.ExecContext(ctx, `INSERT INTO demos(checksum,path,file_name,map_name,demo_date,tick_rate,build_number,source,analysis_version,imported_at,enabled,quality_status,quality_reason,origin) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,'imported')`,
			demo.Checksum, demo.Path, demo.FileName, demo.MapName, demo.DemoDate, demo.TickRate, demo.BuildNumber, demo.Source, demo.AnalysisVersion, now, enabled, quality.Status, quality.Reason)
		if err != nil {
			return nil, err
		}
		demoID, err := res.LastInsertId()
		if err != nil {
			return nil, err
		}
		for _, s := range demo.PlayerStats {
			steamID, err := parseSteamID(s.SteamID)
			if err != nil {
				return nil, err
			}
			if err := upsertPlayer(steamID); err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO player_demo_stats(demo_id,steam_id,rounds,shots,hit_shots,damage_events,head_hit_events,kills,deaths,headshot_kills,smoke_kills,wall_kills,unspotted_damage_events,first_bullet_encounters,first_bullet_head_hits,snap_events,ttd_samples,ttd_sum_ms,moving_shots,moving_hit_shots,airborne_shots,airborne_hit_shots,flashed_shots,flashed_hit_shots,scoped_shots,scoped_hit_shots) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				demoID, steamID, s.Rounds, s.Shots, s.HitShots, s.DamageEvents, s.HeadHitEvents, s.Kills, s.Deaths, s.HeadshotKills, s.SmokeKills, s.WallKills, s.UnspottedDamageEvents, s.FirstBulletEncounters, s.FirstBulletHeadHits, s.SnapEvents, s.TTDSamples, s.TTDSumMS, s.MovingShots, s.MovingHitShots, s.AirborneShots, s.AirborneHitShots, s.FlashedShots, s.FlashedHitShots, s.ScopedShots, s.ScopedHitShots)
			if err != nil {
				return nil, err
			}
		}
		for _, e := range demo.Encounters {
			attacker, err := parseSteamID(e.AttackerSteamID)
			if err != nil {
				return nil, err
			}
			victim, err := parseSteamID(e.VictimSteamID)
			if err != nil {
				return nil, err
			}
			reactionTimeMS := e.ReactionTimeMS
			if payload.Version == 1 {
				// version 1 payloads predate the field; -1 excludes them from reports
				reactionTimeMS = -1
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO encounters(demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,damage_tick,ttd_ms,ttd_confirmed_ms,first_shot_time_ms,reaction_time_ms,first_angle,confirmed_angle,first_shot_angle,distance_meters,weapon_name,snap) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				demoID, e.RoundNumber, attacker, victim, e.FirstSpottedTick, e.ConfirmedTick, e.DamageTick, e.TTDMS, e.TTDConfirmedMS, e.FirstShotTimeMS, reactionTimeMS, e.FirstAngle, e.ConfirmedAngle, e.FirstShotAngle, e.DistanceMeters, e.WeaponName, e.Snap)
			if err != nil {
				return nil, err
			}
		}
		for _, w := range demo.WeaponStats {
			steamID, err := parseSteamID(w.SteamID)
			if err != nil {
				return nil, err
			}
			if err := upsertPlayer(steamID); err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO player_demo_weapon_stats(demo_id,steam_id,weapon_name,shots,hit_shots,damage_events,head_hit_events,kills) VALUES(?,?,?,?,?,?,?,?)`,
				demoID, steamID, w.WeaponName, w.Shots, w.HitShots, w.DamageEvents, w.HeadHitEvents, w.Kills)
			if err != nil {
				return nil, err
			}
		}
		for _, e := range demo.Evidence {
			steamID, err := parseSteamID(e.SteamID)
			if err != nil {
				return nil, err
			}
			victimID, err := parseSteamID(e.VictimSteamID)
			if err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO evidence(demo_id,round_number,tick,steam_id,victim_steam_id,kind,value,details) VALUES(?,?,?,?,?,?,?,?)`,
				demoID, e.RoundNumber, e.Tick, steamID, victimID, e.Kind, e.Value, e.Details)
			if err != nil {
				return nil, err
			}
		}
		result.Imported++
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
