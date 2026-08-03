package infrastructure

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type Server struct {
	service *Service
	token   string
}

func NewServer(service *Service, token string) (*Server, error) {
	if service == nil {
		return nil, fmt.Errorf("infrastructure service is required")
	}
	if len(token) < 32 {
		return nil, fmt.Errorf("infrastructure service token must be at least 32 bytes")
	}
	return &Server{service: service, token: token}, nil
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/internal/v1/execute", s.execute)
	return mux
}
func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(CommandResponse{Error: "unauthorized"})
		return
	}
	var command CommandRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&command); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CommandResponse{Error: "invalid request"})
		return
	}
	result, err := s.dispatch(r.Context(), &command)
	if err != nil {
		_ = json.NewEncoder(w).Encode(CommandResponse{Error: err.Error()})
		return
	}
	_ = json.NewEncoder(w).Encode(CommandResponse{Result: result})
}
func (s *Server) dispatch(ctx context.Context, c *CommandRequest) (*iapiserver.InfraOperationResult, error) {
	switch c.Operation {
	case "create":
		if c.Create == nil {
			return nil, fmt.Errorf("create request is required")
		}
		return s.service.CreateRuntime(ctx, c.Create)
	case "start":
		return s.service.Start(ctx, c.RuntimeID)
	case "stop":
		return s.service.Stop(ctx, c.RuntimeID, c.Delete)
	case "cancel":
		return s.service.Cancel(ctx, c.RuntimeID)
	case "reconcile":
		return s.service.Reconcile(ctx, c.RuntimeID)
	default:
		return nil, fmt.Errorf("unsupported infrastructure operation")
	}
}
