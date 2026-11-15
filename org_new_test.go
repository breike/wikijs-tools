package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

// futureFill подставляет текущие дату/время в шаблон.
func TestFutureFill(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)
	tmpl := "* <ДАТА_СЕГОДНЯ> ВРЕМЯ_СЕЙЧАС ПРИМЕР_СОБЫТИЯ\n" +
		"* <ДАТА_СЕГОДНЯ> ВРЕМЯ_ПЛЮС_ЧАС ПРИМЕР_СОБЫТИЯ_2\n"

	filled, todo := futureFill(tmpl, now)

	if !strings.Contains(filled, "* <2026-11-15> 14:30 ПРИМЕР_СОБЫТИЯ") {
		t.Fatalf("строка 1 не подставлена:\n%s", filled)
	}
	if !strings.Contains(filled, "* <2026-11-15> 15:30 ПРИМЕР_СОБЫТИЯ_2") {
		t.Fatalf("строка 2 (плюс час) не подставлена:\n%s", filled)
	}
	// Оставшиеся плейсхолдеры перечислены в todo.
	if len(todo) != 2 || todo[0] != "ПРИМЕР_СОБЫТИЯ_2" || todo[1] != "ПРИМЕР_СОБЫТИЯ" {
		t.Fatalf("todo = %v, ждём [ПРИМЕР_СОБЫТИЯ_2 ПРИМЕР_СОБЫТИЯ]", todo)
	}
	if strings.Contains(filled, "ДАТА_СЕГОДНЯ") || strings.Contains(filled, "ВРЕМЯ_") {
		t.Fatalf("не заменились все плейсхолдеры даты/времени:\n%s", filled)
	}
}

// Пограничный случай: полночь — время_плюс_час переходит на следующий день.
// В шаблоне обе строки используют ДАТА_СЕГОДНЯ (общий плейсхолдер), поэтому
// время_плюс_час = 00:45 на той же дате; пользователь при желании вручную
// правит вторую строку.
func TestFutureFillMidnight(t *testing.T) {
	now := time.Date(2026, 11, 15, 23, 45, 0, 0, time.Local)
	tmpl := "* <ДАТА_СЕГОДНЯ> ВРЕМЯ_СЕЙЧАС ПРИМЕР_СОБЫТИЯ\n" +
		"* <ДАТА_СЕГОДНЯ> ВРЕМЯ_ПЛЮС_ЧАС ПРИМЕР_СОБЫТИЯ_2\n"

	filled, _ := futureFill(tmpl, now)

	// 23:45 + 1ч = 00:45; дата остаётся сегодняшней (общий плейсхолдер).
	if !strings.Contains(filled, "* <2026-11-15> 00:45") {
		t.Fatalf("полночь: время_плюс_час не подставлено:\n%s", filled)
	}
}

// cmdCopyOrgTemplate создаёт файл по целевому пути и подставляет дату/время.
func TestCmdCopyOrgTemplate(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/wiki-calendar.org"
	t.Setenv("WIKI_ORG_FILE", target)

	cmdCopyOrgTemplate()

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if !strings.Contains(string(data), "<"+time.Now().Format("2006-01-02")+">") {
		t.Fatalf("дата не подставлена:\n%s", data)
	}
	if !strings.Contains(string(data), time.Now().Format("15:04")) {
		t.Fatalf("время не подставлено:\n%s", data)
	}
}
