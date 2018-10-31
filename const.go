package hybrid

const (
	HostHttpPrefix  byte = 'H'
	HostHttpsPrefix byte = 'S'
)

var (
	StandardConnectOK = []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n")
	Standard301Prefix = []byte("HTTP/1.1 301 Moved Permanently\r\nLocation: ")
)
