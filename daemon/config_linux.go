package daemon

import (
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/pkg/ulimit"
)

// PlatformSpecific defines the configuration of a docker daemon
// that is specific to a particular platform.
type PlatformSpecific struct {
	EnableSelinuxSupport bool
	SocketGroup          string
	Ulimits              map[string]*ulimit.Ulimit
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
	flag.StringVar(&config.Pidfile, []string{"p", "-pidfile"}, "/var/run/docker.pid", "Path to use for daemon PID file")
	flag.StringVar(&config.Root, []string{"g", "-graph"}, "/var/lib/docker", "Root of the Docker runtime")
	flag.BoolVar(&config.PlatformSpecific.EnableSelinuxSupport, []string{"-selinux-enabled"}, false, "Enable selinux support")
	flag.StringVar(&config.PlatformSpecific.SocketGroup, []string{"G", "-group"}, "docker", "Group for the unix socket")
	config.PlatformSpecific.Ulimits = make(map[string]*ulimit.Ulimit)
	opts.UlimitMapVar(config.PlatformSpecific.Ulimits, []string{"-default-ulimit"}, "Set default ulimits for containers")
}
