package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

const thresholdsSettingKey = "thresholds"

// migrateSuspicionConfig updates only values that exactly match historical
// defaults. Explicit user calibration remains untouched.
func migrateSuspicionConfig(config SuspicionConfig) SuspicionConfig {
	if config.SampleConfidenceFloor == .75 && config.SampleConfidenceK == 30 {
		defaults := DefaultSuspicionConfig()
		config.SampleConfidenceFloor = defaults.SampleConfidenceFloor
		config.SampleConfidenceK = defaults.SampleConfidenceK
	}
	return config
}

// GetThresholds returns the persisted suspicion config, or false when none has
// been saved yet (the caller falls back to defaults).
func GetThresholds(ctx context.Context, databasePath string) (SuspicionConfig, bool, error) {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return SuspicionConfig{}, false, err
	}
	defer db.Close()
	var raw string
	err = db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, thresholdsSettingKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return SuspicionConfig{}, false, nil
	}
	if err != nil {
		return SuspicionConfig{}, false, err
	}
	var config SuspicionConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return SuspicionConfig{}, false, err
	}
	return migrateSuspicionConfig(config), true, nil
}

// SaveThresholds persists the suspicion config so it survives restarts.
func SaveThresholds(ctx context.Context, databasePath string, config SuspicionConfig) error {
	db, err := openPlayerStatsDB(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value`, thresholdsSettingKey, string(raw))
	return err
}
