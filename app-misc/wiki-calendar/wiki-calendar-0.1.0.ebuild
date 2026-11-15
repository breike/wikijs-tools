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
CONFIG_PROTECT="/etc/conf.d/wiki-calendar"

# No external Go dependencies (stdlib only), so no go-module eclass / vendor needed.
BDEPEND="dev-lang/go"

S="${WORKDIR}/wikijs-tools-master"

src_compile() {
	edo go build -trimpath -o wiki-calendar .
}

src_install() {
	dobin wiki-calendar
	systemd_dounit systemd/wiki-calendar.service
	newinitd "${FILESDIR}"/wiki-calendar.initd wiki-calendar
	newconfd "${FILESDIR}"/wiki-calendar.confd wiki-calendar
	dodoc README.md
}

pkg_postinst() {
	elog "Edit /etc/conf.d/wiki-calendar and set WIKI_HOST / WIKI_TOKEN"
	elog "before starting the service."
	elog
	elog "OpenRC:"
	elog "  rc-update add wiki-calendar default"
	elog "  rc-service wiki-calendar start"
	elog
	elog "systemd:"
	elog "  systemctl edit wiki-calendar"
	elog "  systemctl enable --now wiki-calendar"
}
