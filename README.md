# Wiki.js Calendar Publisher

Publishes a yearly calendar to [Wiki.js](https://js.wiki) and keeps it up to date —
with a dedicated page per week that has an hourly grid, ready to plan your time in.

The calendar is written as a single wiki page `cal-<year>` (e.g. `cal-2026`) containing
all 12 months as markdown tables. Each week number in those tables links to its own
page `cal-<year>-week-<week>` (e.g. `cal-2026-week-44`), which contains a per-hour
grid `00:00`–`23:00` across Monday–Sunday.

## Features

- **Cal-<year> page** — all 12 months, one table per month (`Week | Mon | Tue | ... | Sun`),
  weeks start on Monday, week numbers are ISO-8601.
- **Today is highlighted** — `==**15**==` in markdown, `<mark><strong>15</strong></mark>` in HTML.
- **Per-week pages** — `cal-<year>-week-<week>` with an hourly grid (`00:00`–`23:00`),
  created automatically for all weeks of the year (52–53).
- **Current hour and day are highlighted** — on the page of the *current* week the
  hour row is marked with `==**HH:00**==` (markdown) /
  `<mark><strong>HH:00</strong></mark>` (HTML), and the header cell of the current
  weekday is marked the same way (`==**Пн**==` … `==**Вс**==`). Other weeks stay plain.
  The highlight follows the clock: at every poll the service fetches the current week's
  page, moves both markers and leaves your notes in the table untouched.
- **Notes are preserved** — if you have edited a week page (added tasks, meetings, etc.),
  the service only toggles the `==**...**==` markers on the *time column* and the *weekday
  header*; everything else in the content is left exactly as you wrote it. Nothing is
  overwritten.
- **Links in the week column** — every week number in the yearly calendar links to its
  week page. The year in the link is the *ISO year*, so boundary weeks (e.g. the first
  January week that belongs to the previous ISO year) point at the right page.
- **Idempotent** — only writes when the *content* actually changed (when "today" moves
  or the highlight hour moves), so it does not spam page history with a new version
  every minute.
- **Auto-rollover** — when the year changes, it starts maintaining `cal-<new-year>` and
  stops touching the old one automatically.
- **Works in the background** — polls every `REFRESH_INTERVAL_S` seconds (60 by default),
  logs action to stdout/journald.

## Configuration

All settings are read from the environment (see the systemd unit):

| Variable | Default | Description |
| --- | --- | --- |
| `WIKI_HOST` | `https://wiki.example.com` | Wiki.js base URL, no trailing slash. |
| `WIKI_TOKEN` | — | Bearer token: system API key or personal access token. |
| `WIKI_REFRESH_INTERVAL_S` | `60` | Poll interval in seconds. |
| `WIKI_SITE_ID` | *(empty)* | Site UUID; empty = auto-detected via `GET /_api/bootstrap`. |
| `WIKI_LOCALE` | *(empty)* | Page locale; empty = site's default locale. |
| `WIKI_VERBOSE` | `1` | Log each write / skip to stdout. |

## API

The tool uses the Wiki.js 3.x REST API under `/_api`:

- `GET  /_api/bootstrap` — resolve the site ID.
- `GET  /_api/sites/{siteId}/pages/{pathHash}?withContent=true&path=...` — read a page
  (pages are addressed by a hash of their path, `generatePathHash`).
- `POST /_api/sites/{siteId}/pages` — create a page.
- `PATCH /_api/sites/{siteId}/pages/{pageId}` — update a page (only when content changed).

Authentication: `Authorization: Bearer <token>` (system API key or personal access token).
A personal access token needs the `pages` scope; reading the existing content requires the
`read:source` / `write:pages` rule on the page — otherwise the tool skips rather than
spam page history.

## Install on Arch Linux

### 1. Build and install the package

```sh
git clone https://github.com/breike/wikijs-tools
cd wikijs-tools
makepkg -si
```

> The PKGBUILD pulls the sources from the `master` branch of
> `https://github.com/breike/wikijs-tools` (`sha256sums=('SKIP')`, same approach as
> the project's other packages). Make sure the repository is pushed.

This installs:

- `/usr/bin/wiki-calendar` — the binary
- `/usr/lib/systemd/system/wiki-calendar.service` — the systemd unit (if systemd is used)
- `/etc/init.d/wiki-calendar` + `/etc/conf.d/wiki-calendar` — the OpenRC init script
  and its configuration (on Gentoo, see below)

### 2. Configure

Edit the unit's environment (variables above):

```sh
sudo systemctl edit wiki-calendar
```

```ini
[Service]
Environment=WIKI_HOST=https://wiki.example.com
Environment=WIKI_TOKEN=PASTE_YOUR_TOKEN_HERE
```

Or edit `/usr/lib/systemd/system/wiki-calendar.service` directly (unit file is
tracked-simple; an override via `systemctl edit` survives package upgrades).

### 3. Enable and start

The `.install` hook already enables and starts the service. To control it manually:

```sh
sudo systemctl enable --now wiki-calendar
systemctl status wiki-calendar          # check it is running
journalctl -u wiki-calendar -f          # follow the log
```

### 4. Verify

Open `https://<wiki>/cal-<year>` — you should see the current year's calendar with
"today" highlighted and clickable week numbers. Clicking a week number opens
`cal-<year>-week-<week>` with the hourly grid.

After a year change the service automatically starts maintaining the new `cal-<year>`
page; old pages are left untouched.

## Install on Gentoo Linux

The repository includes an ebuild for an overlay:

```sh
# clone/update the overlay, e.g.:
#   eselect repository enable wiki-calendar   (or add it to repos.conf)
# then:
emerge -av app-misc/wiki-calendar
```

The ebuild installs:

- `/usr/bin/wiki-calendar` — the binary
- `/etc/init.d/wiki-calendar` — the OpenRC init script
- `/etc/conf.d/wiki-calendar` — configuration (the `WIKI_*` variables from the table above)
- `/usr/lib/systemd/system/wiki-calendar.service` — the systemd unit as well

### OpenRC (default on Gentoo)

Edit `/etc/conf.d/wiki-calendar` and set the `WIKI_HOST` / `WIKI_TOKEN` variables, then:

```sh
rc-update add wiki-calendar default
rc-service wiki-calendar start
rc-service wiki-calendar status      # check it is running
tail -f /var/log/messages            # follow the log
```

The init script supervises the daemon with `supervise-daemon` and restarts it if it
crashes. The file `/etc/conf.d/wiki-calendar` is `CONFIG_PROTECT`ed, so your edits
survive package upgrades.

### systemd (optional)

The same variables live in the unit file — use `systemctl edit wiki-calendar`
or edit `/usr/lib/systemd/system/wiki-calendar.service` directly:

```sh
systemctl enable --now wiki-calendar
systemctl status wiki-calendar       # check it is running
journalctl -u wiki-calendar -f       # follow the log
```

## Manual run (without systemd)

```sh
WIKI_HOST=https://wiki.example.com WIKI_TOKEN=... go run .          # dev
WIKI_HOST=https://wiki.example.com WIKI_TOKEN=... ./wiki-calendar   # built binary
```

The tool loops forever; run it under `nohup`, a terminal multiplexer, or a supervisor
of your choice. `systemd/install.sh` automates building to `/usr/bin` + systemd unit
install (for OpenRC, see the Gentoo section).

## Development

```sh
go build ./...    # build
go vet ./...      # static checks
go test ./...     # tests (hash vectors, calendar generation)
```

## License

MIT.