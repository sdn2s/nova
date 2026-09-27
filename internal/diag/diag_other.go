//go:build !windows

package diag

func platformChecks(env Env) []Check {
	return []Check{{Name: "windows checks", Level: Warn, Detail: "skipped: not Windows"}}
}
