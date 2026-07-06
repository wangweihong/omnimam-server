package setting

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

func (rc *SettingController) ServiceProviderList(c *gin.Context) {
	core.Run(c, &iapiserver.ServiceProviderListRequest{}, func(r *iapiserver.ServiceProviderListRequest) (any, error) {
		ret, err := rc.srv.Settings().ServiceProviderList(c, r)
		return ret, err
	})
}

func (rc *SettingController) ServiceProviderGet(c *gin.Context) {
	core.Run(c, &iapiserver.ServiceProviderGetRequest{}, func(r *iapiserver.ServiceProviderGetRequest) (any, error) {
		ret, err := rc.srv.Settings().ServiceProviderGet(c, r)
		return ret, err
	})
}

func (rc *SettingController) ServiceProviderAdd(c *gin.Context) {
	core.Run(c, &iapiserver.ServiceProviderAddRequest{}, func(r *iapiserver.ServiceProviderAddRequest) (any, error) {
		ret, err := rc.srv.Settings().ServiceProviderAdd(c, r)
		return ret, err
	})
}

func (rc *SettingController) ServiceProviderUpdate(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.ServiceProviderUpdateRequest{},
		func(r *iapiserver.ServiceProviderUpdateRequest) (any, error) {
			err := rc.srv.Settings().ServiceProviderUpdate(c, r)
			return nil, err
		},
	)
}

func (rc *SettingController) ServiceProviderDelete(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.ServiceProviderDeleteRequest{},
		func(r *iapiserver.ServiceProviderDeleteRequest) (any, error) {
			err := rc.srv.Settings().ServiceProviderDelete(c, r)
			return nil, err
		},
	)
}

func (rc *SettingController) ServiceProviderRedirectURL(c *gin.Context) {
	core.Run(c, &iapiserver.ServiceProviderGetRequest{}, func(r *iapiserver.ServiceProviderGetRequest) (any, error) {
		ret, err := rc.srv.Settings().ServiceProviderGetRedirectURL(c, r)
		return ret, err
	})
}
