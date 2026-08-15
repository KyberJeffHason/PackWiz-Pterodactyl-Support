package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Listen, PublicListen, DataDir, Token, PackwizBinary, PublicBaseURL, CurseForgeKey string
	MaxUpload                                                                         int64
	CommandTimeout                                                                    time.Duration
}

func Load() (Config, error) {
	c := Config{Listen: env("PWM_LISTEN", "127.0.0.1:8090"), PublicListen: env("PWM_PUBLIC_LISTEN", "127.0.0.1:8091"), DataDir: env("PWM_DATA_DIR", "/srv/packwiz-manager"), Token: os.Getenv("PWM_SERVICE_TOKEN"), PackwizBinary: env("PWM_PACKWIZ_BIN", "/usr/lib/packwiz-manager/packwiz"), PublicBaseURL: env("PWM_PUBLIC_BASE_URL", "http://127.0.0.1:8091/public"), CurseForgeKey: os.Getenv("PWM_CURSEFORGE_API_KEY"), MaxUpload: 256 << 20, CommandTimeout: 2 * time.Minute}
	if v := os.Getenv("PWM_MAX_UPLOAD_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 1 {
			return c, errors.New("invalid PWM_MAX_UPLOAD_BYTES")
		}
		c.MaxUpload = n
	}
	if len(c.Token) < 32 {
		return c, errors.New("PWM_SERVICE_TOKEN must contain at least 32 characters")
	}
	return c, nil
}
func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
