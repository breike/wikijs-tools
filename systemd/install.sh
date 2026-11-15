#!/usr/bin/env bash
# Установка wiki-calendar как systemd-сервиса.
#
# Работает под root (или sudo):
#   sudo bash systemd/install.sh
#
# Что делает:
#   1) собирает бинарник wiki-calendar и кладёт в /usr/bin/wiki-calendar
#   2) копирует юнит в /etc/systemd/system/wiki-calendar.service
#   3) создаёт /etc/wiki-calendar.env с дефолтными переменными (существующий
#      конфиг НЕ перезаписывается — свои значения сохранятся)
#   4) включает и запускает сервис
#
# ПАРАМЕТРЫ задаются в /etc/wiki-calendar.env (WIKI_*), не в юните.
set -euo pipefail

BIN_NAME="wiki-calendar"
BIN_DEST="/usr/bin/${BIN_NAME}"
UNIT_SRC="$(dirname "$0")/wiki-calendar.service"
UNIT_DEST="/etc/systemd/system/${BIN_NAME}.service"
ENV_SRC="$(dirname "$0")/wiki-calendar.env"
ENV_DEST="/etc/${BIN_NAME}.env"

if [[ "$(id -u)" -ne 0 ]]; then
    echo "Ошибка: запусти с sudo (нужны права root для /usr/bin и /etc/systemd)." >&2
    exit 1
fi

# --- Сборка ---
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_DIR"
echo ">> Сборка ${BIN_NAME}..."
go build -trimpath -o "${BIN_NAME}" .

# --- Установка бинарника ---
install -m 0755 "${PROJECT_DIR}/${BIN_NAME}" "${BIN_DEST}"
rm -f "${PROJECT_DIR}/${BIN_NAME}"
echo ">> Бинарник: ${BIN_DEST}"

# --- Юнит ---
install -m 0644 "${UNIT_SRC}" "${UNIT_DEST}"
echo ">> Юнит: ${UNIT_DEST}"

# --- Конфиг (не затираем существующий!) ---
if [[ -e "${ENV_DEST}" ]]; then
    echo ">> ${ENV_DEST} уже есть — не трогаем (отредактируй его под себя)."
else
    install -m 0644 "${ENV_SRC}" "${ENV_DEST}"
    echo ">> Создан ${ENV_DEST} — отредактируй WIKI_HOST / WIKI_TOKEN и перезапусти сервис."
fi

systemctl daemon-reload
systemctl enable "${BIN_NAME}.service"
echo ">> Сервис включён в автозапуск."

systemctl restart "${BIN_NAME}.service"
echo ">> Сервис перезапущен."

systemctl status --no-pager "${BIN_NAME}.service" || true