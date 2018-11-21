package ipfsdial

import (
	"strings"

	inet "github.com/ipsn/go-ipfs/gxlibs/github.com/libp2p/go-libp2p-net"
)

const (
	ProtocolPrefix       = "/hybrid/"
	ProtocolVersion_v1_0 = "1.0"
	PathTokenPrefix      = "/token/"

	CurrentProtocol = ProtocolPrefix + ProtocolVersion_v1_0
)

type VerifyFunc func(peerID, token []byte) bool

func ListenMatch(protocol string) bool {
	return strings.HasPrefix(protocol, ProtocolPrefix)
}

// TODO refactor to client handshake
func DialProtocol(token []byte) string {
	return ProtocolPrefix + ProtocolVersion_v1_0 + PathTokenPrefix + string(token)
}

// TODO replace verify with server handshake
func VerifyToken(verify VerifyFunc) func(inet.Stream) bool {
	return func(is inet.Stream) bool {
		target := []byte(is.Conn().RemotePeer().Pretty())
		token, ok := parseProtocol(string(is.Protocol()))
		return ok && verify(target, token)
	}
}

func parseProtocol(protocol string) (token []byte, ok bool) {
	s := strings.SplitN(protocol, "/", 5)
	if len(s) != 5 {
		return nil, false
	}

	switch s[2] {
	case ProtocolVersion_v1_0:
		if s[3] != "token" {
			return nil, false
		}
		return []byte(s[4]), ok
	default:
		return nil, false
	}
}
