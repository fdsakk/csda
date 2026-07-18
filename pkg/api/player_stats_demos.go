package api

import (
	"context"
	"database/sql"
	"errors"
)

// ErrDemoNotFound is returned when a demo checksum does not exist in the database.
var ErrDemoNotFound = errors.New("demo not found")

// ErrPlayerNotFound is returned when a steam id does not exist in the database.
var ErrPlayerNotFound = errors.New("player not found")

// SetDemoEnabled includes or excludes a demo from the player stats report.
func SetDemoEnabled(ctx context.Context, databasePath, checksum string, enabled bool) error {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.ExecContext(ctx, `UPDATE demos SET enabled=? WHERE checksum=?`, enabled, checksum)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrDemoNotFound
	}
	return nil
}

// SetAllDemosEnabled includes or excludes every imported demo from reports.
func SetAllDemosEnabled(ctx context.Context, databasePath string, enabled bool) error {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, `UPDATE demos SET enabled=?`, enabled)
	return err
}

// DeleteDemo permanently removes a demo and all of its stats (player demo
// stats, encounters, reactions, weapon stats and evidence cascade with it).
// It returns the stored demo path so callers can also remove the file.
func DeleteDemo(ctx context.Context, databasePath, checksum string) (string, error) {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var path string
	err = db.QueryRowContext(ctx, `SELECT path FROM demos WHERE checksum=?`, checksum).Scan(&path)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrDemoNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM demos WHERE checksum=?`, checksum); err != nil {
		return "", err
	}
	return path, nil
}

// SetPlayerSaved marks or unmarks a player as manually tracked.
func SetPlayerSaved(ctx context.Context, databasePath string, steamID uint64, saved bool) error {
	return setPlayerFlag(ctx, databasePath, steamID, "saved", saved)
}

// SetPlayerBanned marks or unmarks a player as already banned.
func SetPlayerBanned(ctx context.Context, databasePath string, steamID uint64, banned bool) error {
	return setPlayerFlag(ctx, databasePath, steamID, "banned", banned)
}

func setPlayerFlag(ctx context.Context, databasePath string, steamID uint64, column string, value bool) error {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.ExecContext(ctx, `UPDATE players SET `+column+`=? WHERE steam_id=?`, value, steamID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPlayerNotFound
	}
	return nil
}
