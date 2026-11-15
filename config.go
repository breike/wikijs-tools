// config.go — настройки скрипта.
//
// Переменные берутся из окружения (см. systemd/wiki-calendar.service),
// при отсутствии — дефолтные значения из этого файла.
package main

import (
	"os"
	"strconv"
)

type Config struct {
	Host             string
	Token            string
	RefreshIntervalS int
	SiteID           string
	Locale           string
	Verbose          bool
}

// config читает настройки из ENV, подставляя дефолты.
func config() Config {
	return Config{
		Host:             getenv("WIKI_HOST", "https://wiki.example.com"),
		Token:            getenv("WIKI_TOKEN", ""),
		RefreshIntervalS: getenvInt("WIKI_REFRESH_INTERVAL_S", 60),
		SiteID:           getenv("WIKI_SITE_ID", ""),
		Locale:           getenv("WIKI_LOCALE", ""),
		Verbose:          getenvBool("WIKI_VERBOSE", true),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := getenv(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getenvBool(key string, def bool) bool {
	v := getenv(key, "")
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
