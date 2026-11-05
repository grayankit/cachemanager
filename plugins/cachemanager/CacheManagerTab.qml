import QtQuick
import qs.Common
import qs.Widgets

Item {
    id: root

    DankFlickable {
        anchors.fill: parent
        anchors.leftMargin: Theme.spacingL
        anchors.rightMargin: Theme.spacingL
        clip: true
        contentWidth: width
        contentHeight: page.implicitHeight + Theme.spacingXL

        Column {
            id: page
            width: Math.min(620, parent.width)
            anchors.horizontalCenter: parent.horizontalCenter
            topPadding: Theme.spacingM
            spacing: Theme.spacingM

            StyledText {
                width: parent.width
                text: "Storage"
                color: Theme.surfaceText
                font.pixelSize: Theme.fontSizeLarge
                font.weight: Font.Medium
            }

            StyledText {
                width: parent.width
                text: "Package caches, journals, and rebuildable files. A clean button asks again before it deletes anything."
                color: Theme.surfaceVariantText
                font.pixelSize: Theme.fontSizeSmall
                wrapMode: Text.WordWrap
            }

            Loader {
                id: viewLoader
                width: parent.width
                source: Qt.resolvedUrl("StorageView.qml")

                onLoaded: {
                    if (item)
                        item.width = Qt.binding(() => viewLoader.width)
                }
            }

            StyledText {
                width: parent.width
                visible: viewLoader.status === Loader.Error
                text: "The storage page failed to load."
                color: Theme.error
                wrapMode: Text.WordWrap
            }
        }
    }
}
