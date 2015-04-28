package daemon

import (
	"github.com/docker/docker/daemon/networkdriver"
	"github.com/docker/docker/daemon/networkdriver/bridge"
	"github.com/docker/docker/opts"
	flag "github.com/docker/docker/pkg/mflag"
	"github.com/docker/docker/runconfig"
)

const (
	defaultNetworkMtu    = 1500
	disableNetworkBridge = "none"
)

// TODO FIX THIS COMMENT
// CommonConfig defines the configuration of a docker daemon.
// These are the configuration settings that you pass
// to the docker daemon when you launch it with say: `docker -d -e lxc`
type CommonConfig struct {
	// Cross-platform common configuration of a docker daemon
	// FIXME: separate runtime configuration from http api configuration
	AutoRestart    bool
	Bridge         bridge.Config
	Context        map[string][]string
	CorsHeaders    string
	DisableNetwork bool
	Dns            []string
	DnsSearch      []string
	EnableCors     bool
	ExecDriver     string
	GraphDriver    string
	GraphOptions   []string
	Labels         []string
	LogConfig      runconfig.LogConfig
	Mtu            int
	Pidfile        string
	Root           string
	TrustKeyPath   string
}

// CommonInstallFlags adds command-line options to the top-level flag parser for
// the current process.
// Subsequent calls to `flag.Parse` will populate config with values parsed
// from the command-line.
func (config *Config) CommonInstallFlags() {

	flag.BoolVar(&config.AutoRestart, []string{"#r", "#-restart"}, true, "--restart on the daemon has been deprecated in favor of --restart policies on docker run")
	flag.BoolVar(&config.Bridge.EnableIptables, []string{"#iptables", "-iptables"}, true, "Enable addition of iptables rules")
	flag.BoolVar(&config.Bridge.EnableIpForward, []string{"#ip-forward", "-ip-forward"}, true, "Enable net.ipv4.ip_forward")
	flag.BoolVar(&config.Bridge.EnableIpMasq, []string{"-ip-masq"}, true, "Enable IP masquerading")
	flag.BoolVar(&config.Bridge.EnableIPv6, []string{"-ipv6"}, false, "Enable IPv6 networking")
	flag.StringVar(&config.Bridge.IP, []string{"#bip", "-bip"}, "", "Specify network bridge IP")
	flag.StringVar(&config.Bridge.Iface, []string{"b", "-bridge"}, "", "Attach containers to a network bridge")
	flag.StringVar(&config.Bridge.FixedCIDR, []string{"-fixed-cidr"}, "", "IPv4 subnet for fixed IPs")
	flag.StringVar(&config.Bridge.FixedCIDRv6, []string{"-fixed-cidr-v6"}, "", "IPv6 subnet for fixed IPs")
	flag.StringVar(&config.Bridge.DefaultGatewayIPv4, []string{"-default-gateway"}, "", "Container default gateway IPv4 address")
	flag.StringVar(&config.Bridge.DefaultGatewayIPv6, []string{"-default-gateway-v6"}, "", "Container default gateway IPv6 address")
	flag.BoolVar(&config.Bridge.InterContainerCommunication, []string{"#icc", "-icc"}, true, "Enable inter-container communication")
	flag.StringVar(&config.GraphDriver, []string{"s", "-storage-driver"}, "", "Storage driver to use")
	flag.StringVar(&config.ExecDriver, []string{"e", "-exec-driver"}, "native", "Exec driver to use")
	flag.IntVar(&config.Mtu, []string{"#mtu", "-mtu"}, 0, "Set the containers network MTU")
	flag.BoolVar(&config.EnableCors, []string{"#api-enable-cors", "#-api-enable-cors"}, false, "Enable CORS headers in the remote API, this is deprecated by --api-cors-header")
	flag.StringVar(&config.CorsHeaders, []string{"-api-cors-header"}, "", "Set CORS headers in the remote API")
	opts.IPVar(&config.Bridge.DefaultIp, []string{"#ip", "-ip"}, "0.0.0.0", "Default IP when binding container ports")
	opts.ListVar(&config.GraphOptions, []string{"-storage-opt"}, "Set storage driver options")
	// FIXME: why the inconsistency between "hosts" and "sockets"?
	opts.IPListVar(&config.Dns, []string{"#dns", "-dns"}, "DNS server to use")
	opts.DnsSearchListVar(&config.DnsSearch, []string{"-dns-search"}, "DNS search domains to use")
	opts.LabelListVar(&config.Labels, []string{"-label"}, "Set key=value labels to the daemon")
	flag.StringVar(&config.LogConfig.Type, []string{"-log-driver"}, "json-file", "Default driver for container logs")
}

func getDefaultNetworkMtu() int {
	if iface, err := networkdriver.GetDefaultRouteIface(); err == nil {
		return iface.MTU
	}
	return defaultNetworkMtu
}
