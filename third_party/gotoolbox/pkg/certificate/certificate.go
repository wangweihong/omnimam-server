package certificate

import (
	"crypto"

	"github.com/wangweihong/gotoolbox/pkg/certificate/helper"
)

type CertificateGenerator interface {
	NewPrivateKey() (crypto.Signer, error)
	NewSelfSignedCert(cfg *helper.CertConfig, key crypto.Signer, IsCa bool) ([]byte, error)
}
