package debug

import (
	"os"
)

var debugHandler chan os.Signal

func SetupRuntimeDebugSignalHandler(outputDir string) {
	installSignalHandler(outputDir)
}
