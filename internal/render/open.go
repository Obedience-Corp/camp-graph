package render

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenFile opens a file with the system default application.
func OpenFile(path string) error {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	case "windows":
		cmd = "start"
	default:
		return fmt.Errorf("unsupported platform for --open: %s", runtime.GOOS)
	}
	return exec.Command(cmd, path).Start()
}
