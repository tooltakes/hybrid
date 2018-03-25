// reserved names: DIRECT localhost 127.0.0.1 0.0.0.0 0
// env:
// HYBRID_CONFIG=$HOME/.hybrid/hybrid.json
// HYBRID_DEV=false
// HYBRID_EXPOSE=7777
// HYBRID_FILES_ROOT=$HOME/.hybrid/files-root
// HYBRID_RULES_ROOT=$HOME/.hybrid/rules-root
// HYBRID_FILE_SERVERS_DISABLED=a,b,c
// HYBRID_ROUTER_DISABLED=a,b,c
package hybridclient

// server types

type TcpServer struct {
	Name            string `validate:"omitempty,hostname"`
	Addr            string `validate:"tcp_addr"`
	NoTLS           bool
	ClientScalarHex string `validate:"omitempty,len=64,hexadecimal"`
	ServerPublicHex string `validate:"len=64,hexadecimal"`
	NoAuth          bool
	Token           string `validate:"lte=732"`
}

type ToxServer struct {
	Name       string `validate:"omitempty,hostname"`
	AddressHex string `validate:"len=76,hexadecimal"`
	Token      string `validate:"lte=732"`
}

type FileServer struct {
	Name     string `validate:"omitempty,hostname"`
	DirName  string `validate:"hostname"`
	Redirect map[string]string
	Dev      bool
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
	AdpRouter   *AdpRouter
	IPNetRouter *IPNetRouter
}

type ToxNode struct {
	Addr    string `validate:"hostname"`
	Port    uint16 `validate:"required"`
	TcpPort uint16
	Pubkey  string `validate:"len=64,hexadecimal"`
}

type Config struct {
	Schema string `json:"-" yaml:"-" toml:"-"`

	Config      string `env:"HYBRID_CONFIG"     validate:"required" default:"$HOME/.hybrid/hybrid.json" json:"-" yaml:"-" toml:"-"`
	Dev         bool   `env:"HYBRID_DEV"`
	Expose      uint16 `env:"HYBRID_EXPOSE"     validate:"gt=0"     default:"7777"`
	FileRootDir string `env:"HYBRID_FILES_ROOT" validate:"required" default:"$HOME/.hybrid/files-root"`
	RuleRootDir string `env:"HYBRID_RULES_ROOT" validate:"required" default:"$HOME/.hybrid/rules-root"`

	ScalarHex string `validate:"len=64,hexadecimal"`
	ToxNospam uint32 `validate:"required"`
	Token     string `validate:"required,lte=732"`

	TcpServers       []TcpServer
	ToxServers       []ToxServer
	FileServers      []FileServer
	HttpProxyServers []HttpProxyServer

	Routers []RouterItem

	ToxNodes []ToxNode
}
