package hybrid

const (
	HostHttpPrefix  byte = 'H'
	HostHttpsPrefix byte = 'S'

	HostHybridSuffix = ".hybrid"
	HostLocalhost    = "localhost"
	HostLocal127     = "127.0.0.1"
	HostLocal0000    = "0.0.0.0"
	HostLocal0       = "0"

	HostPing        = "P"
	HostLocalServer = "0"

	ClientAuthOK byte = 0xa1
)

var (
	StandardConnectOK               = []byte("HTTP/1.1 200 OK\r\n\r\n")
	StandardLocalServiceUnaviliable = []byte("HTTP/1.1 503 Local Service Unaviliable\r\n\r\n")
)
