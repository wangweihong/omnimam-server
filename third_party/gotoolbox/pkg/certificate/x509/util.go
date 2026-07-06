package x509

import (
	"crypto/tls"
	gx509 "crypto/x509"
	"encoding/base64"
)

func ParseCert(certData string, keyData string) (*tls.Certificate, *gx509.Certificate, error) {
	cert, err := base64.StdEncoding.DecodeString(certData)
	if err != nil {
		return nil, nil, err
	}

	key, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return nil, nil, err
	}

	keyPair, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		return nil, nil, err
	}

	keyPair.Leaf, err = gx509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, nil, err
	}
	return &keyPair, keyPair.Leaf, nil
}
