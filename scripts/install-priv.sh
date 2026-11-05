#!/bin/bash
# Install the root helper and a sudoers rule that allows only that helper.
set -euo pipefail

if [[ ${EUID} -ne 0 ]]; then
  exec sudo -- "$0" "$@"
fi

user="${SUDO_USER:-}"
if [[ -z "$user" || "$user" == root ]]; then
  echo "run this as your user: sudo $0" >&2
  exit 1
fi

src="$(cd "$(dirname "$0")" && pwd)/cachemanager-priv"
install -d -m 755 /usr/local/libexec
install -m 755 "$src" /usr/local/libexec/cachemanager-priv

tmp="$(mktemp)"
cat >"$tmp" <<EOF
# cachemanager: passwordless access only to the fixed helper.
$user ALL=(root) NOPASSWD: /usr/local/libexec/cachemanager-priv pacman 0, /usr/local/libexec/cachemanager-priv pacman 1, /usr/local/libexec/cachemanager-priv pacman 2, /usr/local/libexec/cachemanager-priv pacman 3, /usr/local/libexec/cachemanager-priv journal, /usr/local/libexec/cachemanager-priv coredumps
EOF
visudo -cf "$tmp"
install -m 440 "$tmp" /etc/sudoers.d/cachemanager
rm -f "$tmp"
echo "installed /usr/local/libexec/cachemanager-priv and /etc/sudoers.d/cachemanager"
