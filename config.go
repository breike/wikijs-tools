// config.go — настройки скрипта.
//
// Источники (по приоритету):
//   1. Переменные окружения (WIKI_HOST, WIKI_TOKEN, ...) — их задаёт systemd
//      через EnvironmentFile=/etc/wiki-calendar.env или OpenRC через conf.d;
//   2. Если переменной нет в окружении, читаем её из файлов конфигурации:
//        /etc/wiki-calendar.env          (systemd / общий)
//        /etc/conf.d/wiki-calendar       (OpenRC)
//   3. Иначе — дефолт из этого файла.
package main

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host             string
	Token            string
	RefreshIntervalS int
	SiteID           string
	Locale           string
	Verbose          bool
}

// configPaths — файлы, из которых дозаполняются пропущенные переменные.
// WIKI_CONFIG_PATHS (двоеточие-разделитель) позволяет переопределить для тестов.
func configPathList() []string {
	if env := os.Getenv("WIKI_CONFIG_PATHS"); env != "" {
		return strings.Split(env, ":")
	}
	return []string{"/etc/wiki-calendar.env", "/etc/conf.d/wiki-calendar"}
}

// config читает настройки: сначала ENV, затем файлы конфигурации, затем дефолты.
func config() Config {
	cfg := Config{
		Host:             getenv("WIKI_HOST"),
		Token:            getenv("WIKI_TOKEN"),
		RefreshIntervalS: getenvInt("WIKI_REFRESH_INTERVAL_S", 0),
		SiteID:           getenv("WIKI_SITE_ID"),
		Locale:           getenv("WIKI_LOCALE"),
		Verbose:          getenvBool("WIKI_VERBOSE", false),
	}

	// Дозаполняем пропущенные значения из конфиг-файлов.
	if cfg.Host == "" || cfg.Token == "" || cfg.SiteID == "" || cfg.Locale == "" || cfg.RefreshIntervalS == 0 || !cfg.Verbose {
		vals := readConfigFiles()
		if cfg.Host == "" {
			cfg.Host = firstNonEmpty(vals["WIKI_HOST"], "https://wiki.example.com")
		}
		if cfg.Token == "" {
			cfg.Token = vals["WIKI_TOKEN"]
		}
		if cfg.RefreshIntervalS == 0 {
			cfg.RefreshIntervalS = atoiDefault(vals["WIKI_REFRESH_INTERVAL_S"], 60)
		}
		if cfg.SiteID == "" {
			cfg.SiteID = vals["WIKI_SITE_ID"]
		}
		if cfg.Locale == "" {
			cfg.Locale = vals["WIKI_LOCALE"]
		}
		if !cfg.Verbose {
			cfg.Verbose = parseBool(vals["WIKI_VERBOSE"], true)
		}
	}
	return cfg
}

// readConfigFiles читает все файлы конфигурации и собирает KEY=VALUE.
// Последний файл в списке имеет приоритет при дубликатах.
func readConfigFiles() map[string]string {
	out := map[string]string{}
	for _, p := range configPathList() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			eq := strings.Index(line, "=")
			if eq <= 0 {
				continue
			}
			key := strings.TrimSpace(line[:eq])
			val := strings.TrimSpace(line[eq+1:])
			// Убрать возможные кавычки: KEY="value" / KEY='value'
			val = strings.Trim(val, `"'`)
			if key != "" {
				out[key] = val
			}
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func parseBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// getenv возвращает значение переменной окружения или "" (не дефолт!).
func getenv(key string) string {
	return os.Getenv(key)
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return atoiDefault(v, def)
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return parseBool(v, def)
}