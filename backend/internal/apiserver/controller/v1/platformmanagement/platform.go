package platformmanagement

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	platformsvc "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platformmanagement"
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
	core.Run(ctx, &iapiserver.PlatformAuthConfigReplaceRequest{}, func(req *iapiserver.PlatformAuthConfigReplaceRequest) (any, error) {
		return c.service.ReplaceAuthConfig(ctx, req)
	})
}
func (c *Controller) ListAudit(ctx *gin.Context) {
	req := &iapiserver.PlatformAuditLogListRequest{}
	core.Run(ctx, req, func(r *iapiserver.PlatformAuditLogListRequest) (any, error) { return c.service.ListAudit(ctx, r) })
}
func (c *Controller) GetAudit(ctx *gin.Context) {
	core.Run(ctx, nil, func(any) (any, error) { return c.service.GetAudit(ctx, ctx.Param("audit_log_id")) })
}
func (c *Controller) AppendAudit(ctx *gin.Context) {
	core.Run(ctx, &iapiserver.PlatformAuditRecordRequest{}, func(req *iapiserver.PlatformAuditRecordRequest) (any, error) { return c.service.AppendAudit(ctx, req) })
}
