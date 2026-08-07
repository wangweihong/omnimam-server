package v1

import (
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/aichat"
	appplatform "github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/applicationplatform"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/asset"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/prompt"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/service/v1/taskcenter"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
)

// Service defines functions used to return resource interface.
type Service interface {
	Assets() asset.AssetSrv
	Prompts() prompt.PromptSrv
	Platforms() appplatform.PlatformSrv
	TaskCenters() taskcenter.TaskCenterSrv
	AIChat() aichat.AIChatSrv
}

type service struct {
	store              store.Factory
	platformDispatcher appplatform.TaskDispatcher
}

// NewService returns Service interface.
func NewService(store store.Factory, dispatcher ...appplatform.TaskDispatcher) Service {
	service := &service{store: store}
	if len(dispatcher) > 0 {
		service.platformDispatcher = dispatcher[0]
	}
	return service
}

func (s *service) Assets() asset.AssetSrv {
	return asset.NewService(s.store)
}

func (s *service) Prompts() prompt.PromptSrv {
	return prompt.NewService(s.store)
}

func (s *service) Platforms() appplatform.PlatformSrv {
	return appplatform.NewLegacyService(s.store, s.platformDispatcher)
}

func (s *service) TaskCenters() taskcenter.TaskCenterSrv {
	return taskcenter.NewService(s.store)
}

func (s *service) AIChat() aichat.AIChatSrv {
	return aichat.NewService(aichat.Dependencies{Store: s.store, ModelReader: appplatform.NewLegacyService(s.store)})
}
