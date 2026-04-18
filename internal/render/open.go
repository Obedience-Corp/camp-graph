package render

import (
	"os/exec"
	"runtime"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
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
		return graphErrors.New("unsupported platform for --open: " + runtime.GOOS)
	}
	return exec.Command(cmd, path).Start()
}
