package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// PlayerStatsExportFormat identifies player stats export payloads.
	PlayerStatsExportFormat = "cs-demo-analyzer/player-stats"
	// PlayerStatsExportVersion is the current export payload version.
	// Version 2 added reactionTimeMs to encounters; version 1 payloads are
	// still importable with that field defaulting to -1.
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

type ExportedReaction struct {
	RoundNumber      int     `json:"roundNumber"`
	AttackerSteamID  string  `json:"attackerSteamId"`
	VictimSteamID    string  `json:"victimSteamId"`
	FirstSpottedTick int     `json:"firstSpottedTick"`
	ConfirmedTick    int     `json:"confirmedTick"`
	ShotTick         int     `json:"shotTick"`
	ReactionTimeMS   float64 `json:"reactionTimeMs"`
	ConfirmedTimeMS  float64 `json:"confirmedTimeMs"`
	FirstAngle       float64 `json:"firstAngle"`
	ShotAngle        float64 `json:"shotAngle"`
	WeaponName       string  `json:"weaponName"`
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
	Reactions       []ExportedReaction        `json:"reactions"`
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

	reactionRows, err := db.QueryContext(ctx, `SELECT demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,shot_tick,reaction_time_ms,confirmed_time_ms,first_angle,shot_angle,weapon_name FROM reactions ORDER BY id`)
	if err != nil {
		return nil, err
	}
	for reactionRows.Next() {
		var demoID int64
		var attacker, victim uint64
		var r ExportedReaction
		if err := reactionRows.Scan(&demoID, &r.RoundNumber, &attacker, &victim, &r.FirstSpottedTick, &r.ConfirmedTick, &r.ShotTick, &r.ReactionTimeMS, &r.ConfirmedTimeMS, &r.FirstAngle, &r.ShotAngle, &r.WeaponName); err != nil {
			reactionRows.Close()
			return nil, err
		}
		if demo := demoIDs[demoID]; demo != nil {
			r.AttackerSteamID = strconv.FormatUint(attacker, 10)
			r.VictimSteamID = strconv.FormatUint(victim, 10)
			demo.Reactions = append(demo.Reactions, r)
		}
	}
	if err := reactionRows.Close(); err != nil {
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
		// Skip demos already known by checksum, but also by file name + map:
		// the checksum covers the file size, so a re-encoded copy of the same
		// demo would otherwise import as a duplicate.
		var existingID int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM demos WHERE checksum=? OR (file_name=? AND map_name=?)`, demo.Checksum, demo.FileName, demo.MapName).Scan(&existingID)
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
		res, err := tx.ExecContext(ctx, `INSERT INTO demos(checksum,path,file_name,map_name,demo_date,tick_rate,build_number,source,analysis_version,imported_at,enabled,quality_status,quality_reason) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
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
		for _, r := range demo.Reactions {
			attacker, err := parseSteamID(r.AttackerSteamID)
			if err != nil {
				return nil, err
			}
			victim, err := parseSteamID(r.VictimSteamID)
			if err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO reactions(demo_id,round_number,attacker_steam_id,victim_steam_id,first_spotted_tick,confirmed_tick,shot_tick,reaction_time_ms,confirmed_time_ms,first_angle,shot_angle,weapon_name) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
				demoID, r.RoundNumber, attacker, victim, r.FirstSpottedTick, r.ConfirmedTick, r.ShotTick, r.ReactionTimeMS, r.ConfirmedTimeMS, r.FirstAngle, r.ShotAngle, r.WeaponName)
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
