package hybrid

import (
	"crypto/tls"
	"crypto/x509"
)

type KeyPair struct {
	Crt []byte
	Key []byte
}

type TLSConfig struct {
	Chain    []byte
	KeyPairs []KeyPair
}

func (c *TLSConfig) Build() (*tls.Config, error) {
	if c == nil {
		return nil, nil
	}
	return BuildTLSConfig(c.Chain, c.KeyPairs...)
}

func BuildTLSConfig(chain []byte, keyPairs ...KeyPair) (*tls.Config, error) {

	// 1. LoadClientCert
	var certs []tls.Certificate
	for _, keyPair := range keyPairs {
		cert, err := tls.X509KeyPair(keyPair.Crt, keyPair.Key)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}

	// 2. LoadCACert
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(chain)

	return &tls.Config{
		RootCAs:      caPool,
		Certificates: certs,
	}, nil
}
