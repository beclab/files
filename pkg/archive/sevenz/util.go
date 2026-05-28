package sevenz

import "os"

// removeFn is the production implementation of removeIfExists: it
// silently swallows "not exist" errors so callers can use it as a best
// effort cleanup helper without first stat-ing.
func removeFn(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
