package x509

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/certificate/helper"
)

const (
	rsaKeySize = 2048
)

type CertificateGenerator struct {
}

func NewCertificateGenerator() *CertificateGenerator {
	return &CertificateGenerator{}
}

func NewPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
}

func newCaCertificateTemplate(cfg helper.CertConfig) *x509.Certificate {
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0), // 证书序列号, 签名机构唯一
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(helper.Duration365d * 10).UTC(), // CA证书有效期为10年
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true, //标记为CA证书
	}
	return tmpl
}

func helperKeyUsageToGMKeyUsage(usages []helper.ExtKeyUsage) []x509.ExtKeyUsage {
	if usages == nil {
		return nil
	}

	gKeyUsages := make([]x509.ExtKeyUsage, 0, len(usages))
	for _, v := range usages {
		gKeyUsages = append(gKeyUsages, x509.ExtKeyUsage(v))
	}
	return gKeyUsages
}

func newNormalCertificateTemplate(cfg helper.CertConfig) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}

	if len(cfg.ExtKeyUsages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	now := time.Now()
	certTmpl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    now,
		NotAfter:     now.Add(helper.Duration365d * 10).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  helperKeyUsageToGMKeyUsage(cfg.ExtKeyUsages),
	}
	return certTmpl, nil
}

func (g *CertificateGenerator) NewPrivateKey() (crypto.Signer, error) {
	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
}

func (g *CertificateGenerator) NewSelfSignedCert(cfg *helper.CertConfig, key crypto.Signer, IsCa bool) ([]byte, error) {
	certTmpl := newCaCertificateTemplate(*cfg)
	if !IsCa {
		var err error
		certTmpl, err = newNormalCertificateTemplate(*cfg)
		if err != nil {
			return nil, err
		}
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, certTmpl, certTmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}

	certs, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}

	return EncodeCertPEM(certs), nil
}

func (g *CertificateGenerator) NewSignedCert(cfg *helper.CertConfig, key crypto.Signer, signer *x509.Certificate) ([]byte, error) {
	if cfg == nil || key == nil || signer == nil {
		return nil, fmt.Errorf("invalid argument")
	}

	certTmpl, err := newNormalCertificateTemplate(*cfg)
	if err != nil {
		return nil, err
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, certTmpl, signer, key.Public(), key)
	if err != nil {
		return nil, err
	}

	certs, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}

	return EncodeCertPEM(certs), nil
}

func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  helper.PemCertificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func DecodeCertPEM(pemCert []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemCert)
	if block == nil {
		return nil, errors.New("data doesn't contain a valid certificate")
	}

	if block.Type != helper.PemCertificateBlockType {
		return nil, fmt.Errorf("expected block type %q, but PEM had type %q", helper.PemCertificateBlockType, block.Type)
	}

	return x509.ParseCertificate(block.Bytes)
}

func EncodeKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	privateKeyBuf := x509.MarshalPKCS1PrivateKey(key)

	var privateKeyBlock *pem.Block = &pem.Block{Bytes: privateKeyBuf, Type: helper.PemRSAPrivateKey}
	privateKeyBuf = pem.EncodeToMemory(privateKeyBlock)
	return privateKeyBuf, nil
}

func DecodeKeyPEM(pemKey []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, errors.New("data doesn't contain a valid key")
	}

	if block.Type != helper.PemRSAPrivateKey {
		return nil, fmt.Errorf("expected block type %q, but PEM had type %q", helper.PemRSAPrivateKey, block.Type)
	}

	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
