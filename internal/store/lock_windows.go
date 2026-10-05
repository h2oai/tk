//go:build windows

package store

import "os"

// tryFlock is a no-op on Windows: there is no flock, so concurrent tk
// processes are not serialized there.
func tryFlock(*os.File, bool) (bool, error) { return true, nil }
