package apiserver

import "github.com/wangweihong/omnimam/backend/internal/apiserver/config"

// Run runs the specified server.
func Run(cfg *config.Config, stopCh <-chan struct{}) error {
	server, err := createServer(cfg)
	if err != nil {
		return err
	}

	return server.PrepareRun().Run(stopCh)
}
