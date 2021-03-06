// reserved names: DIRECT over with hybrid
// env:
// HYBRID_ROOT_PATH=$HOME/.hybrid
// HYBRID_DEV=false
// HYBRID_BIND=:7777
// HYBRID_FILE_SERVERS_DISABLED=a,b,c
// HYBRID_ROUTER_DISABLED=a,b,c
package config

const (
	HybridIpfsProtocolVersion = "1.0"
	HybridIpfsProtocol        = "/hybrid/1.0"
)

type Log struct {
	Dev bool

	Level string `validate:"omitempty,oneof=debug info warn error dpanic panic fatal"`

	// Target accepts "nop", "tcp://host:port?timeout=5s", filepath or sentryDSN.
	// Register NewTCPSink to support tcp sink. Default is stderr.
	Target string
}

type Ipfs struct {
	ListenProtocols   []string `validate:"unique"   default:"[\"/hybrid/1.0\"]"`
	FakeApiListenAddr string   `validate:"tcp_addr" default:"127.0.127.1:1270"`

	GatewayServerName string `validate:"omitempty,hostname"`
	ApiServerName     string `validate:"omitempty,hostname"`

	Profile          []string `validate:"unique"`
	AutoMigrate      bool
	EnableIPNSPubSub bool
	EnablePubSub     bool
	EnableMultiplex  bool

	Token string `validate:"lte=732"`
}

// server types

type IpfsServer struct {
	Name     string `validate:"omitempty,hostname"`
	Peer     string `validate:"required"`
	Protocol string `validate:"required" default:"/hybrid/1.0"`
	Token    string `validate:"lte=732"`
}

type FileServer struct {
	Name        string `validate:"omitempty,hostname"`
	RootZipName string `validate:"hostname"`
	Redirect    map[string]string
	Dev         bool
}

type HttpProxyServer struct {
	Name      string `validate:"omitempty,hostname"`
	Host      string `validate:"tcp_addr"`
	KeepAlive bool
}

// server types end

// routers

type AdpRouter struct {
	B64RuleDirName      string `validate:"omitempty,hostname"`
	TxtRuleDirName      string `validate:"omitempty,hostname,nefield=B64RuleDirName"`
	Blocked             string `validate:"omitempty,hostname"`
	Unblocked           string `validate:"omitempty,hostname,nefield=Blocked"`
	EtcHostsIPAsBlocked bool
	Dev                 bool
}

type IPNetRouter struct {
	IPs       []string `validate:"dive,ip"`
	Nets      []string `validate:"dive,cidr"`
	Matched   string   `validate:"omitempty,hostname"`
	Unmatched string   `validate:"omitempty,hostname,nefield=Matched"`
	FileTest  string   `validate:"omitempty,hostname"`
}

// routers end

type RouterItem struct {
	Name string `validate:"omitempty,hostname"`
	// router
	Adp   *AdpRouter
	IPNet *IPNetRouter
}

type Config struct {
	Schema   string `json:"-" yaml:"-" toml:"-"`
	RootPath string `env:"HYBRID_ROOT_PATH" validate:"required" default:"$HOME/.hybrid" json:"-" yaml:"-" toml:"-"`

	Version string
	Dev     bool   `env:"HYBRID_DEV"`
	Bind    string `env:"HYBRID_BIND validate:"omitempty,tcp_addr"`

	FlushIntervalMS uint `default:"200"`

	// Token is fallback token that will be veried by servers, both Ipfs
	Token string `validate:"omitempty,lte=732"`

	Log  Log
	Ipfs Ipfs

	IpfsServers      []IpfsServer
	FileServers      []FileServer
	HttpProxyServers []HttpProxyServer

	Routers []RouterItem

	tree *ConfigTree
}
