package setting

import (
	"context"
	"encoding/base64"

	gx509 "github.com/wangweihong/gotoolbox/pkg/certificate/x509"
	"github.com/wangweihong/gotoolbox/pkg/decoder"
	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func (s *settingService) IdentityProviderSAMLMetadataUpsert(
	ctx context.Context,
	req *iapiserver.IdentityProviderMetadataUpsetRequest,
) (*iapiserver.Setting, error) {
	keyEncode := base64.StdEncoding.EncodeToString(req.KeyEncode)
	certEncode := base64.StdEncoding.EncodeToString(req.CertEncode)

	if _, _, err := gx509.ParseCert(certEncode, keyEncode); err != nil {
		return nil, errors.Errorf("invalid certificate:%v", err)
	}

	data := iapiserver.Setting{
		ObjectMeta: imachinery.ObjectMeta{
			Name: iapiserver.SettingKindSSOSamlIdpMetadata,
			Extend: map[string]any{
				"key":                       keyEncode,
				"cert":                      certEncode,
				"endpoint":                  req.Endpoint,
				"authn_name_id_format":      req.AuthnNameIDFormat,
				"redirect_sso_frontend_url": req.RedirectSSOFrontendURL,
			},
		},
	}

	meta, err := s.store.Settings().Upsert(ctx, &data)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return meta, nil
}

func (s *settingService) IdentityProviderSAMLMetadataGet(
	ctx context.Context,
) (*iapiserver.IdentityProviderMetadataGetResponse, error) {
	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlIdpMetadata)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	xmlData, err := xmlIdpSamlGenerate(meta)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.IdentityProviderMetadataGetResponse{
		Setting:    meta,
		XML:        xmlData,
		DecodeKey:  string(decoder.MustBase64Decode(iapiserver.IdentityProviderMetadataExtender(*meta).GetKey())),
		DecodeCert: string(decoder.MustBase64Decode(iapiserver.IdentityProviderMetadataExtender(*meta).GetCert())),
	}, nil
}

func (s *settingService) ServiceProviderSAMLMetadataUpsert(
	ctx context.Context,
	req *iapiserver.ServiceProviderMetadataUpsetRequest,
) (*iapiserver.Setting, error) {
	keyEncode := base64.StdEncoding.EncodeToString(req.KeyEncode)
	certEncode := base64.StdEncoding.EncodeToString(req.CertEncode)

	if _, _, err := gx509.ParseCert(certEncode, keyEncode); err != nil {
		return nil, errors.Errorf("invalid certificate:%v", err)
	}

	data := iapiserver.Setting{
		ObjectMeta: imachinery.ObjectMeta{
			Name: iapiserver.SettingKindSSOSamlSpMetadata,
			Extend: map[string]any{
				"key":                  keyEncode,
				"cert":                 certEncode,
				"endpoint":             req.Endpoint,
				"authn_name_id_format": req.AuthnNameIDFormat,
			},
		},
	}

	meta, err := s.store.Settings().Upsert(ctx, &data)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return meta, nil
}

func (s *settingService) ServiceProviderSAMLMetadataGet(
	ctx context.Context,
) (*iapiserver.ServiceProviderMetadataGetResponse, error) {
	meta, err := s.store.Settings().GetByName(ctx, iapiserver.SettingKindSSOSamlSpMetadata)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	xmlData, err := xmlSpSamlGenerate(meta)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &iapiserver.ServiceProviderMetadataGetResponse{
		Setting:    meta,
		XML:        xmlData,
		DecodeKey:  string(decoder.MustBase64Decode(iapiserver.IdentityProviderMetadataExtender(*meta).GetKey())),
		DecodeCert: string(decoder.MustBase64Decode(iapiserver.IdentityProviderMetadataExtender(*meta).GetCert())),
	}, nil
}
