package grpc

import (
	"github.com/empirefox/hybrid/pkg/auth"
	"github.com/empirefox/hybrid/pkg/authstore"
	"go.uber.org/zap"
)

type Verifier struct {
	log      *zap.Logger
	verifier auth.GetKeyFunc
}

func (v *Verifier) HybridVerify(peerID, token []byte) bool {
	_, err := v.verifier.Verify(peerID, token)
	if err != nil {
		v.log.Debug("Verify", zap.ByteString("peerID", peerID), zap.Error(err))
	}
	return err == nil
}

func NewVerifier(store *authstore.KeyStore, log *zap.Logger) *Verifier {
	return &Verifier{
		log:      log,
		verifier: store.GetKey,
	}
}
