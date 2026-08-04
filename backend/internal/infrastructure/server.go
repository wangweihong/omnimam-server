package infrastructure

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	toolerrors "github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/validator"
)

const (
	infraAPIBasePath = "/api/v1/infra"
	maxRequestBytes  = 1 << 20
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

// Handler 返回 infra-server 的健康检查、内部命令和已发布 Infrastructure API。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/internal/v1/execute", s.execute)

	infraMux := http.NewServeMux()
	s.registerInfrastructureRoutes(infraMux)
	mux.Handle(infraAPIBasePath+"/", s.authenticate(http.StripPrefix(infraAPIBasePath, infraMux)))
	return mux
}

func (s *Server) registerInfrastructureRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /runtimes", s.respond(func(r *http.Request) (any, error) {
		req, err := runtimeListRequest(r)
		if err != nil {
			return nil, err
		}
		return s.service.ListRuntimes(r.Context(), req)
	}))
	mux.HandleFunc("POST /runtimes", s.respond(func(r *http.Request) (any, error) {
		var req iapiserver.InfraCreateRuntimeRequest
		if err := decodeInfraRequest(r, &req, true); err != nil {
			return nil, err
		}
		return runtimeOperation(s.service.CreateRuntime(r.Context(), &req))
	}))
	mux.HandleFunc("GET /runtimes/{runtime_id}", s.respond(func(r *http.Request) (any, error) {
		return s.service.GetRuntime(r.Context(), r.PathValue("runtime_id"))
	}))
	mux.HandleFunc("DELETE /runtimes/{runtime_id}", s.respond(func(r *http.Request) (any, error) {
		if err := decodeActionRequest(r); err != nil {
			return nil, err
		}
		return s.service.Delete(r.Context(), r.PathValue("runtime_id"))
	}))
	mux.HandleFunc("POST /runtimes/{runtime_id}/start", s.respond(func(r *http.Request) (any, error) {
		if err := decodeActionRequest(r); err != nil {
			return nil, err
		}
		return runtimeOperation(s.service.Start(r.Context(), r.PathValue("runtime_id")))
	}))
	mux.HandleFunc("POST /runtimes/{runtime_id}/stop", s.respond(func(r *http.Request) (any, error) {
		if err := decodeActionRequest(r); err != nil {
			return nil, err
		}
		return runtimeOperation(s.service.Stop(r.Context(), r.PathValue("runtime_id"), false))
	}))
	mux.HandleFunc("POST /runtimes/{runtime_id}/cancel", s.respond(func(r *http.Request) (any, error) {
		if err := decodeActionRequest(r); err != nil {
			return nil, err
		}
		return runtimeOperation(s.service.Cancel(r.Context(), r.PathValue("runtime_id")))
	}))
	mux.HandleFunc("POST /runtimes/{runtime_id}/reconcile", s.respond(func(r *http.Request) (any, error) {
		if err := decodeActionRequest(r); err != nil {
			return nil, err
		}
		return runtimeOperation(s.service.Reconcile(r.Context(), r.PathValue("runtime_id")))
	}))
	mux.HandleFunc("GET /runtimes/{runtime_id}/endpoint", s.respond(func(r *http.Request) (any, error) {
		return s.service.GetEndpoint(r.Context(), r.PathValue("runtime_id"))
	}))
	mux.HandleFunc("GET /runtimes/{runtime_id}/logs", s.respond(func(r *http.Request) (any, error) {
		req, err := basicListRequest(r, false)
		if err != nil {
			return nil, err
		}
		return s.service.Logs(r.Context(), r.PathValue("runtime_id"), req)
	}))
	mux.HandleFunc("GET /nodes", s.respond(func(r *http.Request) (any, error) {
		req, err := basicListRequest(r, true)
		if err != nil {
			return nil, err
		}
		return s.service.ListNodes(r.Context(), req)
	}))
	mux.HandleFunc("GET /nodes/{node_id}", s.respond(func(r *http.Request) (any, error) {
		return s.service.GetNode(r.Context(), r.PathValue("node_id"))
	}))
	mux.HandleFunc("GET /runtime-profiles", s.respond(func(r *http.Request) (any, error) {
		req := &iapiserver.InfraBasicListRequest{}
		if err := validateInfraRequest(req); err != nil {
			return nil, err
		}
		return s.service.ListProfiles(r.Context(), req)
	}))
	mux.HandleFunc("GET /runtime-profiles/{profile_id}", s.respond(func(r *http.Request) (any, error) {
		return s.service.GetProfile(r.Context(), r.PathValue("profile_id"))
	}))
	mux.HandleFunc("GET /runtimes/{runtime_id}/outputs", s.respond(func(r *http.Request) (any, error) {
		req, err := basicListRequest(r, false)
		if err != nil {
			return nil, err
		}
		return s.service.ListOutputs(r.Context(), r.PathValue("runtime_id"), req)
	}))
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			writeInfraError(w, toolerrors.NewStatus(code.ErrTokenInvalid, "service bearer token is invalid"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	provided := strings.TrimPrefix(header, prefix)
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) == 1
}

func (s *Server) respond(action func(*http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := action(r)
		if err != nil {
			writeInfraError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func runtimeOperation(result *iapiserver.InfraOperationResult, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if result == nil || result.Runtime == nil {
		return nil, fmt.Errorf("infrastructure operation returned no runtime")
	}
	return result.Runtime, nil
}

func runtimeListRequest(r *http.Request) (*iapiserver.InfraRuntimeListRequest, error) {
	query := r.URL.Query()
	req := &iapiserver.InfraRuntimeListRequest{
		Status:         query.Get("status"),
		OwnerDomain:    query.Get("owner_domain"),
		OwnerReference: query.Get("owner_reference"),
	}
	if err := parseInfraPaging(query.Get("page_num"), query.Get("page_size"), &req.PagingParams); err != nil {
		return nil, err
	}
	if err := validateInfraRequest(req); err != nil {
		return nil, err
	}
	return req, nil
}

func basicListRequest(r *http.Request, withStatus bool) (*iapiserver.InfraBasicListRequest, error) {
	query := r.URL.Query()
	req := &iapiserver.InfraBasicListRequest{}
	if withStatus {
		req.Status = query.Get("status")
	}
	if err := parseInfraPaging(query.Get("page_num"), query.Get("page_size"), &req.PagingParams); err != nil {
		return nil, err
	}
	if err := validateInfraRequest(req); err != nil {
		return nil, err
	}
	return req, nil
}

func parseInfraPaging(pageNum, pageSize string, params *imachinery.PagingParams) error {
	if pageNum != "" {
		parsed, err := strconv.Atoi(pageNum)
		if err != nil {
			return infraRequestError("page_num must be an integer")
		}
		params.PageNum = parsed
	}
	if pageSize != "" {
		parsed, err := strconv.Atoi(pageSize)
		if err != nil {
			return infraRequestError("page_size must be an integer")
		}
		params.PageSize = parsed
	}
	return nil
}

func decodeActionRequest(r *http.Request) error {
	var req iapiserver.InfraActionRequest
	return decodeInfraRequest(r, &req, false)
}

func decodeInfraRequest(r *http.Request, target any, required bool) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxRequestBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if err == io.EOF && !required {
			return nil
		}
		return infraRequestError(err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return infraRequestError("request body must contain a single JSON object")
	}
	return validateInfraRequest(target)
}

func validateInfraRequest(target any) error {
	if err := validator.ValidateAll(target); err != nil {
		return infraRequestError(err.Error())
	}
	if custom, ok := target.(interface{ Validate() error }); ok {
		if err := custom.Validate(); err != nil {
			return infraRequestError(err.Error())
		}
	}
	return nil
}

func infraRequestError(message string) error {
	return toolerrors.NewStatus(code.ErrInfraRequestInvalid, message)
}

type infraErrorResponse struct {
	Code      string         `json:"code"`
	Value     int            `json:"value"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Data      map[string]any `json:"data,omitempty"`
}

func writeInfraError(w http.ResponseWriter, err error) {
	status := toolerrors.ToStatus(err)
	symbol, known := infraErrorSymbols[status.Code]
	if !known {
		status = toolerrors.ToStatus(toolerrors.NewStatus(code.ErrUnknown, "unexpected infrastructure error"))
		symbol = "ERR_UNKNOWN"
	}
	message := status.Message[toolerrors.MessageLangENKey]
	writeJSON(w, status.HTTPStatus, infraErrorResponse{
		Code:      symbol,
		Value:     status.Code,
		Message:   message,
		Retryable: infraRetryableErrors[status.Code],
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

var infraErrorSymbols = map[int]string{
	code.ErrTokenInvalid:                  "ERR_TOKEN_INVALID",
	code.ErrUnknown:                       "ERR_UNKNOWN",
	code.ErrInfraRequestInvalid:           "ERR_INFRA_REQUEST_INVALID",
	code.ErrInfraRuntimeProfileNotFound:   "ERR_INFRA_RUNTIME_PROFILE_NOT_FOUND",
	code.ErrInfraUnsupportedRuntimeMode:   "ERR_INFRA_UNSUPPORTED_RUNTIME_MODE",
	code.ErrInfraIdempotencyConflict:      "ERR_INFRA_IDEMPOTENCY_CONFLICT",
	code.ErrInfraNoEligibleNode:           "ERR_INFRA_NO_ELIGIBLE_NODE",
	code.ErrInfraResourceInsufficient:     "ERR_INFRA_RESOURCE_INSUFFICIENT",
	code.ErrInfraRuntimeNotFound:          "ERR_INFRA_RUNTIME_NOT_FOUND",
	code.ErrInfraRuntimeOperationFailed:   "ERR_INFRA_RUNTIME_OPERATION_FAILED",
	code.ErrInfraRuntimeStateConflict:     "ERR_INFRA_RUNTIME_STATE_CONFLICT",
	code.ErrInfraMountNotAllowed:          "ERR_INFRA_MOUNT_NOT_ALLOWED",
	code.ErrInfraSecretResolutionFailed:   "ERR_INFRA_SECRET_RESOLUTION_FAILED",
	code.ErrInfraEndpointAllocationFailed: "ERR_INFRA_ENDPOINT_ALLOCATION_FAILED",
	code.ErrInfraEndpointAccessDenied:     "ERR_INFRA_ENDPOINT_ACCESS_DENIED",
}

var infraRetryableErrors = map[int]bool{
	code.ErrInfraNoEligibleNode:           true,
	code.ErrInfraResourceInsufficient:     true,
	code.ErrInfraRuntimeOperationFailed:   true,
	code.ErrInfraRuntimeStateConflict:     true,
	code.ErrInfraSecretResolutionFailed:   true,
	code.ErrInfraEndpointAllocationFailed: true,
}

func (s *Server) execute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(CommandResponse{Error: "unauthorized"})
		return
	}
	var command CommandRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(&command); err != nil {
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
