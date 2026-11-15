package main

import (
	"strings"
	"testing"
	"time"
)

// Сгенерировать сетку, добавить пользовательские записи, проверить, что
// applyWeekHighlight выделяет текущий час и день недели, сохраняя записи.
func TestWeekHighlightMerge(t *testing.T) {
	// 2026-11-15 = воскресенье (Вс, idx 6), 14:xx
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)

	md, _ := buildWeekContent() // чистая сетка

	// Пользовательские правки: заметки в ячейках.
	mdEdited := strings.Replace(md, "| 14:00 |  |  |  |  |  |  |  |",
		"| 14:00 |  |  |  | встреча |  |  |  |", 1)
	mdEdited = strings.Replace(mdEdited, "| 09:00 |  |  |  |  |  |  |  |",
		"| 09:00 |  | упражнения |  |  |  |  |", 1)

	got := applyWeekHighlight(mdEdited, now, "")
	if got == mdEdited {
		t.Fatalf("ожидали изменение (выделение)\n%s", got)
	}
	// Час 14:00 выделен, заметка сохранена.
	if !strings.Contains(got, "| ==**14:00**== |  |  |  | встреча |  |  |  |") {
		t.Fatalf("14:00 не выделена с сохранением встречи:\n%s", got)
	}
	// День недели: воскресенье (последняя колонка заголовка) выделен.
	if !strings.Contains(got, "| Время | Пн | Вт | Ср | Чт | Пт | Сб | ==**Вс**== |") {
		t.Fatalf("воскресенье не выделено в заголовке:\n%s", got)
	}
	// Заметка 09:00 цела.
	if !strings.Contains(got, "| 09:00 |  | упражнения |  |  |  |  |") {
		t.Fatalf("заметки 09:00 потеряны:\n%s", got)
	}

	// Смена часа: выделение переезжает, заметки целы.
	now2 := now.Add(time.Hour)
	got2 := applyWeekHighlight(got, now2, "")
	if !strings.Contains(got2, "| ==**15:00**== |") {
		t.Fatalf("15:00 не выделена:\n%s", got2)
	}
	if strings.Contains(got2, "==**14:00**==") {
		t.Fatalf("14:00 всё ещё выделена:\n%s", got2)
	}
	if !strings.Contains(got2, "| 14:00 |  |  |  | встреча |  |  |  |") {
		t.Fatalf("встреча потеряна при снятии выделения:\n%s", got2)
	}
	if !strings.Contains(got2, "| 09:00 |  | упражнения |  |  |  |  |") {
		t.Fatalf("заметки потеряны при смене часа:\n%s", got2)
	}

	// Идемпотентность.
	if got3 := applyWeekHighlight(got2, now2, ""); got3 != got2 {
		t.Fatalf("повторный запуск изменил контент")
	}
}

// Края форматирования: метка времени без пробелов, пользовательские ==**...**==
// в ячейках дней, жирный день.
func TestWeekHighlightEdge(t *testing.T) {
	now := time.Date(2026, 11, 13, 8, 5, 0, 0, time.Local) // пятница (idx 4)
	md := "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"|07:00|  |  |  |  |  |  |  |\n" +
		"| 08:00 | **сегодня** |  |  |  |  |  |  |\n" +
		"| 09:00 |  |  |  | ==**важно**== |  |  |  |\n"
	got := applyWeekHighlight(md, now, "")

	if !strings.Contains(got, "| ==**08:00**== | **сегодня** |") {
		t.Fatalf("08:00 не выделена:\n%s", got)
	}
	// Пятница (5-я колонка данных) выделена в заголовке.
	if !strings.Contains(got, "| Время | Пн | Вт | Ср | Чт | ==**Пт**== | Сб | Вс |") {
		t.Fatalf("пятница не выделена:\n%s", got)
	}
	// 07:00 нормализована, но не выделена.
	if !strings.Contains(got, "| 07:00 |") {
		t.Fatalf("07:00 сломана:\n%s", got)
	}
	if strings.Contains(got, "==**07:00**==") {
		t.Fatalf("07:00 не должна быть выделена")
	}
	// Пользовательская пометка ==**важно**== в ячейке нетронута.
	if !strings.Contains(got, "| ==**важно**== |") {
		t.Fatalf("пользовательская пометка потеряна:\n%s", got)
	}
}

// extractWeekData парсит таблицу в dict: {"день": {"ЧЧ:ММ": "строка"}}.
func TestExtractWeekData(t *testing.T) {
	md := "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"| 09:00 |  | упражнения |  |  |  |  |  |\n" +
		"| 07:30 | йога |  |  |  |  |  |  |\n" +
		"| 14:30 |  |  |  |  | созвон |  |  |\n"

	d, labels := extractWeekData(md)
	if got := d["Вт"]["09:00"]; got != "упражнения" {
		t.Fatalf("d[Вт][09:00] = %q, ждём упражнения", got)
	}
	if got := d["Пн"]["07:30"]; got != "йога" {
		t.Fatalf("d[Пн][07:30] = %q, ждём йога", got)
	}
	if got := d["Пт"]["14:30"]; got != "созвон" {
		t.Fatalf("d[Пт][14:30] = %q, ждём созвон", got)
	}
	// labels содержит ВСЕ метки (включая ровные и кастомные), включая 09:00.
	wantLabels := map[string]bool{"09:00": true, "07:30": true, "14:30": true}
	for _, lb := range labels {
		delete(wantLabels, lb)
	}
	if len(wantLabels) != 0 {
		t.Fatalf("в labels не хватает: %v; имеем %v", wantLabels, labels)
	}
	// Пустых ячеек в dict быть не должно.
	for day, times := range d {
		for tm, val := range times {
			if val == "" {
				t.Fatalf("пустое значение в dict: %s %s", day, tm)
			}
			if !isTimeLabel(tm) {
				t.Fatalf("не-времени ключ в dict: %s -> %q", day, tm)
			}
		}
	}
}

// Парсер устойчив к отсутствию пробелов вокруг "|" — wiki.js рендерит такие
// таблицы корректно. Проверяем, что "|07:30|" и "|:--|" распознаются.
func TestExtractWeekDataNoSpaces(t *testing.T) {
	md := "|Время|Пн|Вт|Ср|Чт|Пт|Сб|Вс|\n" +
		"|:--|:--|:--|:--|:--|:--|:--|:--|\n" +
		"|07:30|йога|  |  |  |  |  |  |\n" +
		"|09:00|  |упражнения|  |  |  |  |  |\n" +
		"|14:30|  |  |  |  |созвон|  |  |\n"

	d, labels := extractWeekData(md)
	if got := d["Пн"]["07:30"]; got != "йога" {
		t.Fatalf("d[Пн][07:30] = %q, ждём йога (нет пробелов вокруг |)", got)
	}
	if got := d["Вт"]["09:00"]; got != "упражнения" {
		t.Fatalf("d[Вт][09:00] = %q, ждём упражнения", got)
	}
	if got := d["Пт"]["14:30"]; got != "созвон" {
		t.Fatalf("d[Пт][14:30] = %q, ждём созвон", got)
	}
	// Кастомная метка без пробелов попала в labels.
	found := false
	for _, lb := range labels {
		if lb == "07:30" {
			found = true
		}
	}
	if !found {
		t.Fatalf("07:30 не в labels: %v", labels)
	}
}

// rebuildWeekTable переносит пользовательские записи из старого markdown
// в новую таблицу, сохраняя и кастомные строки времени (напр. "07:30"),
// и выделение текущего времени/дня.
func TestRebuildWeekTable(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local) // воскресенье, 14:30

	// Старая таблица: заметки в ячейках, кастомная строка "07:30" без пробелов,
	// лишняя строка перед таблицей, выделения старого времени 13:00.
	old := "какой-то текст\n" +
		"| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"| 09:00 |  | упражнения |  |  |  |  |  |\n" +
		"|07:30|  |  |  |  |  |  |  |\n" +
		"| 13:00 | ==**дедлайн**== |  |  |  |  |  |  |\n" +
		"| 14:00 |  |  |  | встреча |  |  |  |\n" +
		"| 14:30 |  |  |  |  | созвон |  |  |\n"

	got := rebuildWeekTable(old, now)

	// Новая таблица: записи перенесены.
	if !strings.Contains(got, "| 09:00 |  | упражнения |  |  |  |  |  |") {
		t.Fatalf("запись 09:00 не перенесена:\n%s", got)
	}
	// Кастомная строка 07:30 сохранилась.
	if !strings.Contains(got, "| 07:30 |") {
		t.Fatalf("кастомная строка 07:30 не сохранена:\n%s", got)
	}
	// 14:30 (текущее время) выделена; встреча 14:00 перенесена, но не выделена.
	if !strings.Contains(got, "| ==**14:30**== |  |  |  |  | созвон |  |  |") {
		t.Fatalf("текущее время 14:30 не выделено (или запись не перенесена):\n%s", got)
	}
	if !strings.Contains(got, "| 14:00 |  |  |  | встреча |  |  |  |") {
		t.Fatalf("14:00 не перенесена:\n%s", got)
	}
	// День недели выделен.
	if !strings.Contains(got, "| Время | Пн | Вт | Ср | Чт | Пт | Сб | ==**Вс**== |") {
		t.Fatalf("день недели не выделен:\n%s", got)
	}
	// Старое выделение убрано.
	if strings.Contains(got, "==**дедлайн**==") {
		t.Fatalf("старое выделение осталось:\n%s", got)
	}
	// В таблице 24 ровных часа + кастомные 07:30 и 14:30 = 26 строк времени.
	hourRows := 0
	for _, line := range strings.Split(got, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		f := strings.TrimSpace(strings.Split(trimmed, "|")[1])
		// Снять возможные обёртки выделения (==**14:30**== -> 14:30).
		f = strings.ReplaceAll(strings.ReplaceAll(f, "==", ""), "**", "")
		if len(f) == 5 && f[2] == ':' {
			hourRows++
		}
	}
	if hourRows != 26 {
		t.Fatalf("ожидали 26 строк времени (24 часа + 07:30 + 14:30), получили %d:\n%s", hourRows, got)
	}
}

// buildWeekContent — чистая сетка без выделений.
func TestBuildWeekContent(t *testing.T) {
	md, html := buildWeekContent()
	if strings.Contains(md, "==**") {
		t.Fatalf("новая сетка не должна содержать выделений:\n%s", md)
	}
	if !strings.Contains(md, "| 14:00 |") || !strings.Contains(md, "| 23:00 |") {
		t.Fatalf("сетка неполная:\n%s", md)
	}
	if !strings.Contains(html, "<table>") || !strings.Contains(html, "</table>") {
		t.Fatalf("нет таблицы в html")
	}
}

// Пользователь удалил строку текущего часа — скрипт не должен ломаться.
func TestWeekHighlightMissingRow(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)
	md := "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"| 13:00 | дедлайн |  |  |  |  |  |  |\n" +
		"| 15:00 |  |  |  |  |  |  |  |\n"
	got := applyWeekHighlight(md, now, "")
	if strings.Contains(got, "==**14:00**==") {
		t.Fatalf("не должно быть выделения без строки 14:00:\n%s", got)
	}
	if !strings.Contains(got, "| 13:00 | дедлайн |") {
		t.Fatalf("запись потеряна:\n%s", got)
	}
}

func TestWeekTitle(t *testing.T) {
	if got := weekTitle(2026, 46); got != "Неделя 46 (2026)" {
		t.Fatalf("weekTitle(2026,46) = %q", got)
	}
	// Граница года: неделя 1 января 2026 принадлежит ISO-2025 (52-я неделя).
	if got := weekTitle(2025, 52); got != "Неделя 52 (2025)" {
		t.Fatalf("weekTitle(2025,52) = %q", got)
	}
}
