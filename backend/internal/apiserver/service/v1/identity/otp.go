package identity

import (
	"context"
	"fmt"

	gerrors "errors"

	"github.com/pquerna/otp/totp"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

func (s *identityService) UserOTPGetOrAdd(ctx context.Context, req *iapiserver.OTPGenerateRequest) (string, error) {
	var secret string
	issuer := "Your Service"
	meta, err := s.store.UserOTPs().GetByUser(ctx, req.UserID)
	if err != nil {
		if !gerrors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.WithStack(err)
		}

		// 生成新密钥
		key, err := totp.Generate(totp.GenerateOpts{
			Issuer:      issuer,
			AccountName: req.UserID,
		})
		if err != nil {
			return "", errors.WithStack(err)
		}
		secret = key.Secret()
		// 保存到数据库
		newOTP := iapiserver.UserOTP{
			UserID: req.UserID,
			Secret: secret,
		}
		if _, err := s.store.UserOTPs().Add(ctx, &newOTP); err != nil {
			return "", errors.WithStack(err)
		}
	} else {
		secret = meta.Secret
	}
	qrCodeURL := fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, req.UserID, secret, issuer)
	return qrCodeURL, nil
}

func (s *identityService) UserOTPGet(ctx context.Context, userid string) (*iapiserver.UserOTP, error) {
	meta, err := s.store.UserOTPs().GetByUser(ctx, userid)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return meta, nil
}
