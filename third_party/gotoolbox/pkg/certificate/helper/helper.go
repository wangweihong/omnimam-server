package helper

import (
	"net"
	"time"
)

const (
	Duration365d = time.Hour * 24 * 365
	// certificate PEM block id
	PemGMCertificate        = "GM CERTIFICATE"
	PemCertificateBlockType = "RSA CERTIFICATE" // can openssl tool parse this pem block?
	PemGMPrivateKey         = "GM PRIVATE KEY"
	PemRSAPrivateKey        = "RSA PRIVATE KEY"
)

type CertConfig struct {
	CommonName   string
	Organization []string
	AltNames     AltNames
	ExtKeyUsages []ExtKeyUsage // must set
}

type AltNames struct {
	DNSNames []string
	IPs      []net.IP
}

type KeyUsage int

const (
	KeyUsageDigitalSignature KeyUsage = 1 << iota
	KeyUsageContentCommitment
	KeyUsageKeyEncipherment
	KeyUsageDataEncipherment
	KeyUsageKeyAgreement
	KeyUsageCertSign
	KeyUsageCRLSign
	KeyUsageEncipherOnly
	KeyUsageDecipherOnly
)

type ExtKeyUsage int

const (
	ExtKeyUsageAny ExtKeyUsage = iota
	ExtKeyUsageServerAuth
	ExtKeyUsageClientAuth
	ExtKeyUsageCodeSigning
	ExtKeyUsageEmailProtection
	ExtKeyUsageIPSECEndSystem
	ExtKeyUsageIPSECTunnel
	ExtKeyUsageIPSECUser
	ExtKeyUsageTimeStamping
	ExtKeyUsageOCSPSigning
	ExtKeyUsageMicrosoftServerGatedCrypto
	ExtKeyUsageNetscapeServerGatedCrypto
)
