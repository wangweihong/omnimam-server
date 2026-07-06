package certificate

import (
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/certificate/gm"
	"github.com/wangweihong/gotoolbox/pkg/certificate/x509"
)

var _ CertificateCrypto = &gm.CertificateCrypto{}
var _ CertificateCrypto = &x509.CertificateCrypto{}

type CertificateCrypto interface {
	//KeyAlgorithm() PublicKeyAlgorithm
	PrivateSign(msg []byte) ([]byte, error) // 私钥加密
	//用公钥验证签名数据signature是否由msg签名而成
	PublicVerify(msg, signature []byte) error
	PublicSign(msg []byte) ([]byte, error)     // 公钥加密
	PrivateVerify(msg, signature []byte) error // 私钥解密
}

type CertificateType int

const (
	UnknownCertificate CertificateType = iota
	CertificateTypeX509
	CertificateTypeGM //国密
)

func NewCertificateCrypto(t CertificateType) (CertificateCrypto, error) {
	switch t {
	case CertificateTypeX509:
		privateKey, err := x509.NewPrivateKey()
		if err != nil {
			return nil, err
		}
		return x509.NewCertificateCrypto(privateKey)
	case CertificateTypeGM:
		privateKey, err := gm.NewPrivateKey()
		if err != nil {
			return nil, err
		}
		return gm.NewCertificateCrypto(privateKey)
	}
	return nil, fmt.Errorf("invalid certificate type:%v", t)
}
