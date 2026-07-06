package x509

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

type CertificateCrypto struct {
	key *rsa.PrivateKey
}

func NewCertificateCrypto(key *rsa.PrivateKey) (*CertificateCrypto, error) {
	if key == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return &CertificateCrypto{
		key: key,
	}, nil
}

// 私钥加密
func (c *CertificateCrypto) PrivateSign(msg []byte) ([]byte, error) {
	hash := crypto.SHA256.New()
	hash.Write(msg)
	sign, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, hash.Sum(nil))
	if err != nil {
		return nil, err
	}

	return sign, nil
}

// 公钥解密
func (c *CertificateCrypto) PublicVerify(msg, signature []byte) error {
	hash := crypto.SHA256.New()
	hash.Write(msg)

	return rsa.VerifyPKCS1v15(&c.key.PublicKey, crypto.SHA256, hash.Sum(nil), signature)
}

// 公钥加密
func (c *CertificateCrypto) PublicSign(msg []byte) ([]byte, error) {
	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, &c.key.PublicKey, msg)
	if err != nil {
		return nil, err
	}

	return cipherText, nil
}

// 私钥解密
func (c *CertificateCrypto) PrivateVerify(msg []byte, signature []byte) error {
	text, err := rsa.DecryptPKCS1v15(rand.Reader, c.key, signature)
	if err != nil {
		return err
	}

	if bytes.Compare(text, msg) != 0 {
		return fmt.Errorf("verify fail")
	}

	return nil
}
