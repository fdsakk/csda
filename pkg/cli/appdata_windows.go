//go:build windows

package cli

import (
	"errors"
	"os"
)

func localAppDataRoot() (string, error) {
	if root := os.Getenv("LOCALAPPDATA"); root != "" {
		return root, nil
	}
	return "", errors.New("LOCALAPPDATA is not set")
}
