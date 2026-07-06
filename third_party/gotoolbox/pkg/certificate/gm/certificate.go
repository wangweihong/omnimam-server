package gm

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/tjfoc/gmsm/sm2"
	gmx509 "github.com/tjfoc/gmsm/x509"
	"github.com/wangweihong/gotoolbox/pkg/certificate/helper"
)

type CertificateGenerator struct {
}

func NewCertificateGenerator() *CertificateGenerator {
	return &CertificateGenerator{}
}

func NewPrivateKey() (*sm2.PrivateKey, error) {
	return sm2.GenerateKey(cryptorand.Reader)
}

func newCaCertificateTemplate(cfg helper.CertConfig) *gmx509.Certificate {
	now := time.Now()
	tmpl := &gmx509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0), // 证书序列号, 签名机构唯一
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(helper.Duration365d * 10).UTC(), // CA证书有效期为10年
		KeyUsage:              gmx509.KeyUsageKeyEncipherment | gmx509.KeyUsageDigitalSignature | gmx509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true, //标记为CA证书
	}
	return tmpl
}

func newNormalCertificateTemplate(cfg helper.CertConfig) (*gmx509.Certificate, error) {
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
	certTmpl := &gmx509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    now,
		NotAfter:     now.Add(helper.Duration365d * 10).UTC(),
		KeyUsage:     gmx509.KeyUsageKeyEncipherment | gmx509.KeyUsageDigitalSignature,
		ExtKeyUsage:  helperKeyUsageToGMKeyUsage(cfg.ExtKeyUsages),
	}
	return certTmpl, nil
}

func helperKeyUsageToGMKeyUsage(usages []helper.ExtKeyUsage) []gmx509.ExtKeyUsage {
	if usages == nil {
		return nil
	}

	gKeyUsages := make([]gmx509.ExtKeyUsage, 0, len(usages))
	for _, v := range usages {
		gKeyUsages = append(gKeyUsages, gmx509.ExtKeyUsage(v))
	}
	return gKeyUsages
}

func NewSelfSignedCert(cfg *helper.CertConfig, key crypto.Signer, IsCa bool) ([]byte, error) {
	privKey, ok := key.(*sm2.PrivateKey)
	if !ok {
		return nil, errors.New("x509: certificate private key does not implement crypto.Signer")
	}

	certTmpl := newCaCertificateTemplate(*cfg)
	if !IsCa {
		var err error
		certTmpl, err = newNormalCertificateTemplate(*cfg)
		if err != nil {
			return nil, err
		}
	}

	certDERBytes, err := gmx509.CreateCertificate(certTmpl, certTmpl, &privKey.PublicKey, key)
	if err != nil {
		return nil, err
	}

	cert, err := gmx509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}
	return EncodeCertPEM(cert), nil
}

func EncodeCertPEM(cert *gmx509.Certificate) []byte {
	block := pem.Block{
		Type:  helper.PemGMCertificate,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func DecodeCertPEM(pemCert []byte) (*gmx509.Certificate, error) {
	block, _ := pem.Decode(pemCert)
	if block == nil {
		return nil, errors.New("data doesn't contain a valid certificate")
	}

	if block.Type != helper.PemGMCertificate {
		return nil, fmt.Errorf("expected block type %q, but PEM had type %q", helper.PemGMCertificate, block.Type)
	}

	return gmx509.ParseCertificate(block.Bytes)
}

func EncodeKeyPEM(key *sm2.PrivateKey) ([]byte, error) {
	privateKeyBuf, err := gmx509.MarshalSm2PrivateKey(key, nil)
	if err != nil {
		return nil, err
	}
	var privateKeyBlock *pem.Block = &pem.Block{Bytes: privateKeyBuf, Type: helper.PemGMPrivateKey}
	privateKeyBuf = pem.EncodeToMemory(privateKeyBlock)
	return privateKeyBuf, nil
}

func DecodeKeyPEM(pemKey []byte) (*sm2.PrivateKey, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, errors.New("data doesn't contain a valid key")
	}

	if block.Type != helper.PemGMPrivateKey {
		return nil, fmt.Errorf("expected block type %q, but PEM had type %q", helper.PemGMPrivateKey, block.Type)
	}

	return gmx509.ParseSm2PrivateKey(block.Bytes)
}
