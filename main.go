// Публикует календарь текущего года в Wiki.js как страницу cal-<год>.
//
// Формат: для каждого месяца markdown-таблица (Неделя | Пн..Вс | link),
// текущий день выделен ==**N**== (markdown) / <mark><strong>N</strong></mark> (HTML).
//
// Работает в фоне: раз в RefreshIntervalS пересобирает календарь и пишет
// страницу только если контент изменился (иначе каждая проверка создавала бы
// версию в истории). При наступлении нового года автоматически создаёт страницу
// cal-<новый год> и дальше пишет в неё.
//
// Используется REST API Wiki.js 3.x (Fastify): /_api.
//
// Контракт (из исходников requarks/wiki):
//
//	GET    /_api/bootstrap                          -> { site: { id, ... } } (определение siteId)
//	GET    /_api/sites/{sid}/pages/{hash}
//	       ?withContent=true&path=...               -> страница (content отдаётся только с правом)
//	POST   /_api/sites/{sid}/pages                  -> { ok, message, page } ; body PageInput
//	PATCH  /_api/sites/{sid}/pages/{pageId}         -> { ok, message, page } ; body Partial<PageInput>
//	Аутентификация: Authorization: Bearer <token>  (system API key или personal access token)
//	Страницы адресуются по hash пути: generatePathHash() в backend/helpers/common.ts.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// cfg — глобальная конфигурация, читается из ENV при старте (см. config.go
// и systemd/wiki-calendar.service).
var cfg = config()

// ---------- Календарь ----------

var monthsRU = [12]string{
	"Январь", "Февраль", "Март", "Апрель", "Май", "Июнь",
	"Июль", "Август", "Сентябрь", "Октябрь", "Ноябрь", "Декабрь",
}

var weekdays = [7]string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}

// day — одна клетка календаря (число или пустая).
type day struct {
	num     int // 0 = пустая клетка
	isToday bool
}

// week — строка календаря: iso-номер недели, iso-год недели и 7 дней (Пн..Вс).
// isoYear отличается от календарного года на границах: первая неделя января
// может принадлежать ISO-году прошлого года, последняя неделя декабря — ISO-году следующего.
type week struct {
	iso     int
	isoYear int
	days    [7]day
}

// monthWeeks возвращает недели месяца: от понедельника недели, в которую
// попадает 1-е число, до понедельника недели, содержащей последний день месяца.
func monthWeeks(year, month int, today time.Time) []week {
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
	startDow := (int(first.Weekday()) + 6) % 7 // 0 = Понедельник
	daysInMonth := time.Date(year, time.Month(month)+1, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, -1).Day()

	firstWeekStart := first.AddDate(0, 0, -startDow)
	last := time.Date(year, time.Month(month), daysInMonth, 0, 0, 0, 0, time.Local)
	lastDow := (int(last.Weekday()) + 6) % 7
	lastWeekStart := last.AddDate(0, 0, -lastDow)

	sameDate := func(a, b time.Time) bool {
		return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
	}

	weeks := []week{}
	for cur := firstWeekStart; !cur.After(lastWeekStart); cur = cur.AddDate(0, 0, 7) {
		var w week
		w.isoYear, w.iso = cur.ISOWeek() // ISOWeek() возвращает (year, week)
		for i := 0; i < 7; i++ {
			d := cur.AddDate(0, 0, i)
			if d.Month() == time.Month(month) {
				w.days[i] = day{num: d.Day(), isToday: sameDate(d, today)}
			} else {
				w.days[i] = day{num: 0}
			}
		}
		weeks = append(weeks, w)
	}
	return weeks
}

func dayMD(d day) string {
	if d.num == 0 {
		return ""
	}
	if d.isToday {
		return fmt.Sprintf("==**%d**==", d.num)
	}
	return strconv.Itoa(d.num)
}

func dayHTML(d day) string {
	if d.num == 0 {
		return "<td></td>"
	}
	if d.isToday {
		return fmt.Sprintf("<td><mark><strong>%d</strong></mark></td>", d.num)
	}
	return fmt.Sprintf("<td>%d</td>", d.num)
}

// weekPath — путь страницы недели: cal-<iso-год>-week-<номер недели>.
// iso-год, а не календарный: неделя 1 января может принадлежать прошлому ISO-году,
// последняя неделя декабря — следующему.
func weekPath(isoYear, iso int) string {
	return fmt.Sprintf("cal-%d-week-%d", isoYear, iso)
}

// weekLinkMD — первая колонка строки недели в markdown: ссылка на страницу недели.
func weekLinkMD(w week) string {
	return fmt.Sprintf("[%d](%s)", w.iso, weekPath(w.isoYear, w.iso))
}

// weekLinkHTML — первая колонка строки недели в HTML: ссылка на страницу недели.
func weekLinkHTML(w week) string {
	return fmt.Sprintf("<td><a href=\"%s\">%d</a></td>", weekPath(w.isoYear, w.iso), w.iso)
}

// buildCalendar возвращает (markdown_content, html_render) на весь год.
func buildCalendar(year int, today time.Time) (string, string) {
	var mdParts, htmlParts []string

	for month := 1; month <= 12; month++ {
		title := monthsRU[month-1]
		mdParts = append(mdParts, fmt.Sprintf("## %s\n", title))
		htmlParts = append(htmlParts, fmt.Sprintf("<h2>%s</h2>", title))

		// Markdown-таблица: номер недели — ссылка на её страницу.
		header := "| Неделя | " + strings.Join(weekdays[:], " | ") + " |"
		sep := "| :-- | " + strings.Join([]string{":--", ":--", ":--", ":--", ":--", ":--", ":--"}, " | ") + " |"
		mdParts = append(mdParts, header, sep)
		for _, w := range monthWeeks(year, month, today) {
			cells := []string{weekLinkMD(w)}
			for _, d := range w.days {
				cells = append(cells, dayMD(d))
			}
			mdParts = append(mdParts, "| "+strings.Join(cells, " | ")+" |")
		}
		mdParts = append(mdParts, "")

		// HTML-рендер (эквивалент markdown-it): номер недели — ссылка на её страницу.
		rows := []string{}
		for _, w := range monthWeeks(year, month, today) {
			var cells strings.Builder
			for _, d := range w.days {
				cells.WriteString(dayHTML(d))
			}
			rows = append(rows, fmt.Sprintf("<tr>%s%s</tr>", weekLinkHTML(w), cells.String()))
		}
		heads := append([]string{"Неделя"}, weekdays[:]...)
		var th strings.Builder
		for _, h := range heads {
			th.WriteString(fmt.Sprintf("<th>%s</th>", h))
		}
		htmlParts = append(htmlParts,
			"<table>\n<thead>\n<tr>"+th.String()+"</tr>\n</thead>\n<tbody>\n"+
				strings.Join(rows, "\n")+"\n</tbody>\n</table>\n")
	}

	return strings.TrimSpace(strings.Join(mdParts, "\n")) + "\n",
		strings.TrimSpace(strings.Join(htmlParts, "\n")) + "\n"
}

// ---------- Хеш пути (копия generatePathHash из Wiki.js) ----------

// generatePathHash — 53-битный хеш, hex-строка.
// Точная копия backend/helpers/common.ts (Math.imul, беззнаковая 32-битная арифметика).
func generatePathHash(path string, seed uint32) string {
	const (
		imul1 = 2654435761
		imul2 = 1597334677
		imul3 = 2246822507
		imul4 = 3266489909
	)
	imul := func(a, b uint32) uint32 { return a * b } // JS Math.imul == uint32 умножение по модулю 2^32

	h1 := uint32(0xdeadbeef) ^ seed
	h2 := uint32(0x41c6ce57) ^ seed
	for i := 0; i < len(path); i++ {
		ch := uint32(path[i])
		h1 = imul(h1^ch, imul1)
		h2 = imul(h2^ch, imul2)
	}
	h1 = imul(h1^(h1>>16), imul3)
	h1 ^= imul(h2^(h2>>13), imul4)
	h2 = imul(h2^(h2>>16), imul3)
	h2 ^= imul(h1^(h1>>13), imul4)

	low := uint64(h1)                     // h1 >>> 0
	high := uint64(0x1fffff) & uint64(h2) // 2097151 & h2
	return fmt.Sprintf("%x", high<<32|low)
}

// ---------- API ----------

type apiError struct {
	status int
	raw    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.status, e.raw)
}

// api выполняет запрос к /_api. body == nil означает запрос без тела.
// Результат — декодированный JSON (объект). Non-2xx — error.
func api(method, path string, body any) (map[string]any, error) {
	reqURL := cfg.Host + "/_api" + path

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, reqURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &apiError{status: resp.StatusCode, raw: truncate(string(raw), 300)}
	}

	var out map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &out); err != nil {
			// Некоторые ответы — массивы; возвращаем как есть в special-ключе
			var arr []any
			if aerr := json.Unmarshal(raw, &arr); aerr == nil {
				return map[string]any{"_list": arr}, nil
			}
			return nil, err
		}
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func getSiteID() (string, error) {
	if cfg.SiteID != "" {
		return cfg.SiteID, nil
	}
	data, err := api("GET", "/bootstrap", nil)
	if err != nil {
		return "", fmt.Errorf("не удалось получить bootstrap: %w", err)
	}
	site, _ := data["site"].(map[string]any)
	sid, _ := site["id"].(string)
	if sid == "" {
		return "", errors.New("в ответе bootstrap нет site.id")
	}
	return sid, nil
}

// ---------- Страница недели ----------

// weekHourHighlight — markdown-обёртка для выделения метки (часа или дня недели)
// в таблице недели.
func weekHourHighlight(label string) string {
	return fmt.Sprintf("==**%s**==", label)
}

// hourLabel — время текущего часа в формате "HH:00".
func hourLabel(t time.Time) string {
	return fmt.Sprintf("%02d:00", t.Hour())
}

// weekdayIndex — индекс сегодняшнего дня в [7]string{Пн..Вс} (0 = Пн, 6 = Вс).
func weekdayIndex(t time.Time) int {
	return (int(t.Weekday()) + 6) % 7
}

// applyWeekHighlight применяет к markdown-сетке недели выделение текущего
// часа (первая колонка строки "| HH:00 |") и текущего дня недели (заголовок
// колонки "| Время | Пн | ... |"). Прежние ==**...**== с тех же мест убираются.
// Пользовательские записи в ячейках таблицы не трогаются.
func applyWeekHighlight(md string, today time.Time) string {
	curHour := hourLabel(today)
	curDow := weekdayIndex(today) // 0 = Пн
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cells := strings.Split(trimmed, "|")
		if len(cells) < 3 {
			continue
		}
		// -> Заголовок таблицы: "| Время | Пн | Вт | ... |" — выделяем день недели.
		if strings.Contains(trimmed, "Время") && strings.Contains(cells[1], "ремя") {
			for c := 2; c < len(cells); c++ {
				clean := strings.Trim(strings.ReplaceAll(strings.ReplaceAll(cells[c], "==", ""), "**", ""), " ")
				idx := c - 2
				if idx == curDow {
					cells[c] = " " + weekHourHighlight(clean) + " "
				} else {
					cells[c] = " " + clean + " "
				}
			}
			lines[i] = strings.Join(cells, "|")
			continue
		}
		// -> Строка времени: "| HH:00 | ... |" — выделяем час в первой колонке.
		first := strings.TrimSpace(cells[1])
		clean := strings.ReplaceAll(strings.ReplaceAll(first, "==", ""), "**", "")
		if !strings.Contains(clean, ":") {
			continue // -> не строка времени
		}
		label := strings.TrimSpace(clean)
		if label == curHour {
			cells[1] = " " + weekHourHighlight(label) + " "
		} else {
			cells[1] = " " + label + " "
		}
		lines[i] = strings.Join(cells, "|")
	}
	return strings.Join(lines, "\n")
}

// buildWeekContent возвращает markdown и HTML страницы недели:
// часовая сетка 00:00–23:00 по дням недели. Без выделений — их накладывает
// applyWeekHighlight только на страницу текущей недели.
func buildWeekContent() (string, string) {
	var mdRows, htmlRows []string

	mdRows = append(mdRows, "| Время | Пн | Вт | Ср | Чт | Пт | Сб | Вс |")
	mdRows = append(mdRows, "| :-- | "+strings.Join([]string{":--", ":--", ":--", ":--", ":--", ":--", ":--"}, " | ")+" |")

	htmlRows = append(htmlRows,
		"<tr>"+strings.Join([]string{"<th>Время</th>", "<th>Пн</th>", "<th>Вт</th>", "<th>Ср</th>",
			"<th>Чт</th>", "<th>Пт</th>", "<th>Сб</th>", "<th>Вс</th>"}, "")+"</tr>")

	for hour := 0; hour < 24; hour++ {
		label := fmt.Sprintf("%02d:00", hour)
		mdRows = append(mdRows, "| "+label+" |  |  |  |  |  |  |  |")
		htmlRows = append(htmlRows,
			fmt.Sprintf("<tr><td>%s</td><td></td><td></td><td></td><td></td><td></td><td></td><td></td></tr>", label))
	}

	md := strings.Join(mdRows, "\n") + "\n"
	html := "<table>\n<thead>\n" + strings.Join(htmlRows, "\n") + "\n</thead>\n</table>\n"
	return md, html
}

// updateWeekHighlight обновляет на существующей странице недели выделение
// текущего часа и дня недели: скачивает контент, меняет только обёртки
// ==**...**== вокруг меток в таблице, пользовательские записи сохраняет.
// Возвращает true, если контент изменился и страница обновлена.
func updateWeekHighlight(siteID, path string, page map[string]any, today time.Time) (bool, error) {
	current, _ := page["content"].(string)
	if current == "" && page["content"] == nil {
		if cfg.Verbose {
			fmt.Printf("[%s] %s: исходник недоступен (нет права), пропуск\n", time.Now().Format("15:04:05"), path)
		}
		return false, nil
	}
	updated := applyWeekHighlight(current, today)
	if updated == current {
		return false, nil
	}
	pageID, _ := page["id"].(string)
	if pageID == "" {
		return false, errors.New("страница есть, но без id")
	}
	body := map[string]any{
		"content": updated,
	}
	if _, err := api("PATCH", fmt.Sprintf("/sites/%s/pages/%s", siteID, pageID), body); err != nil {
		return false, fmt.Errorf("PATCH страницы %s: %w", path, err)
	}
	if cfg.Verbose {
		fmt.Printf("[%s] Выделено %s в %s на %s\n", time.Now().Format("15:04:05"), hourLabel(today), weekdays[weekdayIndex(today)], path)
	}
	return true, nil
}

// ensureWeekPages создаёт страницы всех недель ISO-года year (cal-<год>-week-<n>),
// которых ещё нет; для существующих обновляет выделение текущего часа и дня
// только на странице текущей недели, не трогая пользовательский контент.
// Возвращает число созданных.
func ensureWeekPages(siteID string, year int) (int, error) {
	// Неделя 1 ISO-года — та, что содержит 4 января. Начинаем с её понедельника
	// и идём по +7 дней, пока понедельник ещё принадлежит этому ISO-году.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)
	monday := jan4.AddDate(0, 0, -int((jan4.Weekday()+6)%7))
	curISOYear, curISO := time.Now().ISOWeek()
	created := 0
	for {
		isoYear, iso := monday.ISOWeek()
		if isoYear != year {
			break // -> Вышли за пределы ISO-года: недели года кончились.
		}
		path := weekPath(isoYear, iso)
		page, err := getPage(siteID, path, cfg.Locale)
		if err != nil {
			return created, err
		}
		isCurrentWeek := isoYear == curISOYear && iso == curISO
		if page == nil {
			// -> Вновь создаваемые страницы — чистая сетка без выделений;
			//    выделяем сразу только текущую неделю.
			md, html := buildWeekContent()
			if isCurrentWeek {
				md = applyWeekHighlight(md, time.Now())
			}
			body := map[string]any{
				"path":    path,
				"title":   fmt.Sprintf("Неделя %d (%d)", iso, isoYear),
				"editor":  "markdown",
				"content": md,
				"render":  html,
			}
			if cfg.Locale != "" {
				body["locale"] = cfg.Locale
			}
			if _, err := api("POST", fmt.Sprintf("/sites/%s/pages", siteID), body); err != nil {
				return created, fmt.Errorf("POST страницы %s: %w", path, err)
			}
			created++
			if cfg.Verbose {
				fmt.Printf("[%s] Создана страница %s\n", time.Now().Format("15:04:05"), path)
			}
		} else if isCurrentWeek {
			if _, err := updateWeekHighlight(siteID, path, page, time.Now()); err != nil {
				return created, err
			}
		}
		monday = monday.AddDate(0, 0, 7)
	}
	return created, nil
}

// pagePath — путь годового календаря: cal-<год>.
func pagePath(year int) string {
	return fmt.Sprintf("cal-%d", year)
}

// getPage возвращает страницу по пути (через hash) или nil, если её нет.
func getPage(siteID, path, locale string) (map[string]any, error) {
	h := generatePathHash(path, 0)
	qs := "?withContent=true&path=" + url.QueryEscape(path)
	if locale != "" {
		qs += "&locale=" + url.QueryEscape(locale)
	}
	data, err := api("GET", fmt.Sprintf("/sites/%s/pages/%s%s", siteID, h, qs), nil)
	if err != nil {
		var ae *apiError
		if errors.As(err, &ae) && ae.status == 404 {
			return nil, nil
		}
		return nil, fmt.Errorf("GET страницы %s: %w", path, err)
	}
	return data, nil
}

// saveCalendar создаёт или обновляет страницу cal-<year>. Пишет только при изменении контента.
func saveCalendar(md, html string, year int) error {
	siteID, err := getSiteID()
	if err != nil {
		return err
	}
	path := pagePath(year)
	title := fmt.Sprintf("Календарь %d", year)

	page, err := getPage(siteID, path, cfg.Locale)
	if err != nil {
		return err
	}

	if page == nil {
		body := map[string]any{
			"path":    path,
			"title":   title,
			"editor":  "markdown",
			"content": md,
			"render":  html,
		}
		if cfg.Locale != "" {
			body["locale"] = cfg.Locale
		}
		data, err := api("POST", fmt.Sprintf("/sites/%s/pages", siteID), body)
		if err != nil {
			return fmt.Errorf("POST страницы %s: %w", path, err)
		}
		if cfg.Verbose {
			pg, _ := data["page"].(map[string]any)
			fmt.Printf("[%s] Создана страница %s (id=%v)\n", time.Now().Format("15:04:05"), path, pg["id"])
		}
		return nil
	}

	// Изменение только контента: не создавать версию, если ничего не поменялось.
	current, _ := page["content"].(string)
	if current == "" && page["content"] == nil {
		// Сервер не отдал исходник (нет права read:source / write:pages).
		// Не знаем, изменился ли контент, — молчим, чтобы не плодить версии.
		if cfg.Verbose {
			fmt.Printf("[%s] %s: исходник недоступен (нет права), пропуск\n", time.Now().Format("15:04:05"), path)
		}
		return nil
	}
	if current == md {
		return nil
	}

	pageID, _ := page["id"].(string)
	if pageID == "" {
		return errors.New("страница есть, но без id")
	}
	body := map[string]any{
		"content": md,
		"render":  html,
		"title":   title,
	}
	data, err := api("PATCH", fmt.Sprintf("/sites/%s/pages/%s", siteID, pageID), body)
	if err != nil {
		return fmt.Errorf("PATCH страницы %s: %w", path, err)
	}
	if cfg.Verbose {
		_ = data
		fmt.Printf("[%s] Обновлена страница %s\n", time.Now().Format("15:04:05"), path)
	}
	return nil
}

func main() {
	if strings.HasPrefix(cfg.Token, "PASTE_") || cfg.Host == "https://wiki.example.com" {
		fmt.Println("Укажи WIKI_HOST и WIKI_TOKEN в systemd/wiki-calendar.service,")
		fmt.Println("/etc/conf.d/wiki-calendar (OpenRC) или в окружении.")
		os.Exit(1)
	}

	// Первичная проверка подключения и siteId
	siteID, err := getSiteID()
	if err != nil {
		fmt.Printf("Ошибка при старте: %v\n", err)
		os.Exit(1)
	}
	if cfg.Verbose {
		fmt.Printf("Подключено к %s, siteId=%s\n", cfg.Host, siteID)
	}

	lastYear := 0
	for {
		year := time.Now().Year()
		today := time.Now()
		md, html := buildCalendar(year, today)
		if err := saveCalendar(md, html, year); err != nil {
			fmt.Printf("[%s] Ошибка: %v\n", time.Now().Format("15:04:05"), err)
		}
		// -> Страницы недель создаются при старте и при смене года («все сразу»);
		//    существующие не переписываются, поэтому повторный вызов дешёв.
		if _, err := ensureWeekPages(siteID, year); err != nil {
			fmt.Printf("[%s] Ошибка недель: %v\n", time.Now().Format("15:04:05"), err)
		}
		if lastYear != 0 && lastYear != year && cfg.Verbose {
			fmt.Printf("[%s] Новый год! Переключился на %d\n", time.Now().Format("15:04:05"), year)
		}
		lastYear = year
		time.Sleep(time.Duration(cfg.RefreshIntervalS) * time.Second)
	}
}
