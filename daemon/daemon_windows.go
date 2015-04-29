//  +build windows

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/docker/api"
	"github.com/docker/docker/daemon/execdriver/execdrivers"
	"github.com/docker/docker/daemon/graphdriver"
	_ "github.com/docker/docker/daemon/graphdriver/windows"
	_ "github.com/docker/docker/daemon/graphdriver/windummy"
	"github.com/docker/docker/engine"
	"github.com/docker/docker/graph"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/truncindex"
	_ "github.com/docker/docker/pkg/winresources"
	"github.com/docker/docker/runconfig"
	"github.com/docker/docker/trust"
	"github.com/docker/docker/utils"
	"github.com/docker/docker/volumes"
	"github.com/docker/libcontainer/label"
)

// TODO Windows This can probably be factored out better
func KillIfLxc(ID string) {
	// No-op on Windows as Lxc execution driver is not used.
}

func (daemon *Daemon) createRootfs(container *Container) error {
	// Step 1: create the container directory.
	// This doubles as a barrier to avoid race conditions.
	if err := os.Mkdir(container.root, 0700); err != nil {
		return err
	}
	if err := daemon.driver.Create(container.ID, container.ImageID); err != nil {
		return err
	}
	return nil
}

func (daemon *Daemon) Changes(container *Container) ([]archive.Change, error) {
	return daemon.driver.Changes(container.ID, container.ImageID)
}

func (daemon *Daemon) Diff(container *Container) (archive.Archive, error) {
	return daemon.driver.Diff(container.ID, container.ImageID)
}

func NewDaemonFromDirectory(config *Config, eng *engine.Engine) (*Daemon, error) {

	var dwVersion uint32

	dwVersion, err := syscall.GetVersion()
	if err != nil {
		return nil, fmt.Errorf("Failed to call GetVersion()")
	}

	if int(dwVersion&0xFF) < 10 {
		return nil, fmt.Errorf("Incorrect Windows Version. Is the manifest present?")
	}

	if config.Mtu == 0 {
		config.Mtu = getDefaultNetworkMtu()
	}

	config.DisableNetwork = config.BridgeIface == disableNetworkBridge

	// Claim the pidfile first, to avoid any and all unexpected race conditions.
	// Some of the init doesn't need a pidfile lock - but let's not try to be smart.
	if config.Pidfile != "" {
		if err := utils.CreatePidFile(config.Pidfile); err != nil {
			return nil, err
		}
		eng.OnShutdown(func() {
			// Always release the pidfile last, just in case
			utils.RemovePidFile(config.Pidfile)
		})
	}

	// Check that the system is supported and we have sufficient privileges
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("The Docker daemon is only supported on Linux and Windows")
	}

	// set up the TempDir to use a canonical path
	tmp, err := utils.TempDir(config.Root)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the TempDir under %s: %s", config.Root, err)
	}
	realTmp, err := utils.ReadSymlinkedDirectory(tmp)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	os.Setenv("TMPDIR", realTmp)

	// get the canonical path to the Docker root directory
	var realRoot string
	if _, err := os.Stat(config.Root); err != nil && os.IsNotExist(err) {
		realRoot = config.Root
	} else {
		realRoot, err = utils.ReadSymlinkedDirectory(config.Root)
		if err != nil {
			return nil, fmt.Errorf("Unable to get the full path to root (%s): %s", config.Root, err)
		}
	}
	config.Root = realRoot
	// Create the root directory if it doesn't exists
	if err := system.MkdirAll(config.Root, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	// Set the default driver
	graphdriver.DefaultDriver = config.GraphDriver

	// Load storage driver
	driver, err := graphdriver.New(config.Root, config.GraphOptions)
	if err != nil {
		return nil, err
	}
	log.Debugf("Using graph driver %s", driver)

	daemonRepo := filepath.Join(config.Root, "containers")

	if err := system.MkdirAll(daemonRepo, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}

	log.Debugf("Creating images graph")
	g, err := graph.NewGraph(filepath.Join(config.Root, "graph"), driver)
	if err != nil {
		return nil, err
	}

	// Use the same graph driver from storage for volumes.
	volumes, err := volumes.NewRepository(filepath.Join(config.Root, "volumes"), driver)
	if err != nil {
		return nil, err
	}

	trustKey, err := api.LoadOrCreateTrustKey(config.TrustKeyPath)
	if err != nil {
		return nil, err
	}

	log.Debugf("Creating repository list")
	repositories, err := graph.NewTagStore(filepath.Join(config.Root, "repositories-"+driver.String()), g, trustKey)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create Tag store: %s", err)
	}

	trustDir := filepath.Join(config.Root, "trust")
	if err := system.MkdirAll(trustDir, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	t, err := trust.NewTrustStore(trustDir)
	if err != nil {
		return nil, fmt.Errorf("could not create trust store: %s", err)
	}

	if !config.DisableNetwork {
		job := eng.Job("init_networkdriver")

		job.Setenv("BridgeIface", config.BridgeIface)

		if err := job.Run(); err != nil {
			return nil, err
		}
	}

	graphdbPath := filepath.Join(config.Root, "linkgraph.db")
	graph, err := graphdb.NewSqliteConn(graphdbPath)
	if err != nil {
		return nil, err
	}

	// TODO Windows. This is temporary.
	sysInitPath := os.Getenv("TEMP")

	sysInfo := sysinfo.New(false)
	ed, err := execdrivers.NewDriver(config.ExecDriver, config.Root, sysInitPath, sysInfo)
	if err != nil {
		return nil, err
	}

	daemon := &Daemon{
		ID:             trustKey.PublicKey().KeyID(),
		repository:     daemonRepo,
		containers:     &contStore{s: make(map[string]*Container)},
		execCommands:   newExecStore(),
		graph:          g,
		repositories:   repositories,
		idIndex:        truncindex.NewTruncIndex([]string{}),
		sysInfo:        sysInfo,
		volumes:        volumes,
		config:         config,
		containerGraph: graph,
		driver:         driver,
		sysInitPath:    sysInitPath,
		execDriver:     ed,
		eng:            eng,
		trustStore:     t,
		statsCollector: newStatsCollector(1 * time.Second),
	}

	// Setup shutdown handlers
	// FIXME: can these shutdown handlers be registered closer to their source?
	eng.OnShutdown(func() {
		// FIXME: if these cleanup steps can be called concurrently, register
		// them as separate handlers to speed up total shutdown time
		if err := daemon.shutdown(); err != nil {
			log.Errorf("daemon.shutdown(): %s", err)
		}

		// TODO Windows. Don't believe PortAllocator will be needed.
		// TODO Windows. Actually, I think we do :)
		//if err := portallocator.ReleaseAll(); err != nil {
		//	log.Errorf("portallocator.ReleaseAll(): %s", err)
		//}
		if err := daemon.driver.Cleanup(); err != nil {
			log.Errorf("daemon.driver.Cleanup(): %s", err.Error())
		}
		if err := daemon.containerGraph.Close(); err != nil {
			log.Errorf("daemon.containerGraph.Close(): %s", err.Error())
		}
	})

	if err := daemon.restore(); err != nil {
		return nil, err
	}

	return daemon, nil
}

// Factored out AppArmor.
// TODO Windows: What about label? Is this valid?
func parseSecurityOpt(container *Container, config *runconfig.HostConfig) error {
	var (
		labelOpts []string
		err       error
	)

	for _, opt := range config.SecurityOpt {
		con := strings.SplitN(opt, ":", 2)
		if len(con) == 1 {
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
		switch con[0] {
		case "label":
			labelOpts = append(labelOpts, con[1])
		default:
			return fmt.Errorf("Invalid --security-opt: %q", opt)
		}
	}

	container.ProcessLabel, container.MountLabel, err = label.InitLabels(labelOpts)
	return err
}
