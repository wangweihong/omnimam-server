package grpctls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"google.golang.org/grpc/credentials"
)

// NewTlsServerCredentials generate tls-credentials for server.
func NewTlsServerCredentials(serverCertData []byte, serverKeyData []byte) (credentials.TransportCredentials, error) {
	// Load server's certificate and private key
	serverCert, err := tls.X509KeyPair(serverCertData, serverKeyData)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		// 忽略客户端证书校验
		ClientAuth: tls.NoClientCert,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	return credentials.NewTLS(config), nil
}

func LoadTLSServerCredentials(serverCertPath, serverKeyPath string) (credentials.TransportCredentials, error) {
	certPEMBlock, err := os.ReadFile(serverCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate")
	}

	keyPEMBlock, err := os.ReadFile(serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server key")
	}

	return NewTlsServerCredentials(certPEMBlock, keyPEMBlock)
}

// NewMutualTlsServerCredentials generate mtls-credentials for server.
func NewMutualTlsServerCredentials(
	clientCA []byte,
	serverCertData []byte,
	serverKeyData []byte,
) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed client's certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(clientCA) {
		return nil, fmt.Errorf("failed to add client CA's certificate")
	}

	// Load server's certificate and private key
	serverCert, err := tls.X509KeyPair(serverCertData, serverKeyData)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	return credentials.NewTLS(config), nil
}

func LoadMutualTlsServerCredentials(
	clientCAPath, serverCertPath, serverKeyPath string,
) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed client's certificate
	pemClientCA, err := ioutil.ReadFile(clientCAPath)
	if err != nil {
		return nil, err
	}

	certPEMBlock, err := os.ReadFile(serverCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate")
	}

	keyPEMBlock, err := os.ReadFile(serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server key")
	}

	return NewMutualTlsServerCredentials(pemClientCA, certPEMBlock, keyPEMBlock)
}
