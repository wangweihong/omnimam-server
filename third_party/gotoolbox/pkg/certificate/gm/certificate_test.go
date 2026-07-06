package gm

import (
	"encoding/pem"
	"fmt"
	"github.com/wangweihong/gotoolbox/pkg/certificate/helper"
	"testing"

	gmx509 "github.com/tjfoc/gmsm/x509"
)

func TestNewSelfSignedCert(t *testing.T) {
	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	keyPem, err := EncodeKeyPEM(key)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(keyPem))

	cfg := &helper.CertConfig{
		CommonName:   "wwhvw",
		Organization: []string{"wwhvw"},
		AltNames:     helper.AltNames{},
		ExtKeyUsages: []helper.ExtKeyUsage{helper.ExtKeyUsageAny},
	}

	certPemBytes, err := NewSelfSignedCert(cfg, key, true)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(certPemBytes))

	p, _ := pem.Decode(certPemBytes)

	if _, err := gmx509.ParseCertificate(p.Bytes); err != nil {
		t.Fatal(err)
	}

	//cert := "-----BEGIN GM CERTIFICATE-----\nMIIBUTCB+KADAgECAgEAMAoGCCqBHM9VAYN1MCAxDjAMBgNVBAoTBXd3aHZ3MQ4w\nDAYDVQQDEwV3d2h2dzAeFw0yMjA5MDYxMTE4MDRaFw0zMjA5MDMxMTE4MDRaMCAx\nDjAMBgNVBAoTBXd3aHZ3MQ4wDAYDVQQDEwV3d2h2dzBZMBMGByqGSM49AgEGCCqB\nHM9VAYItA0IABG8EYy83WCs/dppaNRysDYiHHvcwoOa4zheHjBFG0qbx/39dMJ+L\nulz2P80KEfjX6BeDpaBmdh1xNauOmcM0ntGjIzAhMA4GA1UdDwEB/wQEAwICpDAP\nBgNVHRMBAf8EBTADAQH/MAoGCCqBHM9VAYN1A0gAMEUCIQCHVOYFwyPxo1+AuNaW\nugUPsxEZ0U5ZzCCW2xu8r5xJzAIgHYDcdhWgSoFoJMEcKWrVQaHvPyMa59xlhkdO\niOBUhzY=\n-----END GM CERTIFICATE-----"
	//key := "-----BEGIN GM PRIVATE KEY-----\nMIGTAgEAMBMGByqGSM49AgEGCCqBHM9VAYItBHkwdwIBAQQgVSxDKnK98j7r93lm\nTR6DmY0phwAAGoUuDC2EGEiGaGWgCgYIKoEcz1UBgi2hRANCAARvBGMvN1grP3aa\nWjUcrA2Ihx73MKDmuM4Xh4wRRtKm8f9/XTCfi7pc9j/NChH41+gXg6WgZnYdcTWr\njpnDNJ7R\n-----END GM PRIVATE KEY-----"
}
