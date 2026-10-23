package app

import "runtime"

const isDarwin = runtime.GOOS == "darwin"

// isWindows 标记当前平台是否为 Windows
const isWindows = runtime.GOOS == "windows"

func pasteAccel() string {
	if isDarwin {
		return "Super+V"
	}
	return "Ctrl+V"
}
