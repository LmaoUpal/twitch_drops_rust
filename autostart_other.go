//go:build !windows

package main

import "errors"

func setAutostart(enabled bool) error {
	if enabled {
		return errors.New("start at sign-in is only supported on Windows")
	}
	return nil
}
