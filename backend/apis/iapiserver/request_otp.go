package iapiserver

type (
	OTPValidateRequest struct {
		UserID string `json:"user_id" binding:"required"`
		OTP    string `json:"otp"     binding:"required"`
	}

	OTPValidateResponse struct {
		Status string `json:"status,omitempty"`
	}
)

type (
	OTPGenerateRequest struct {
		UserID string `json:"user_id" binding:"required"`
	}

	OTPGenerateResponse struct {
		Secret string `json:"secret,omitempty"`
		URL    string `json:"url,omitempty"`
		QRCode string `json:"qrcode,omitempty"`
		Status string `json:"status,omitempty"`
		Error  string `json:"error,omitempty"`
	}
)
