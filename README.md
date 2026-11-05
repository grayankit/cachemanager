<img src="assets/icon.svg" width="64" height="64" alt="">

# cacheManager

A survey-then-confirm cleaner for an Arch machine that already runs DankMaterialShell. It shows what is using disk space, then deletes only the store you name. The Quickshell page never deletes by itself. It calls this CLI, and a clean button has to be clicked twice.

Zen, Spotify, clipboard history, Downloads, and anything not on the allowlist are reported and left alone.

## Install

```bash
go build -o "$HOME/.local/bin/cachemanager" ./cmd/cachemanager
ln -sfn "$PWD/plugins/cachemanager" "${XDG_CONFIG_HOME:-$HOME/.config}/dms/plugins/cachemanager"
```

Open Settings, Plugins, scan, and enable Cache Manager. The control-center tile id is `plugin_cachemanager`. Reload with:

```bash
dms ipc call plugins reload cachemanager
```

Pacman, the journal, and coredumps need root. Until the helper is installed they ask through a password dialog. To skip the prompt for those three operations only:

```bash
sudo ./scripts/install-priv.sh
```

That installs `/usr/local/libexec/cachemanager-priv` and a sudoers rule that names its exact arguments. It cannot run a shell.

## Commands

```bash
cachemanager disk                 # read-only report
cachemanager disk --json
cachemanager clean pacman         # preview, keep 2 versions
cachemanager clean yay            # preview older tarballs
cachemanager clean all --yes      # pacman, yay, journal, coredumps, allowlisted caches
cachemanager clean gradle --yes   # opt-in, not part of all
```

`clean` without `--yes` only prints the plan. Without a terminal it refuses instead of guessing. `--keep` is 0–3 and defaults to 2. Keep 0 drops every cached pacman package.

Safe cleaners:

| id | what it deletes |
|---|---|
| pacman | `paccache`, keep N installed versions, drop uninstalled |
| yay | older `*.pkg.tar.*` of packages that are still installed |
| journal | vacuum to 300 MiB |
| coredumps | files in `/var/lib/systemd/coredump` older than 7 days |
| caches | `~/.cache/{thumbnails,pip,pnpm,yarn,go-build,uv,vscode-cpptools}` and `~/.npm/_cacache` |

Expensive, each its own command: `gradle`, `pub`, `cargo`, `shaders`, `flatpak`, `orphans`. Pub leaves git checkouts. Cargo leaves `~/.cargo/bin`. Flatpak removes unused runtimes only. Orphans are whatever `pacman -Qdtq` lists, and the names are checked before they are passed to pacman.

Nothing walks `$HOME` looking for `node_modules`. A symlink is removed as a link. It is not followed.
