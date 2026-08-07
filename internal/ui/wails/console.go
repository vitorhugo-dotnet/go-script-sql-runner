package wailsui

func shouldHideOwnConsole(attachedProcessCount uintptr) bool {
	return attachedProcessCount == 1
}
