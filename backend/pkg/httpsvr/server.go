package httpsvr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/debug"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/profiling"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericmiddleware"

	ginprometheus "github.com/zsais/go-gin-prometheus"

	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/version"

	cryptotls "crypto/tls"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	"golang.org/x/sync/errgroup"
)

// GenericHTTPServer gin.Engine.
type GenericHTTPServer struct {
	*gin.Engine

	// which middleware want to install
	// 注意中间件顺序的影响
	middlewares []string

	// SecureServingInfo holds configuration of the TLS server.
	SecureServingInfo *SecureServingInfo

	// InsecureServingInfo holds configuration of the insecure HTTP server.
	InsecureServingInfo *InsecureServingInfo

	healthz       bool
	enableMetrics bool
	profiling     *FeatureProfilingInfo
	version       bool

	insecureServer, secureServer *http.Server

	runtimeDebug *debug.RuntimeDebugInfo
}

// 安装通用服务的中间件和api
// 1. 这里安装的api仅会被提前安装的插件所影响
// 2. 这里安装的中间件会影响后续所有的接口。如果不希望这里有影响, 可以将中间件和通用路由特性等选项关闭。
func initGenericHTTPServer(s *GenericHTTPServer) {
	s.Setup()
	s.InstallMiddlewares()
	// 注意, 这里的API仅会被上面安装的中间件影响。
	s.InstallAPIs()
	s.InstallRuntimeDebug()
}

// InstallAPIs install generic apis.
func (s *GenericHTTPServer) InstallAPIs() {
	// install healthz handler
	if s.healthz {
		s.GET("/healthz", func(c *gin.Context) {
			c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		})
	}

	// install metric handler
	if s.enableMetrics {
		prometheus := ginprometheus.NewPrometheus("gin")
		prometheus.Use(s.Engine)
	}

	// install pprof handler
	if s.profiling != nil && s.profiling.EnableProfiling {
		if !s.profiling.StandAloneProfiling {
			pprof.Register(s.Engine)
		} else {
			if err := profiling.StartProfilingServer(s.profiling.ProfileAddress); err != nil {
				log.Warnf("start standalone profiling server in %s err:%v", s.profiling.ProfileAddress, err)
			}
		}
	}

	// install version apis
	if s.version {
		s.GET("/version", func(c *gin.Context) {
			c.JSON(http.StatusOK, version.Get())
		})
	}
}

// Setup do some setup work for gin engine.
func (s *GenericHTTPServer) Setup() {
	// 设置gin在debug模式已安装路由的打印写到log
	gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
		log.Infof("%-6s %-s --> %s (%d handlers)", httpMethod, absolutePath, handlerName, nuHandlers)
	}
}

// InstallMiddlewares install generic middlewares.
func (s *GenericHTTPServer) InstallMiddlewares() {
	// install custom middlewares
	for _, m := range s.middlewares {
		mw, ok := genericmiddleware.MiddlewareList[m]
		if !ok {
			log.Warnf("can not find middleware: %s", m)

			continue
		}

		log.Infof("install middleware: %s", m)
		s.Use(mw)
	}
}

func (s *GenericHTTPServer) InstallRuntimeDebug() {
	if s.runtimeDebug == nil {
		return
	}

	if s.runtimeDebug.Enable {
		if s.runtimeDebug.OutputDir == "" {
			log.Warn("runtime debug output is empty")
			return
		}

		if err := os.MkdirAll(s.runtimeDebug.OutputDir, 0o755); err != nil {
			log.Warnf("mkdir runtime debug output dir `%s` err:%w", s.runtimeDebug.OutputDir, err)
			return
		}
		debug.SetupRuntimeDebugSignalHandler(s.runtimeDebug.OutputDir)
		log.Info("runtime debug start")
	}
}

// Run spawns the http server. It only returns when the port cannot be listened on initially.
func (s *GenericHTTPServer) Run() error {
	// For scalability, use custom HTTP configuration mode here
	// For scalability, use custom HTTP configuration mode here
	var eg errgroup.Group

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	if s.InsecureServingInfo.Required {
		s.insecureServer = &http.Server{
			Addr:    s.InsecureServingInfo.Address,
			Handler: s,
			// ReadTimeout:    10 * time.Second,
			// WriteTimeout:   10 * time.Second,
			// MaxHeaderBytes: 1 << 20,
		}

		eg.Go(func() error {
			log.Infof("Start to listening the incoming requests on http address: %s", s.InsecureServingInfo.Address)

			if err := s.insecureServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err.Error())

				return err
			}

			log.Infof("HTTP Server on %s stopped", s.InsecureServingInfo.Address)

			return nil
		})
	}

	if s.SecureServingInfo.Required {
		cert, err := cryptotls.X509KeyPair(
			[]byte(s.SecureServingInfo.CertKey.Cert),
			[]byte(s.SecureServingInfo.CertKey.Key),
		)
		if err != nil {
			log.Fatalf("Failed to generate credentials %s", err.Error())
		}
		// For scalability, use custom HTTP configuration mode here
		s.secureServer = &http.Server{
			Addr:    s.SecureServingInfo.Address(),
			Handler: s,
			TLSConfig: &cryptotls.Config{
				Certificates: []cryptotls.Certificate{cert},
			},
			// ReadTimeout:    10 * time.Second,
			// WriteTimeout:   10 * time.Second,
			// MaxHeaderBytes: 1 << 20,
		}

		eg.Go(func() error {
			log.Infof("Start to listening the incoming requests on https address: %s", s.SecureServingInfo.Address())

			if err := s.secureServer.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err.Error())

				return err
			}

			log.Infof("HTTPs Server on %s stopped", s.SecureServingInfo.Address())

			return nil
		})
	}

	// Ping the server to make sure the router is working.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// 服务启动, 确认路由已经安装成功
	if s.healthz {
		if s.InsecureServingInfo.Required {
			if err := s.pingInsecure(ctx); err != nil {
				return err
			}
		}

		if s.SecureServingInfo.Required {
			if err := s.pingSecure(ctx); err != nil {
				return err
			}
		}
	}

	if err := eg.Wait(); err != nil {
		log.Fatal(err.Error())
	}

	return nil
}

// Close graceful shutdown the apis server.
func (s *GenericHTTPServer) Close() {
	// The context is used to inform the server it has 10 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.SecureServingInfo.Required {
		if err := s.secureServer.Shutdown(ctx); err != nil {
			log.Warnf("Shutdown secure server failed: %s", err.Error())
		}
	}

	if s.InsecureServingInfo.Required {
		if err := s.insecureServer.Shutdown(ctx); err != nil {
			log.Warnf("Shutdown insecure server failed: %s", err.Error())
		}
	}
}

// ping pings the http server to make sure the router is working.
func (s *GenericHTTPServer) pingInsecure(ctx context.Context) error {
	url := fmt.Sprintf("http://%s/healthz", s.InsecureServingInfo.Address)
	if strings.Contains(s.InsecureServingInfo.Address, "0.0.0.0") {
		url = fmt.Sprintf("http://127.0.0.1:%s/healthz", strings.Split(s.InsecureServingInfo.Address, ":")[1])
	}
	if err := s.ping(ctx, url); err != nil {
		log.Fatal("can not ping https server within the specified time interval.")
	}
	return nil
}

// ping pings the http server to make sure the router is working.
func (s *GenericHTTPServer) pingSecure(ctx context.Context) error {
	addr := net.JoinHostPort(s.SecureServingInfo.BindAddress, strconv.Itoa(s.SecureServingInfo.BindPort))
	url := fmt.Sprintf("https://%s/healthz", addr)
	if strings.Contains(addr, "0.0.0.0") {
		url = fmt.Sprintf("https://127.0.0.1:%d/healthz", s.SecureServingInfo.BindPort)
	}
	if err := s.ping(ctx, url); err != nil {
		log.Fatal("can not ping https server within the specified time interval.")
	}
	return nil
}

// ping pings the http server to make sure the router is working.
func (s *GenericHTTPServer) ping(ctx context.Context, url string) error {
	for {
		// Change NewRequest to NewRequestWithContext and pass context it
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		// Ping the server by sending a GET request to `/healthz`.
		tr := &http.Transport{
			TLSClientConfig: &cryptotls.Config{InsecureSkipVerify: true},
		}
		client := http.Client{Transport: tr}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			log.Infof("The router `%v` has been deployed successfully.", url)

			resp.Body.Close()

			return nil
		}

		// Sleep for a second to continue the next ping.
		log.Infof("Waiting for the router `%v`, retry in 1 second.", url)
		time.Sleep(1 * time.Second)

		select {
		case <-ctx.Done():
			return fmt.Errorf("the router %v has no response, or it might took too long to start up", url)
		default:
		}
	}
}
