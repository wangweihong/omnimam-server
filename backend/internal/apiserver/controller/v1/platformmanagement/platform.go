package platformmanagement

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platformmanagement"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// Controller exposes the released v1.11 Platform Management overview, auth config and audit endpoints.
type Controller struct{ service *platformsvc.Service }

func NewController(service *platformsvc.Service) *Controller { return &Controller{service: service} }
func (c *Controller) Overview(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.Overview(ctx) })
}
func (c *Controller) GetAuthConfig(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAuthConfig(ctx) })
}
func (c *Controller) ReplaceAuthConfig(ctx *gin.Context) {
	req := &iapiserver.PlatformAuthConfigReplaceRequest{}
	if err := decodeStrictJSON(ctx, req, validateAuthConfigRequiredFields); err != nil {
		core.WriteResponse(ctx, errors.NewStatus(code.ErrPlatformAuthConfigInvalid, err.Error()), nil)
		return
	}
	result, err := c.service.ReplaceAuthConfig(ctx, req)
	core.WriteResponse(ctx, err, result)
}
func (c *Controller) ListAudit(ctx *gin.Context) {
	req := &iapiserver.PlatformAuditLogListRequest{}
	if err := core.DecodeParameter(ctx, req); err != nil {
		core.WriteResponse(ctx, errors.NewStatus(code.ErrPlatformAuditQueryInvalid, err.Error()), nil)
		return
	}
	result, err := c.service.ListAudit(ctx, req)
	core.WriteResponse(ctx, err, result)
}
func (c *Controller) GetAudit(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAudit(ctx, ctx.Param("audit_log_id")) })
}
func (c *Controller) AppendAudit(ctx *gin.Context) {
	req := &iapiserver.PlatformAuditRecordRequest{}
	if err := decodeStrictJSON(ctx, req, validateAuditRequiredFields); err != nil {
		core.WriteResponse(ctx, errors.NewStatus(code.ErrPlatformAuditRecordInvalid, err.Error()), nil)
		return
	}
	result, err := c.service.AppendAudit(ctx, req)
	core.WriteResponse(ctx, err, result)
}

const maxPlatformJSONBodyBytes = 64 * 1024

func decodeStrictJSON(ctx *gin.Context, target any, validateRequired func(map[string]json.RawMessage) error) error {
	raw, err := io.ReadAll(io.LimitReader(ctx.Request.Body, maxPlatformJSONBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if len(raw) > maxPlatformJSONBodyBytes {
		return fmt.Errorf("request body exceeds %d bytes", maxPlatformJSONBodyBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON object")
		}
		return fmt.Errorf("decode trailing request content: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return fmt.Errorf("request body must be a JSON object")
	}
	return validateRequired(fields)
}

func validateAuthConfigRequiredFields(fields map[string]json.RawMessage) error {
	if err := requireFields(fields, "registration_mode", "password_policy", "login_failure_policy", "online_presence_window_seconds", "access_token_lifetime", "refresh_token_lifetime", "resource_version"); err != nil {
		return err
	}
	for field, required := range map[string][]string{
		"password_policy":      {"min_length", "max_length", "require_uppercase", "require_lowercase", "require_digit", "require_special_character", "disallow_username"},
		"login_failure_policy": {"max_failed_attempts", "failure_window_seconds", "lockout_duration_seconds"},
	} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(fields[field], &nested); err != nil || nested == nil {
			return fmt.Errorf("%s must be a JSON object", field)
		}
		if err := requireFields(nested, required...); err != nil {
			return fmt.Errorf("%s: %w", field, err)
		}
	}
	return nil
}

func validateAuditRequiredFields(fields map[string]json.RawMessage) error {
	return requireFields(fields, "source_domain", "source_module", "principal_type", "action", "result", "occurred_at", "idempotency_key")
}

func requireFields(fields map[string]json.RawMessage, required ...string) error {
	for _, field := range required {
		if _, ok := fields[field]; !ok {
			return fmt.Errorf("required field %q is missing", field)
		}
	}
	return nil
}
