package main

import (
	"strings"
	"testing"
	"time"
)

func TestHourHighlightMerge(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)
	md, _ := buildWeekContent(now)

	mdEdited := strings.Replace(md, "| ==**14:00**== |  |  |  |  |  |  |  |",
		"| 14:00 |  |  |  | встреча |  |  |  |", 1)
	mdEdited = strings.Replace(mdEdited, "| 09:00 |  |  |  |  |  |  |  |",
		"| 09:00 |  | упражнения |  |  |  |  |", 1)

	got1 := applyHourHighlight(mdEdited, now)
	if got1 == mdEdited {
		t.Fatalf("ожидали изменение (восстановление выделения 14:00)\n%s", got1)
	}
	if !strings.Contains(got1, "| ==**14:00**== |  |  |  | встреча |  |  |  |") {
		t.Fatalf("14:00 не выделена с сохранением встречи:\n%s", got1)
	}
	if !strings.Contains(got1, "| 09:00 |  | упражнения |  |  |  |  |") {
		t.Fatalf("заметки 09:00 потеряны:\n%s", got1)
	}

	now2 := now.Add(time.Hour)
	got2 := applyHourHighlight(got1, now2)
	if !strings.Contains(got2, "| ==**15:00**== |") {
		t.Fatalf("15:00 не выделена:\n%s", got2)
	}
	if strings.Contains(got2, "==**14:00**==") {
		t.Fatalf("14:00 всё ещё выделена:\n%s", got2)
	}
	if !strings.Contains(got2, "| 09:00 |  | упражнения |  |  |  |  |") {
		t.Fatalf("заметки потеряны при смене часа:\n%s", got2)
	}
	if !strings.Contains(got2, "| 14:00 |  |  |  | встреча |  |  |  |") {
		t.Fatalf("встреча потеряна при снятии выделения:\n%s", got2)
	}

	got3 := applyHourHighlight(got2, now2)
	if got3 != got2 {
		t.Fatalf("повторный запуск изменил контент")
	}
}

// Угловые: пользователь вставил ==**...**== в ячейку дня (не время), метка времени без пробелов.
func TestHourHighlightEdge(t *testing.T) {
	now := time.Date(2026, 11, 15, 8, 5, 0, 0, time.Local)
	md := "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"|07:00|  |  |  |  |  |  |  |\n" +
		"| 08:00 | **сегодня** |  |  |  |  |  |  |\n" +
		"| 09:00 |  |  |  | ==**важно**== |  |  |  |\n"
	got := applyHourHighlight(md, now)
	// 08:00 выделена
	if !strings.Contains(got, "| ==**08:00**== | **сегодня** |") {
		t.Fatalf("08:00 не выделена:\n%s", got)
	}
	// 07:00 без выделения — нормализована, но содержимое цело
	if !strings.Contains(got, "| 07:00 |") {
		t.Fatalf("07:00 сломана:\n%s", got)
	}
	if strings.Contains(got, "==**07:00**==") {
		t.Fatalf("07:00 не должна быть выделена")
	}
	// ==**важно**== в ячейке вторника нетронут
	if !strings.Contains(got, "| ==**важно**== |") {
		t.Fatalf("пользовательская пометка потеряна:\n%s", got)
	}
	// Никаких других ==**...**== на строках времени (кроме 08:00) нет
	if strings.Contains(got, "==**09:00**==") {
		t.Fatalf("09:00 не должна быть выделена:\n%s", got)
	}
}

// Хелпер: убрать ==**...**== со строки времени (не нужен, но для полноты не нужен)
func TestBuildWeekContentHighlights(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)
	md, html := buildWeekContent(now)
	if !strings.Contains(md, "| ==**14:00**== |") {
		t.Fatalf("md не содержит выделение 14:00")
	}
	if !strings.Contains(html, "<mark><strong>14:00</strong></mark>") {
		t.Fatalf("html не содержит выделение 14:00")
	}
	// Ровно один выделенный час (==**HH:00**==)
	c := strings.Count(md, "==**")
	if c != 1 {
		t.Fatalf("должен быть ровно один выделенный час, count== (%d):\n%s", c, md)
	}
	if !strings.Contains(md, "| ==**14:00**== |") {
		t.Fatalf("нет ==**14:00**== в md")
	}
}

// Пользователь удалил строку 14:00 полностью — скрипт не должен ломаться,
// а только отметить существующие.
func TestHourHighlightMissingRow(t *testing.T) {
	now := time.Date(2026, 11, 15, 14, 30, 0, 0, time.Local)
	md := "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |\n" +
		"| :-- | :-- | :-- | :-- | :-- | :-- | :-- | :-- |\n" +
		"| 13:00 | дедлайн |  |  |  |  |  |  |\n" +
		"| 15:00 |  |  |  |  |  |  |  |\n"
	got := applyHourHighlight(md, now)
	// 14:00 нет — ничего не выделено, 13:00 цела
	if strings.Contains(got, "==**") {
		t.Fatalf("не должно быть выделения без строки 14:00:\n%s", got)
	}
	if !strings.Contains(got, "| 13:00 | дедлайн |") {
		t.Fatalf("запись потеряна:\n%s", got)
	}
	if got != md {
		t.Fatalf("контент не должен меняться без изменений:\n%s\nvs\n%s", got, md)
	}
}
