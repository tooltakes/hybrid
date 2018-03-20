package hybrid

const (
	HostHttpPrefix  byte = 'H'
	HostHttpsPrefix byte = 'S'

	HostHybridSuffix = ".hybrid"
	HostLocalhost    = "localhost.hybrid"
	HostLocal127     = "127.0.0.1.hybrid"

	HostPing        = "P"
	HostLocalServer = "L"
)

var (
	StandardConnectOK = []byte("HTTP/1.1 200 OK\r\n\r\n")
)
