package setting

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

// IdentityProviderSAMLMetadataUpsert 生成或者更新当前服务的Identity Provider SAML元数据文件
// 只有在当前服务作为SSO的Identity Provider才有意义。
// 1. 第三方SP需要下载当前服务的元数据文件
// 2. 当前服务需要添加第三方SP作为SSO Service Provider(即获取sp的元数据文件)
// 3. 第三方sp执行单点登录时会根据元数据文件的url跳转到idp来验证
// 4. 验证通过后再根据sp元数据文件的url跳转回去
func (rc *SettingController) IdentityProviderSAMLMetadataUpsert(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.IdentityProviderMetadataUpsetRequest{},
		func(r *iapiserver.IdentityProviderMetadataUpsetRequest) (any, error) {
			meta, err := rc.srv.Settings().IdentityProviderSAMLMetadataUpsert(c, r)
			return meta, err
		},
	)
}

func (rc *SettingController) IdentityProviderSAMLMetadataGet(c *gin.Context) {
	meta, err := rc.srv.Settings().IdentityProviderSAMLMetadataGet(c)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	core.WriteResponse(c, err, meta)
}

func (rc *SettingController) IdentityProviderSAMLMetadataDownload(c *gin.Context) {
	meta, err := rc.srv.Settings().IdentityProviderSAMLMetadataGet(c)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	filename := "metadata.xml"
	// 3. 设置HTTP响应头·
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(len(meta.XML)))

	c.Data(200, "application/octet-stream", []byte(meta.XML))
}

// ServiceProviderSAMLMetadataUpsert 生成或者更新当前服务的Service Provider SAML元数据文件
// 只有在当前服务作为SSO的Service Provider才有意义。
// 1. 下载第三方Idp的元数据文件
// 2. 添加第三方Idp作为SSO Identity Provider
// 3. 生成当前服务的Service Provider Metadata XML
// 4. 在第三方Idp平台进行注册
func (rc *SettingController) ServiceProviderSAMLMetadataUpsert(c *gin.Context) {
	core.Run(
		c,
		&iapiserver.ServiceProviderMetadataUpsetRequest{},
		func(r *iapiserver.ServiceProviderMetadataUpsetRequest) (any, error) {
			meta, err := rc.srv.Settings().ServiceProviderSAMLMetadataUpsert(c, r)
			return meta, err
		},
	)
}

func (rc *SettingController) ServiceProviderSAMLMetadataGet(c *gin.Context) {
	meta, err := rc.srv.Settings().ServiceProviderSAMLMetadataGet(c)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	core.WriteResponse(c, err, meta)
}

func (rc *SettingController) ServiceProviderSAMLMetadataDownload(c *gin.Context) {
	meta, err := rc.srv.Settings().ServiceProviderSAMLMetadataGet(c)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	filename := "metadata.xml"
	// 3. 设置HTTP响应头·
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.Itoa(len(meta.XML)))

	c.Data(200, "application/octet-stream", []byte(meta.XML))
}
