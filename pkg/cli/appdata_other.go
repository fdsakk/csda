//go:build !windows

package cli

import "os"

func localAppDataRoot() (string, error) {
	return os.UserConfigDir()
}
