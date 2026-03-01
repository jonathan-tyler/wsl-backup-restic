package platform

import "fmt"

type GetenvFunc func(string) string

type WSLGuard struct {
	Getenv GetenvFunc
}

func NewWSLGuard(getenv GetenvFunc) WSLGuard {
	return WSLGuard{Getenv: getenv}
}

func (g WSLGuard) Validate() error {
	if g.Getenv == nil {
		return fmt.Errorf("environment reader is not configured")
	}

	if g.Getenv("WSL_DISTRO_NAME") == "" {
		return fmt.Errorf("this CLI is WSL-only; run it from a WSL shell")
	}

	return nil
}
