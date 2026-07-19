package api

import (
	"context"
	"database/sql"

	"github.com/fdsakk/csda/pkg/api/constants"
)

const (
	demoQualityStatusOK         = "ok"
	demoQualityStatusWarning    = "warning"
	demoQualityStatusNotChecked = "not_checked"
)

type demoQualityAssessment struct {
	Status string
	Reason string
}

// assessDemoQuality only flags a shared low-timing pattern. One fast player is
// not enough: at least four players need eight paired non-AWP samples each,
// and at least 75% of them must have both medians below the suspicious floor.
func assessDemoQuality(encounters []DemoEncounter) demoQualityAssessment {
	type timings struct {
		ttd      []float64
		reaction []float64
	}
	byPlayer := make(map[uint64]*timings)
	totalSamples := 0
	for _, encounter := range encounters {
		if encounter.AttackerSteamID64 == 0 || encounter.WeaponName == constants.WeaponAWP.String() ||
			encounter.TTDMS < 0 || encounter.TTDMS > 1000 ||
			encounter.ReactionTimeMS < 0 || encounter.ReactionTimeMS > 1000 {
			continue
		}
		player := byPlayer[encounter.AttackerSteamID64]
		if player == nil {
			player = &timings{}
			byPlayer[encounter.AttackerSteamID64] = player
		}
		player.ttd = append(player.ttd, encounter.TTDMS)
		player.reaction = append(player.reaction, encounter.ReactionTimeMS)
		totalSamples++
	}

	qualifiedPlayers, suspiciousPlayers := 0, 0
	for _, player := range byPlayer {
		if len(player.ttd) < 8 {
			continue
		}
		qualifiedPlayers++
		if percentile(player.ttd, .5) < 300 && percentile(player.reaction, .5) < 190 {
			suspiciousPlayers++
		}
	}
	if totalSamples >= 40 && qualifiedPlayers >= 4 && suspiciousPlayers*4 >= qualifiedPlayers*3 {
		return demoQualityAssessment{Status: demoQualityStatusWarning, Reason: "systemic_low_timing"}
	}
	return demoQualityAssessment{Status: demoQualityStatusOK}
}

// Existing demos are checked once after the schema migration. Only unchecked
// rows are auto-disabled, so a user's later manual override remains intact.
func auditUncheckedDemoQuality(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT id FROM demos WHERE quality_status=? ORDER BY id`, demoQualityStatusNotChecked)
	if err != nil {
		return err
	}
	var demoIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		demoIDs = append(demoIDs, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, demoID := range demoIDs {
		encounterRows, err := db.QueryContext(ctx, `SELECT attacker_steam_id,ttd_ms,reaction_time_ms,weapon_name FROM encounters WHERE demo_id=?`, demoID)
		if err != nil {
			return err
		}
		var encounters []DemoEncounter
		for encounterRows.Next() {
			var encounter DemoEncounter
			if err := encounterRows.Scan(&encounter.AttackerSteamID64, &encounter.TTDMS, &encounter.ReactionTimeMS, &encounter.WeaponName); err != nil {
				encounterRows.Close()
				return err
			}
			encounters = append(encounters, encounter)
		}
		if err := encounterRows.Close(); err != nil {
			return err
		}
		quality := assessDemoQuality(encounters)
		autoDisable := quality.Status == demoQualityStatusWarning
		if _, err := db.ExecContext(ctx, `UPDATE demos SET quality_status=?,quality_reason=?,enabled=CASE WHEN ? THEN 0 ELSE enabled END WHERE id=?`, quality.Status, quality.Reason, autoDisable, demoID); err != nil {
			return err
		}
	}
	return nil
}
