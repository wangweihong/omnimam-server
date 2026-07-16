package v1

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/aichat"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/asset"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/canvas"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/identity"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/platform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/prompt"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/setting"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// Service defines functions used to return resource interface.
type Service interface {
	Settings() setting.SettingSrv
	Identities() identity.IdentitySrv
	Assets() asset.AssetSrv
	Prompts() prompt.PromptSrv
	Canvases() canvas.CanvasSrv
	Platforms() platform.PlatformSrv
	TaskCenters() taskcenter.TaskCenterSrv
	AIChat() aichat.AIChatSrv
}

type service struct {
	store store.Factory
}

// NewService returns Service interface.
func NewService(store store.Factory) Service {
	return &service{
		store: store,
	}
}

func (s *service) Settings() setting.SettingSrv {
	return setting.NewService(s.store)
}

func (s *service) Identities() identity.IdentitySrv {
	return identity.NewService(s.store)
}

func (s *service) Assets() asset.AssetSrv {
	return asset.NewService(s.store)
}

func (s *service) Prompts() prompt.PromptSrv {
	return prompt.NewService(s.store)
}

func (s *service) Canvases() canvas.CanvasSrv {
	return canvas.NewService(s.store)
}

func (s *service) Platforms() platform.PlatformSrv {
	return platform.NewService(s.store)
}

func (s *service) TaskCenters() taskcenter.TaskCenterSrv {
	return taskcenter.NewService(s.store)
}

func (s *service) AIChat() aichat.AIChatSrv {
	return aichat.NewService(s.store)
}
