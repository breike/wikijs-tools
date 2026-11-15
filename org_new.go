// Подкоманда wiki-calendar copy-org-template: копирует шаблон org/calendar.org.example
// в целевой путь (по умолчанию /tmp/wiki-calendar.org) и подставляет в него
// текущие дату и время:
//
//   - <ДАТА_СЕГОДНЯ> ВРЕМЯ_СЕЙЧАС ПРИМЕР_СОБЫТИЯ
//   - <ДАТА_СЕГОДНЯ> ВРЕМЯ_ПЛЮС_ЧАС ПРИМЕР_СОБЫТИЯ_2
//
// Плейсхолдеры ДАТА_СЕГОДНЯ/ВРЕМЯ_СЕЙЧАС/ВРЕМЯ_ПЛЮС_ЧАС заменяются на
// реальные значения; ПРИМЕР_СОБЫТИЯ* остаются плейсхолдерами — их заполняет
// пользователь. Затем запускается импорт (org-import).
//
// Целевой путь переопределяется env WIKI_ORG_FILE (дефолт /tmp/wiki-calendar.org).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// orgTargetPath — путь, куда кладём заполненный файл (env WIKI_ORG_FILE или
// /tmp/wiki-calendar.org). Общий для copy-org-template и org-import.
func orgTargetPath() string {
	if p := os.Getenv("WIKI_ORG_FILE"); p != "" {
		return p
	}
	return "/tmp/wiki-calendar.org"
}

// orgTemplatePath — путь к шаблону в проекте. Сначала ищем рядом с бинарником
// (запуск из репозитория или установка в /usr/bin), затем в share-каталогах.
func orgTemplatePath() string {
	bin, err := os.Executable()
	if err == nil {
		base := filepath.Dir(bin)
		cands := []string{
			filepath.Join(base, "org", "calendar.org.example"), // рядом с бинарником
			filepath.Join(base, "..", "share", "wiki-calendar", "org", "calendar.org.example"),
			"/usr/share/wiki-calendar/org/calendar.org.example",
		}
		for _, p := range cands {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	// Запуск из корня репозитория.
	if _, err := os.Stat("org/calendar.org.example"); err == nil {
		return "org/calendar.org.example"
	}
	return "org/calendar.org.example" // дефолт для сообщения об ошибке
}

// futureFill подставляет в шаблон текущие дату и время.
// Возвращает заполненное содержимое и упорядоченный список плейсхолдеров,
// которые пользователь должен отредактировать (или nil, если таковых нет).
func futureFill(tmpl string, now time.Time) (string, []string) {
	date := now.Format("2006-01-02")
	cur := now.Format("15:04")
	plus := now.Add(time.Hour).Format("15:04")

	repl := strings.NewReplacer(
		"ДАТА_СЕГОДНЯ", date,
		"ВРЕМЯ_СЕЙЧАС", cur,
		"ВРЕМЯ_ПЛЮС_ЧАС", plus,
	)
	filled := repl.Replace(tmpl)

	var todo []string
	for _, ph := range []string{"ПРИМЕР_СОБЫТИЯ_2", "ПРИМЕР_СОБЫТИЯ"} {
		if strings.Contains(filled, ph) {
			todo = append(todo, ph)
		}
	}
	return filled, todo
}

// cmdCopyOrgTemplate — реализация `wiki-calendar copy-org-template`.
func cmdCopyOrgTemplate() {
	tmpl := orgTemplatePath()
	tmplData, err := os.ReadFile(tmpl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Не найден шаблон %s: %v\n", tmpl, err)
		os.Exit(1)
	}

	filled, todo := futureFill(string(tmplData), time.Now())

	target := orgTargetPath()
	if err := os.WriteFile(target, []byte(filled), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "Не удалось записать %s: %v\n", target, err)
		os.Exit(1)
	}

	fmt.Printf("Создан %s — отредактируй события:\n", target)
	for _, ph := range todo {
		fmt.Printf("  %s\n", ph)
	}
	fmt.Println("Затем запусти импорт (если он не запустится автоматически):")
	fmt.Println("  wiki-calendar org-import")
}
