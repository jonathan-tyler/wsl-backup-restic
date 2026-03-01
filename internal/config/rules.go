package config

import "path/filepath"

func IncludeRulesPath(configDir string, profile string, cadence string) string {
	return filepath.Join(configDir, profile+".include."+cadence+".txt")
}

func ExcludeRulesPath(configDir string, profile string, cadence string) string {
	return filepath.Join(configDir, profile+".exclude."+cadence+".txt")
}
