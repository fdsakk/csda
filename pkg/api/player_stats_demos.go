package api

import (
	"context"
	"errors"
)

// ErrDemoNotFound is returned when a demo checksum does not exist in the database.
var ErrDemoNotFound = errors.New("demo not found")

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
