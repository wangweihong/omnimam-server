package profiling

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"sync"
	"time"
)

var (
	profilingLock   sync.Mutex
	profilingServer *http.Server
)

func updateProfilingServer(server *http.Server) {
	profilingLock.Lock()
	defer profilingLock.Unlock()

	profilingServer = server
}

func getProfilingServer() *http.Server {
	profilingLock.Lock()
	defer profilingLock.Unlock()

	return profilingServer
}

func StartProfilingServer(address string) error {
	if address == "" {
		return fmt.Errorf("invalid address %v", address)
	}

	if getProfilingServer() != nil {
		return fmt.Errorf("profiling server is running:%v", getProfilingServer().Addr)
	}

	retChan := make(chan error)
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		updateProfilingServer(&http.Server{Addr: address})
		if err := getProfilingServer().ListenAndServe(); err != nil {
			updateProfilingServer(nil)
			retChan <- err
			return
		}
	}()

	select {
	case <-time.After(5 * time.Second):
		return nil
	case err := <-retChan:
		return err
	}
}

func StopProfilingServer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if getProfilingServer() != nil {
		err := getProfilingServer().Shutdown(ctx)
		updateProfilingServer(nil)
		return err
	}
	return nil
}
