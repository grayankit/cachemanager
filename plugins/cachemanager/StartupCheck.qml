import QtQuick
import qs.Common

QtObject {
    function check(done) {
        Proc.runCommand("cachemanager.dep", ["bash", "-c", "command -v cachemanager >/dev/null || test -x \"$HOME/.local/bin/cachemanager\""], (stdout, exitCode) => {
            if (exitCode === 0) {
                done(null)
                return
            }
            done({
                title: "cachemanager is not installed",
                details: "The binary was not found on PATH or in ~/.local/bin. Build it, then enable this plugin again."
            })
        })
    }
}
