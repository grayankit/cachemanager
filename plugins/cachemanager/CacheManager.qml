import QtQuick
import Quickshell
import qs.Common
import qs.Services
import qs.Widgets
import qs.Modules.Plugins

PluginComponent {
    id: root

    PluginGlobalVar {
        id: freeVar
        varName: "freeLabel"
        defaultValue: "Storage"
    }

    readonly property string binPath: Quickshell.env("HOME") + "/.local/bin/cachemanager"

    ccWidgetIcon: "cleaning_services"
    ccWidgetPrimaryText: "Storage"
    ccWidgetSecondaryText: freeVar.value || "Storage"
    ccWidgetIsToggle: false
    ccWidgetIsActive: false

    onCcWidgetExpanded: brief()

    Component.onCompleted: brief()

    function brief() {
        Proc.runCommand("cachemanager.brief", [binPath, "brief", "--json"], (stdout, exitCode) => {
            if (exitCode !== 0)
                return
            try {
                const report = JSON.parse(stdout)
                if (report.freeLabel)
                    PluginService.setGlobalVar("cachemanager", "freeLabel", report.freeLabel)
            } catch (e) {
            }
        })
    }

    ccDetailContent: Component {
        StorageView {
            width: parent.width
        }
    }

    horizontalBarPill: Component {
        Row {
            spacing: Theme.spacingXS

            DankIcon {
                name: "cleaning_services"
                color: Theme.widgetIconColor
                size: Theme.barIconSize(root.barThickness, -4)
                anchors.verticalCenter: parent.verticalCenter
            }

            StyledText {
                text: freeVar.value || "Storage"
                color: Theme.widgetTextColor
                font.pixelSize: Theme.barTextSize(root.barThickness, root.barConfig?.fontScale)
                anchors.verticalCenter: parent.verticalCenter
            }
        }
    }

    verticalBarPill: Component {
        DankIcon {
            name: "cleaning_services"
            color: Theme.widgetIconColor
            size: Theme.barIconSize(root.barThickness, -4)
        }
    }
}
