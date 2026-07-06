package httpsvr_test

import (
	cryptotls "crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/wangweihong/gotoolbox/pkg/tls"
)

const (
	caData   = "-----BEGIN CERTIFICATE-----\nMIIFlTCCA32gAwIBAgIUOc6BpN0Oub+CYhSubB5/F9E9GaMwDQYJKoZIhvcNAQEN\nBQAwWjELMAkGA1UEBhMCQ04xEjAQBgNVBAgMCUd1YW5nZG9uZzERMA8GA1UEBwwI\nU2hlbnpoZW4xEjAQBgNVBAoMCUVhenlDbG91ZDEQMA4GA1UECwwHRGV2ZWxvcDAe\nFw0yMzA3MjYwNjQ2NTJaFw0zMzA3MjMwNjQ2NTJaMFoxCzAJBgNVBAYTAkNOMRIw\nEAYDVQQIDAlHdWFuZ2RvbmcxETAPBgNVBAcMCFNoZW56aGVuMRIwEAYDVQQKDAlF\nYXp5Q2xvdWQxEDAOBgNVBAsMB0RldmVsb3AwggIiMA0GCSqGSIb3DQEBAQUAA4IC\nDwAwggIKAoICAQCXS7sY/f2KGF4cis/tcQUyArXpyJ3MgiGpCJmv94GUSIVAzbWU\neuKdmqbh+zBvmGX8Jgan0KAC+2o5/8WYZLw9v7H1Py1DtI12MYW/QaI4+734ZsHc\nyg4IK3rmTGXWR1TLusUdJcywMBSl7BpJ8C1Vr1JomaSFuE/tP9i3fv0BT02lFlHd\n0+AvI9c9Ridhrymnn2qAFY8EKuPmu1lRV8wUB8oIL7lM3/CIFJUkGsmUxdJuTZU4\npuDn8DnTQ886jrPepxe4+j0zweZQbVgfqiQaZ+Ubl48fUtS0HZbthXJUUa260XPY\ns4nkUSGZakn2lOOBu4BUAfkwLSqrqpOX3sMujOiEXZ80BM6jQ55INurqQuMVdWTf\neJr+by5X7plUdF3Hd+7oikw6d2TvM8CoQTVNH+F+MEWJ8sncugBOhPIbJJbW6P9e\nuekQq6r61oyqg4AUGTvi6tDc6UC6FpDnP/oJlei7BCH3fr0ASws1ePyQ+reyp439\neiwMdqEHAzon7VZu7Tot9eLELSsIPb/5EkwV6qAirn3aTKol13M7+YI2UptCSIa/\n5+ruY92OueuZf9Z6VoqTt95lSrlET/femVikXCflFySWR+xbSWjdEY8faSDAIYnt\nEg/5MPcK7k3aGL3BdiU+krS7UAdbVbx8y03Zw8hO7UdfNAD2S1y8BKH2HQIDAQAB\no1MwUTAdBgNVHQ4EFgQU2YYebmvS23/VSPatO1awTf1SjkYwHwYDVR0jBBgwFoAU\n2YYebmvS23/VSPatO1awTf1SjkYwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0B\nAQ0FAAOCAgEAMMRVan8I2h7Rj1eEIrfdL8/5pLT6O+bPSYoNUgnj1EuSYAr+u+sS\nr0l7gFwg1WISQlxRbmyZKQBE44+8Pn/qHVRKC0RUVja8j70HpxY2clLLolWunETQ\n6MbplZ8w4ei1Rx5L6S4Vcz+EAJwmseTJa5B8U69coZzeuiyHAUKmnLsSudJr9bTc\n5vnMOve+eG8Y4EKcpYMwJJy8272eQFNXwKmIrfD/5qTV03aMVcANXxvGpZWBYz5w\nCE/NDsMO/BnRFm4//ml5cKiTppG9u3/94Ah2bz4dATZl8AxwfQ2vOVQqKDXm5XwD\nH2XV82FJDTfAUfYwQZhSzXXwMRYgnKfDyLxmuRrRO1NCN8ddFpH+SLbhNjQ56cJT\n2qvsXJ/n8AaKeAr2mGEJ0d8cy69IxLZSdLDmL081GdhRGTExMnIxfLL0wvOMNlMM\nQokVRGuKShrE8LFNVjTlfSLBmuVKomugCXn+VVJFhRMFkSSg1kcmusEqEWuWsx9Y\n7hMs/MVVRS0YjTjxgTFNFcevNXkY0xoHzur1ccCIArJXroF2UzUctF/dpqMJP6bk\npIT7Pu3UNj/qChMLZ8ostJyhM/24PwkLLHy1v9lU+86lYWZGLhL3QSSnctypI4fF\nGCx0CfIEfjVIKsvaSa4v+JTX/yiSnUj8ChNM7r5I2bDxB7vy2wYPn/Y=\n-----END CERTIFICATE-----\n"
	keyData  = "-----BEGIN PRIVATE KEY-----\n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\nZg66Jj6F2cJFOxnaQ09mSxszQJBU94A=\n-----END PRIVATE KEY-----\n"
	certData = "-----BEGIN CERTIFICATE-----\n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\n-----END CERTIFICATE-----\n"
)

func testTls(conf *httpsvr.Config, ca string) {
	s, err := conf.Complete().New()
	So(err, ShouldBeNil)

	go func() {
		s.Run()
	}()
	// Wait for the server to start (you can use a more sophisticated wait mechanism)
	time.Sleep(3 * time.Second)

	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM([]byte(ca))
	So(ok, ShouldBeTrue)
	// 创建一个带有证书的 TLS 配置
	tlsConfig := &cryptotls.Config{
		//客户端证书
		//	Certificates: []cryptotls.Certificate{cert},
		RootCAs: caCertPool,
	}

	// 创建一个自定义的 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	// 发起 HTTP 请求
	resp, err := client.Get("https://" + conf.SecureServing.Address() + "/version")
	So(err, ShouldBeNil)
	defer resp.Body.Close()

	So(resp.StatusCode, ShouldEqual, http.StatusOK)
	s.Close()
}

func TestGenericHTTPServer_Serving(t *testing.T) {
	Convey("installApi", t, func() {
		conf := httpsvr.NewConfig()
		conf.SecureServing = &httpsvr.SecureServingInfo{
			BindAddress: "0.0.0.0",
			BindPort:    55557,
			CertKey: tls.CertData{
				Cert: certData,
				Key:  keyData,
			},
			Required: true,
		}
		SkipConvey("right ca", func() {
			testTls(conf, caData)
		})
	})
}

func TestGenericHTTPServer_InstallAPIs(t *testing.T) {
	Convey("installApi", t, func() {
		conf := httpsvr.NewConfig()

		Convey("测试特性路由是否正常安装", func() {
			conf.Version = true
			conf.Healthz = true
			conf.EnableMetrics = true
			conf.Profiling = &httpsvr.FeatureProfilingInfo{
				EnableProfiling:     true,
				StandAloneProfiling: false,
				ProfileAddress:      "",
			}

			s, err := conf.Complete().New()
			So(err, ShouldBeNil)

			{
				req, _ := http.NewRequest(http.MethodGet, "/version", nil)
				w := httptest.NewRecorder()
				s.Engine.ServeHTTP(w, req)
				So(w.Code, ShouldEqual, 200)
			}

			{
				req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
				w := httptest.NewRecorder()
				s.Engine.ServeHTTP(w, req)
				So(w.Code, ShouldEqual, 200)
			}

			{
				req, _ := http.NewRequest(http.MethodGet, "/metrics", nil)
				w := httptest.NewRecorder()
				s.Engine.ServeHTTP(w, req)
				So(w.Code, ShouldEqual, 200)
			}

			{
				req, _ := http.NewRequest(http.MethodGet, "/debug/pprof/profile", nil)
				w := httptest.NewRecorder()
				s.Engine.ServeHTTP(w, req)
				So(w.Code, ShouldEqual, 200)
			}

		})
	})
}
