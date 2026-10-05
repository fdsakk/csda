package api

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strconv"

	"github.com/fdsakk/csda/pkg/api/constants"
)

// PlayerEncounterRow is one analyzed encounter of a player: the ticks needed
// to find it in the original demo and the timings derived from them.
type PlayerEncounterRow struct {
	DemoChecksum     string  `json:"demoChecksum"`
	DemoFileName     string  `json:"demoFileName"`
	MapName          string  `json:"mapName"`
	TickRate         float64 `json:"tickRate"`
	RoundNumber      int     `json:"roundNumber"`
	VictimSteamID    string  `json:"victimSteamId"`
	VictimName       string  `json:"victimName"`
	FirstSpottedTick int     `json:"firstSpottedTick"`
	ConfirmedTick    int     `json:"confirmedTick"`
	// FirstShotTick is null when no shot was recorded for the encounter. It is
	// derived from the stored reaction time, so it is exact to the tick.
	FirstShotTick *int    `json:"firstShotTick"`
	DamageTick    int     `json:"damageTick"`
	TTDMS         float64 `json:"ttdMs"`
	// ReactionMS is null for rows stored before the column existed.
	ReactionMS *float64 `json:"reactionMs"`
	// ReactionEstimated is true when no shot was recorded and the reaction is
	// the time to damage instead of the time to the first shot.
	ReactionEstimated bool     `json:"reactionEstimated"`
	ConfirmedAngle    float64  `json:"confirmedAngle"`
	FirstShotAngle    *float64 `json:"firstShotAngle"`
	DistanceMeters    float64  `json:"distanceMeters"`
	WeaponName        string   `json:"weaponName"`
	AWP               bool     `json:"awp"`
	Snap              bool     `json:"snap"`
	// Counted is false when the timing falls outside the 0–1000 ms window and
	// therefore does not enter the aggregates.
	Counted bool `json:"counted"`
}

// GetPlayerEncounters lists the player's encounters from the enabled demos,
// the same population the aggregated report is computed from.
func GetPlayerEncounters(ctx context.Context, databasePath string, steamID uint64) ([]PlayerEncounterRow, error) {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var known int
	if err := db.QueryRowContext(ctx, `SELECT 1 FROM players WHERE steam_id=?`, steamID).Scan(&known); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPlayerNotFound
	} else if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT d.checksum,d.file_name,d.map_name,d.tick_rate,e.round_number,e.victim_steam_id,COALESCE(v.latest_name,''),e.first_spotted_tick,e.confirmed_tick,e.damage_tick,e.ttd_ms,e.first_shot_time_ms,e.reaction_time_ms,e.confirmed_angle,e.first_shot_angle,e.distance_meters,e.weapon_name,e.snap
FROM encounters e JOIN demos d ON d.id=e.demo_id AND d.enabled=1 LEFT JOIN players v ON v.steam_id=e.victim_steam_id
WHERE e.attacker_steam_id=? ORDER BY d.demo_date,d.checksum,e.round_number,e.damage_tick`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	awpName := constants.WeaponAWP.String()
	result := []PlayerEncounterRow{}
	for rows.Next() {
		var r PlayerEncounterRow
		var victim uint64
		var firstShotTimeMS, reactionMS, firstShotAngle float64
		if err := rows.Scan(&r.DemoChecksum, &r.DemoFileName, &r.MapName, &r.TickRate, &r.RoundNumber, &victim, &r.VictimName,
			&r.FirstSpottedTick, &r.ConfirmedTick, &r.DamageTick, &r.TTDMS, &firstShotTimeMS, &reactionMS, &r.ConfirmedAngle, &firstShotAngle,
			&r.DistanceMeters, &r.WeaponName, &r.Snap); err != nil {
			return nil, err
		}
		r.VictimSteamID = strconv.FormatUint(victim, 10)
		r.AWP = r.WeaponName == awpName
		r.Counted = r.TTDMS >= 0 && r.TTDMS <= 1000

		// first_shot_time_ms is -1 when no shot was recorded, in every analysis version.
		shotRecorded := firstShotTimeMS >= 0
		r.ReactionEstimated = !shotRecorded
		if reactionMS >= 0 {
			r.ReactionMS = &reactionMS
			if shotRecorded && r.TickRate > 0 {
				tick := r.FirstSpottedTick + int(math.Round(reactionMS*r.TickRate/1000))
				r.FirstShotTick = &tick
			}
		}
		if shotRecorded {
			r.FirstShotAngle = &firstShotAngle
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
