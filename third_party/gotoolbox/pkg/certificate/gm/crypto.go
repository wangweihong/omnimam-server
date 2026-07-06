package gm

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/tjfoc/gmsm/sm2"
)

type CertificateCrypto struct {
	key *sm2.PrivateKey
}

func NewCertificateCrypto(key *sm2.PrivateKey) (*CertificateCrypto, error) {
	if key == nil {
		return nil, fmt.Errorf("private key is nil")
	}
	return &CertificateCrypto{key: key}, nil
}

// 私钥加密
func (c *CertificateCrypto) PrivateSign(msg []byte) ([]byte, error) {
	signature, err := c.key.Sign(rand.Reader, msg, nil)
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// 公钥解密
func (c *CertificateCrypto) PublicVerify(msg, signature []byte) error {
	if ok := c.key.PublicKey.Verify(msg, signature); !ok {
		return errors.New("public key failed to verify")
	}
	return nil
}

// 公钥加密
func (c *CertificateCrypto) PublicSign(msg []byte) ([]byte, error) {
	cipherText, err := sm2.Encrypt(&c.key.PublicKey, msg, rand.Reader, sm2.C1C3C2)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Error: failed to public encrypt: %v\n", err))
	}
	return cipherText, nil
}

// 私钥解密
func (c *CertificateCrypto) PrivateVerify(msg []byte, signature []byte) error {
	text, err := sm2.Decrypt(c.key, signature, sm2.C1C3C2)
	if err != nil {
		return fmt.Errorf("Error: failed to private decrypt: %v\n", err)
	}

	if bytes.Compare(msg, text) != 0 {
		return fmt.Errorf("private verify fail")
	}

	return nil
}
