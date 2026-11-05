import QtQuick
import QtCore
import qs.Common
import qs.Services
import qs.Widgets

Item {
    id: root

    property int keep: 2
    property bool busy: false
    property bool cleaning: false
    property string status: "Reading disk…"
    property string logText: ""
    property string armedId: ""
    property int usePercent: 0
    property string diskLabel: ""
    property string safeText: ""
    property string expensiveText: ""
    property string yoursText: ""
    property bool rootHelper: false
    property int scanGen: 0

    readonly property string binPath: StandardPaths.writableLocation(StandardPaths.HomeLocation) + "/.local/bin/cachemanager"

    implicitHeight: column.implicitHeight
    width: parent ? parent.width : 480

    ListModel { id: safeModel }
    ListModel { id: expensiveModel }
    ListModel { id: yoursModel }

    Timer {
        id: armTimer
        interval: 5000
        onTriggered: root.armedId = ""
    }

    Timer {
        id: stallTimer
        interval: 4000
        onTriggered: {
            if (root.safeText === "" && root.diskLabel === "") {
                root.busy = false
                root.status = "Scan did not start"
                root.logText = root.binPath + " did not return. Collapse this plugin and open it again."
            }
        }
    }

    Component.onCompleted: refresh()

    function refresh() {
        if (cleaning)
            return
        if (!binPath || binPath.charAt(0) !== "/") {
            busy = false
            status = "Cannot find cachemanager"
            logText = "Expected ~/.local/bin/cachemanager"
            return
        }
        const gen = ++scanGen
        busy = true
        status = "Reading disk…"
        stallTimer.restart()
        Proc.runCommand("cachemanager.brief", [binPath, "brief", "--json"], (stdout, exitCode) => {
            if (gen !== scanGen || exitCode !== 0)
                return
            stallTimer.stop()
            try {
                const fs = JSON.parse(stdout)
                root.usePercent = fs.usePercent || 0
                root.diskLabel = fs.label || ""
                if (!root.safeText)
                    root.status = fs.label || "Measuring caches…"
            } catch (e) {
            }
        }, 50)
        Proc.runCommand("cachemanager.disk", [binPath, "disk", "--json", "--keep", String(keep)], (stdout, exitCode) => {
            if (gen !== scanGen)
                return
            busy = false
            if (exitCode !== 0) {
                status = "Scan failed"
                logText = stdout || "cachemanager disk failed (" + binPath + ")"
                return
            }
            try {
                applyReport(JSON.parse(stdout))
            } catch (e) {
                status = "Could not read the report"
                logText = stdout
            }
        }, 50)
    }

    function applyReport(report) {
        usePercent = report.filesystem ? report.filesystem.usePercent : 0
        diskLabel = report.filesystem ? report.filesystem.label : ""
        safeText = report.totals ? report.totals.safeText : ""
        expensiveText = report.totals ? report.totals.expensiveText : ""
        yoursText = report.totals ? report.totals.yoursText : ""
        rootHelper = !!report.rootHelper
        stallTimer.stop()
        status = diskLabel || "No report"
        if (report.filesystem && report.filesystem.freeLabel) {
            try {
                PluginService.setGlobalVar("cachemanager", "freeLabel", report.filesystem.freeLabel)
            } catch (e) {
            }
        }
        fill(safeModel, report.stores, "safe")
        fill(expensiveModel, report.stores, "expensive")
        fill(yoursModel, report.stores, "yours")
    }

    function fill(model, stores, kind) {
        model.clear()
        if (!stores)
            return
        for (let i = 0; i < stores.length; i++) {
            const s = stores[i]
            if (s.kind !== kind)
                continue
            model.append({
                storeId: s.id,
                label: s.label || "",
                detail: s.detail || "",
                sizeText: s.sizeText || "",
                reclaimText: s.reclaimText || "",
                canClean: !!s.canClean
            })
        }
    }

    function arm(storeId) {
        if (busy || !storeId)
            return
        if (armedId === storeId) {
            armedId = ""
            armTimer.stop()
            runClean(storeId)
            return
        }
        armedId = storeId
        armTimer.restart()
    }

    function runClean(storeId) {
        cleaning = true
        busy = true
        status = "Cleaning " + storeId + "…"
        logText = ""
        const args = [binPath, "clean", storeId, "--yes", "--json"]
        if (storeId === "pacman" || storeId === "all") {
            args.push("--keep")
            args.push(String(keep))
        }
        Proc.runCommand("cachemanager.clean." + storeId, args, (stdout, exitCode) => {
            cleaning = false
            busy = false
            logText = formatClean(stdout, exitCode)
            refresh()
        }, 50)
    }

    function formatClean(stdout, exitCode) {
        try {
            const result = JSON.parse(stdout)
            if (result.error)
                return result.error + (result.log ? "\n" + result.log : "")
            if (result.log)
                return result.log
            if (result.detail)
                return (result.applied ? "Done. " : "") + result.detail
        } catch (e) {
        }
        return stdout || (exitCode !== 0 ? "clean failed" : "")
    }

    component StoreList: Column {
        property var entries
        property var view
        width: parent.width
        spacing: Theme.spacingS

        Repeater {
            model: entries
            Rectangle {
                required property string storeId
                required property string label
                required property string detail
                required property string sizeText
                required property string reclaimText
                required property bool canClean

                width: parent.width
                implicitHeight: row.implicitHeight + Theme.spacingS * 2
                height: implicitHeight
                radius: Theme.cornerRadius
                color: Theme.floatingWindowNestedSurface
                border.color: Theme.outlineMedium
                border.width: Theme.layerOutlineWidth

                Row {
                    id: row
                    width: parent.width - Theme.spacingM * 2
                    anchors.left: parent.left
                    anchors.leftMargin: Theme.spacingM
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Theme.spacingS

                    Column {
                        width: parent.width - (canClean ? 96 : 0)
                        spacing: 2
                        StyledText {
                            width: parent.width
                            text: label
                            color: Theme.surfaceText
                            elide: Text.ElideRight
                        }
                        StyledText {
                            width: parent.width
                            text: sizeText + (reclaimText && reclaimText !== sizeText ? " · " + reclaimText : "")
                            color: Theme.surfaceVariantText
                            font.pixelSize: Theme.fontSizeSmall
                            elide: Text.ElideRight
                        }
                        StyledText {
                            width: parent.width
                            text: detail
                            color: Theme.surfaceVariantText
                            font.pixelSize: Theme.fontSizeSmall
                            wrapMode: Text.WordWrap
                        }
                    }

                    Rectangle {
                        visible: canClean
                        width: 88
                        height: 32
                        radius: Theme.cornerRadius
                        color: view && view.armedId === storeId ? Theme.primary : Theme.surfaceContainerHighest
                        border.color: view && view.armedId === storeId ? Theme.primary : Theme.outlineMedium
                        border.width: Theme.layerOutlineWidth
                        anchors.verticalCenter: parent.verticalCenter
                        StyledText {
                            anchors.centerIn: parent
                            text: view && view.armedId === storeId ? "Confirm" : "Clean"
                            color: view && view.armedId === storeId ? Theme.primaryText : Theme.surfaceText
                            font.pixelSize: Theme.fontSizeSmall
                        }
                        MouseArea {
                            anchors.fill: parent
                            enabled: view && !view.busy
                            cursorShape: Qt.PointingHandCursor
                            onClicked: view.arm(storeId)
                        }
                    }
                }
            }
        }
    }

    Column {
        id: column
        width: parent.width
        spacing: Theme.spacingM

        StyledText {
            width: parent.width
            text: root.status
            color: Theme.surfaceText
            font.pixelSize: Theme.fontSizeLarge
            wrapMode: Text.WordWrap
        }

        Rectangle {
            width: parent.width
            height: 8
            radius: 4
            color: Theme.surfaceContainerHighest

            Rectangle {
                width: Math.max(0, Math.min(parent.width, parent.width * root.usePercent / 100))
                height: parent.height
                radius: 4
                color: Theme.primary
            }
        }

        Row {
            width: parent.width
            spacing: Theme.spacingS

            Repeater {
                model: [
                    { title: "Safe", value: root.safeText },
                    { title: "Expensive", value: root.expensiveText },
                    { title: "Left alone", value: root.yoursText }
                ]
                Rectangle {
                    required property var modelData
                    width: (column.width - Theme.spacingS * 2) / 3
                    height: 52
                    radius: Theme.cornerRadius
                    color: Theme.surfaceContainerHighest
                    border.color: Theme.outlineMedium
                    border.width: Theme.layerOutlineWidth

                    Column {
                        anchors.centerIn: parent
                        spacing: 2
                        StyledText {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: modelData.title
                            color: Theme.surfaceVariantText
                            font.pixelSize: Theme.fontSizeSmall
                        }
                        StyledText {
                            anchors.horizontalCenter: parent.horizontalCenter
                            text: modelData.value || "—"
                            color: Theme.surfaceText
                        }
                    }
                }
            }
        }

        Row {
            spacing: Theme.spacingS

            StyledText {
                text: "Pacman keep"
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                anchors.verticalCenter: parent.verticalCenter
            }

            Repeater {
                model: [1, 2, 3]
                Rectangle {
                    required property var modelData
                    width: 28
                    height: 28
                    radius: 14
                    color: root.keep === modelData ? Theme.primary : Theme.surfaceContainerHighest
                    border.color: root.keep === modelData ? Theme.primary : Theme.outlineMedium
                    border.width: Theme.layerOutlineWidth
                    StyledText {
                        anchors.centerIn: parent
                        text: modelData
                        color: root.keep === modelData ? Theme.primaryText : Theme.surfaceText
                        font.pixelSize: Theme.fontSizeSmall
                    }
                    MouseArea {
                        anchors.fill: parent
                        enabled: !root.busy
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            root.keep = modelData
                            root.refresh()
                        }
                    }
                }
            }

            Rectangle {
                width: 88
                height: 28
                radius: Theme.cornerRadius
                color: Theme.surfaceContainerHighest
                    border.color: Theme.outlineMedium
                    border.width: Theme.layerOutlineWidth
                StyledText {
                    anchors.centerIn: parent
                    text: "Refresh"
                    color: Theme.surfaceText
                    font.pixelSize: Theme.fontSizeSmall
                }
                MouseArea {
                    anchors.fill: parent
                    enabled: !root.busy
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.refresh()
                }
            }
        }

        StyledText {
            width: parent.width
            visible: !root.rootHelper
            text: "Pacman, the journal, and coredumps ask for your password until you run sudo ./scripts/install-priv.sh"
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
            wrapMode: Text.WordWrap
        }

        Item {
            width: parent.width
            height: 28
            StyledText {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: "Safe"
                color: Theme.primary
                font.pixelSize: Theme.fontSizeMedium
                font.weight: Font.Medium
            }
            Rectangle {
                anchors.right: parent.right
                width: 96
                height: 28
                radius: Theme.cornerRadius
                color: root.armedId === "all" ? Theme.primary : Theme.surfaceContainerHighest
                border.color: root.armedId === "all" ? Theme.primary : Theme.outlineMedium
                border.width: Theme.layerOutlineWidth
                StyledText {
                    anchors.centerIn: parent
                    text: root.armedId === "all" ? "Confirm" : "Clean safe"
                    color: root.armedId === "all" ? Theme.primaryText : Theme.surfaceText
                    font.pixelSize: Theme.fontSizeSmall
                }
                MouseArea {
                    anchors.fill: parent
                    enabled: !root.busy
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.arm("all")
                }
            }
        }

        StoreList { entries: safeModel; view: root }

        StyledText {
            text: "Expensive"
            color: Theme.primary
            font.pixelSize: Theme.fontSizeMedium
            font.weight: Font.Medium
        }
        StyledText {
            width: parent.width
            text: "Not included in Clean safe. A second click deletes."
            color: Theme.surfaceVariantText
            font.pixelSize: Theme.fontSizeSmall
            wrapMode: Text.WordWrap
        }
        StoreList { entries: expensiveModel; view: root }

        StyledText {
            text: "Left alone"
            color: Theme.primary
            font.pixelSize: Theme.fontSizeMedium
            font.weight: Font.Medium
        }
        StoreList { entries: yoursModel; view: root }

        StyledText {
            width: parent.width
            visible: root.logText.length > 0
            text: root.logText
            color: Theme.surfaceText
            font.pixelSize: Theme.fontSizeSmall
            wrapMode: Text.WordWrap
        }
    }
}
