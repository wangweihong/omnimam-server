package grpctls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"

	"google.golang.org/grpc/credentials"
)

// 从客户端的角度
// 单向认证为通过CA证书来验证服务端证书是否由自己签名, 且服务端和证书主体相同(服务IP/域名和证书匹配)
// 双向认证为在认证服务端证书时，将自己的证书交由服务端进行认证。

// NewTlsClientSkipVerifiedCredentials generate tls-credentials skip server certificate verify for client.
func NewTlsClientSkipVerifiedCredentials() (credentials.TransportCredentials, error) {
	config := &tls.Config{
		// 跳过服务端证书检测
		// 注意该标识在服务器开启mTLS时无效
		InsecureSkipVerify: true,
	}

	return credentials.NewTLS(config), nil
}

// NewTlsClientCredentials generate tls-credentials for client.
func NewTlsClientCredentials(serverCA []byte) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(serverCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Create the credentials and return it
	config := &tls.Config{
		RootCAs: certPool,
	}

	return credentials.NewTLS(config), nil
}

func LoadTLSClientCredentials(serverCAPath string) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile(serverCAPath)
	if err != nil {
		return nil, err
	}

	return NewTlsClientCredentials(pemServerCA)
}

// NewMutualTlsClientCredentials generate mtls-credentials for client.
func NewMutualTlsClientCredentials(
	serverCA []byte,
	clientCertData []byte,
	clientKeyData []byte,
) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(serverCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	// Load client's certificate and private key
	clientCert, err := tls.X509KeyPair(clientCertData, clientKeyData)
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
	}

	return credentials.NewTLS(config), nil
}

func LoadMutualTlsClientCredentials(
	serverCAPath, clientCertPath, clientKeyPath string,
) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile(serverCAPath)
	if err != nil {
		return nil, err
	}

	certPEMBlock, err := os.ReadFile(clientCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate")
	}

	keyPEMBlock, err := os.ReadFile(clientKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load client key")
	}

	return NewMutualTlsClientCredentials(pemServerCA, certPEMBlock, keyPEMBlock)
}
