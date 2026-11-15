# Maintainer: Rusty <rusty@localhost>

pkgname=wiki-calendar
pkgver=0.1.0
pkgrel=1
pkgdesc="Publish a yearly calendar with per-week hour tables to Wiki.js (cal-<year>, cal-<year>-week-<week>)"
arch=('x86_64')
url="https://github.com/breike/wikijs-tools"
license=('MIT')
depends=()
makedepends=('go')
install=wiki-calendar.install

# Source comes from the git repo's default branch. The default branch
# (main) is a moving target, so the checksum is intentionally SKIP —
# same approach as thinkpad_remapper.
source=("${url}/archive/refs/heads/main.tar.gz")
sha256sums=('SKIP')

build() {
    cd "${srcdir}/wikijs-tools-main"
    go build -trimpath -o wiki-calendar .
}

check() {
    cd "${srcdir}/wikijs-tools-main"
    go vet ./...
}

package() {
    cd "${srcdir}/wikijs-tools-main"

    # Binary
    install -Dm755 wiki-calendar "${pkgdir}/usr/bin/wiki-calendar"

    # Systemd unit
    install -Dm644 systemd/wiki-calendar.service \
        "${pkgdir}/usr/lib/systemd/system/wiki-calendar.service"
}