package setting

import (
	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

func (rc *SettingController) IdentityProviderList(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.IdentityProviderListRequest{},
		func(r *iapiserver.IdentityProviderListRequest) (any, error) {
			ret, err := rc.srv.Settings().IdentityProviderList(c, r)
			return ret, err
		},
	)
}

func (rc *SettingController) IdentityProviderGet(c *gin.Context) {
	core.Run(c, &iapiserver.IdentityProviderGetRequest{}, func(r *iapiserver.IdentityProviderGetRequest) (any, error) {
		ret, err := rc.srv.Settings().IdentityProviderGet(c, r)
		return ret, err
	})
}

func (rc *SettingController) IdentityProviderAdd(c *gin.Context) {
	core.Run(c, &iapiserver.IdentityProviderAddRequest{}, func(r *iapiserver.IdentityProviderAddRequest) (any, error) {
		ret, err := rc.srv.Settings().IdentityProviderAdd(c, r)
		return ret, err
	})

}

func (rc *SettingController) IdentityProviderUpdate(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.IdentityProviderUpdateRequest{},
		func(r *iapiserver.IdentityProviderUpdateRequest) (any, error) {
			err := rc.srv.Settings().IdentityProviderUpdate(c, r)
			return nil, err
		},
	)
}

func (rc *SettingController) IdentityProviderDelete(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.IdentityProviderDeleteRequest{},
		func(r *iapiserver.IdentityProviderDeleteRequest) (any, error) {
			err := rc.srv.Settings().IdentityProviderDelete(c, r)
			return nil, err
		},
	)
}
