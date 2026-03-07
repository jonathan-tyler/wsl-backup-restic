package config

import "path/filepath"

func IncludeRulesPath(configDir string, profile string, cadence string) string {
	return filepath.Join(configDir, "includes."+cadence+".txt")
}

func ExcludeRulesPath(configDir string) string {
	return filepath.Join(configDir, "excludes.txt")
}
