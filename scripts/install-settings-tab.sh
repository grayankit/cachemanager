#!/bin/bash
# Re-add Storage under System after a DankMaterialShell update overwrites the shell files.
set -euo pipefail

shell="${DMS_SHELL:-$HOME/.local/share/dms-shell}"
tabs="$shell/Common/SettingsTabs.qml"
content="$shell/Modals/Settings/SettingsContent.qml"
page="${XDG_CONFIG_HOME:-$HOME/.config}/dms/plugins/cachemanager/CacheManagerTab.qml"

if [[ ! -f "$tabs" || ! -f "$content" ]]; then
  echo "dms-shell not found at $shell" >&2
  exit 1
fi
if [[ ! -f "$page" ]]; then
  echo "missing $page" >&2
  exit 1
fi

if ! grep -q '"id": "storage"' "$tabs"; then
  python3 - "$tabs" <<'PY'
import sys
path = sys.argv[1]
text = open(path).read()
needle = '''            "children": [
                {
                    "id": "audio",'''
entry = '''            "children": [
                {
                    "id": "storage",
                    "text": I18n.tr("Storage"),
                    "icon": "cleaning_services",
                    "tabIndex": 48
                },
                {
                    "id": "audio",'''
if needle not in text:
    raise SystemExit("could not find the System section in SettingsTabs.qml")
open(path, "w").write(text.replace(needle, entry, 1))
PY
  echo "added Storage to the System section"
else
  echo "Storage tab already registered"
fi

if ! grep -q 'id: storageLoader' "$content"; then
  python3 - "$content" "$page" <<'PY'
import sys
path, page = sys.argv[1], sys.argv[2]
text = open(path).read()
marker = "            sourceComponent: KeyboardTab {}\n"
if marker not in text:
    raise SystemExit("could not find the keyboard loader in SettingsContent.qml")
loader = f'''            sourceComponent: KeyboardTab {{}}

        Loader {{
            id: storageLoader
            anchors.fill: parent
            active: root.currentIndex === 48
            visible: active
            focus: active
            source: "{page}"

            onActiveChanged: {{
                if (active && item)
                    Qt.callLater(() => item.forceActiveFocus());
            }}
        }}
'''
# The marker appears inside the keyboard loader. Insert the new loader after that whole loader.
end = text.find(marker)
if end < 0:
    raise SystemExit("keyboard loader missing")
# Find the closing of the keyboard loader that follows the marker.
close = text.find("        }\n    }\n}", end)
if close < 0:
    raise SystemExit("could not find the end of SettingsContent.qml")
insert = f'''
        Loader {{
            id: storageLoader
            anchors.fill: parent
            active: root.currentIndex === 48
            visible: active
            focus: active
            source: "{page}"

            onActiveChanged: {{
                if (active && item)
                    Qt.callLater(() => item.forceActiveFocus());
            }}
        }}
'''
open(path, "w").write(text[:close] + insert + text[close:])
PY
  echo "added the Storage page loader"
else
  echo "Storage loader already present"
fi

echo "restart the shell: dms restart"
