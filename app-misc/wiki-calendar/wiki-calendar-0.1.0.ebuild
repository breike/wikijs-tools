# Copyright 2026 breike
# Distributed under the terms of the MIT license

EAPI=8

inherit edo systemd

DESCRIPTION="Publish a yearly calendar with per-week hour tables to Wiki.js (cal-<year>, cal-<year>-week-<week>)"
HOMEPAGE="https://github.com/breike/wikijs-tools"
# master-branch snapshot; versioned tarballs are not cut upstream yet.
# sha256sums are pinned in the Manifest after ebuild maintenance.
SRC_URI="https://github.com/breike/wikijs-tools/archive/refs/heads/master.tar.gz -> ${P}-master.tar.gz"

LICENSE="MIT"
SLOT="0"
KEYWORDS="~amd64"
# Оба файла конфигурации защищены от перезаписи при апгрейде:
# OpenRC использует /etc/conf.d/wiki-calendar, systemd — /etc/wiki-calendar.env.
CONFIG_PROTECT="/etc/conf.d/wiki-calendar /etc/wiki-calendar.env"

# No external Go dependencies (stdlib only), so no go-module eclass / vendor needed.
BDEPEND="dev-lang/go"

S="${WORKDIR}/wikijs-tools-master"

src_compile() {
	edo go build -trimpath -o wiki-calendar .
}

src_install() {
	dobin wiki-calendar
	systemd_dounit systemd/wiki-calendar.service
	# Файл переменных для systemd-юнита (EnvironmentFile); на OpenRC его
	# не используем — там /etc/conf.d/wiki-calendar.
	insinto /etc
	newins systemd/wiki-calendar.env wiki-calendar.env
	newinitd "${FILESDIR}"/wiki-calendar.initd wiki-calendar
	newconfd "${FILESDIR}"/wiki-calendar.confd wiki-calendar
	insinto /usr/share/wiki-calendar/org
	newins org/calendar.org.example calendar.org.example
	dodoc README.md
}

pkg_postinst() {
	elog "Configure the token/host, then start the service:"
	elog
	elog "  OpenRC:  edit /etc/conf.d/wiki-calendar,"
	elog "           rc-update add wiki-calendar default && rc-service wiki-calendar start"
	elog
	elog "  systemd: edit /etc/wiki-calendar.env (EnvironmentFile),"
	elog "           systemctl enable --now wiki-calendar"
	elog
	elog "Both config files are CONFIG_PROTECT'ed and survive upgrades."
}
