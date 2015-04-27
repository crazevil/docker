package daemon

import (
	"os"

	flag "github.com/docker/docker/pkg/mflag"
)

// PlatformSpecific defines the configuration of a docker daemon
// that is specific to a particular platform.
type PlatformSpecific struct {
}

// InstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) InstallFlags() {
	// First handle install flags which are consistent cross-platform
	config.CommonInstallFlags()

	// Then platform-specific install flags, or install flags that are present
	// across platforms but are configured differently.
	flag.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, os.Getenv("programdata")+string(os.PathSeparator)+"docker.pid", "Path to use for daemon PID file")
	flag.StringVar(&config.Root, []string{"g", "-graph"}, os.Getenv("programdata")+string(os.PathSeparator)+"docker", "Root of the Docker runtime")
}
