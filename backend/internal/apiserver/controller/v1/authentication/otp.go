package authentication

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/pkg/core"
)

func (rc *AuthController) OTPGenerateOrGet(c *gin.Context) {
	r := &iapiserver.OTPGenerateRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	qrCodeURL, err := rc.srv.Identities().UserOTPGetOrAdd(c, r)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	png, err := qrcode.Encode(qrCodeURL, qrcode.Medium, 256)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	// 直接返回二维码图片
	c.Data(http.StatusOK, "image/png", png)
}

func (rc *AuthController) OTPValidate(c *gin.Context) {
	r := &iapiserver.OTPValidateRequest{}
	if err := core.DecodeParameter(c, r); err != nil {
		core.WriteResponse(c, err, nil)
		return
	}

	meta, err := rc.srv.Identities().UserOTPGet(c, r.UserID)
	if err != nil {
		core.WriteResponse(c, err, nil)
		return
	}
	// 验证OTP
	valid := totp.Validate(r.OTP, meta.Secret)
	if !valid {
		// 尝试在前后时间窗口中验证
		valid, err = totp.ValidateCustom(r.OTP, meta.Secret, time.Now(), totp.ValidateOpts{
			Period:    30,
			Skew:      1,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}
	response := iapiserver.OTPValidateResponse{
		Status: "验证失败",
	}
	if valid {
		response.Status = "验证成功"
	}
	core.WriteResponse(c, err, response)
}
