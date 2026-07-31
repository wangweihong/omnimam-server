package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/internal/pkg/code"
)

// Invoke 执行带 EngineInstance 认证与超时约束的 JSON Provider 请求。
func Invoke(ctx context.Context, engine *iapiserver.EngineInstance, method, requestPath string, payload any) (map[string]any, error) {
	if engine == nil || strings.TrimSpace(engine.BaseURL) == "" {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "engine base URL is required")
	}
	endpoint := strings.TrimRight(engine.BaseURL, "/") + "/" + strings.TrimLeft(requestPath, "/")
	builder := httpcli.NewHttpRequestBuilder().WithEndpoint(endpoint).WithMethod(method).AddHeaderParam("Accept", "application/json")
	var raw json.RawMessage
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, errors.NewStatus(code.ErrAIAppApplicationInputInvalid, "provider request could not be encoded")
		}
		raw = encoded
		builder.WithBody("json", raw).AddHeaderParam("Content-Type", "application/json")
	}
	if err := ApplyAuthentication(builder, engine, method, requestPath, raw); err != nil {
		return nil, err
	}
	timeout := time.Duration(engine.RequestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	response, err := builder.Build().InvokeWithContext(ctx, httpcli.TimeoutCallOption(timeout))
	if err != nil {
		if stderrors.Is(err, context.DeadlineExceeded) || stderrors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request timed out")
		}
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider request failed")
	}
	body := response.GetBody()
	if response.GetStatusCode() < http.StatusOK || response.GetStatusCode() >= http.StatusMultipleChoices {
		switch response.GetStatusCode() {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "provider authentication was rejected")
		case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity:
			return nil, errors.NewStatus(code.ErrAIAppProviderRuntimeCapabilityMismatch, failureSummary(body))
		default:
			return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, failureSummary(body))
		}
	}
	if strings.TrimSpace(body) == "" {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		return nil, errors.NewStatus(code.ErrAIAppEngineUnavailable, "provider response could not be parsed")
	}
	return decoded, nil
}

// ApplyAuthentication 将 EngineInstance 的鉴权联合类型映射到 Provider 请求头或签名。
func ApplyAuthentication(builder *httpcli.HttpRequestBuilder, engine *iapiserver.EngineInstance, method, requestPath string, body []byte) error {
	switch engine.AuthType {
	case "none", "":
		return nil
	case "api_key":
		apiKey, _ := engine.AuthConfig["api_key"].(string)
		if apiKey == "" {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "api_key is required")
		}
		if engine.ApplicationEngineTypeID == "comfyui" {
			builder.AddHeaderParam("X-API-Key", apiKey)
		} else {
			builder.AddHeaderParam("Authorization", "Bearer "+apiKey)
		}
		return nil
	case "bearer_token":
		token, _ := engine.AuthConfig["bearer_token"].(string)
		if token == "" {
			return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "bearer_token is required")
		}
		builder.AddHeaderParam("Authorization", "Bearer "+token)
		return nil
	case "ak_sk":
		return applyModelArkSignature(builder, engine, method, requestPath, body, time.Now().UTC())
	default:
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "engine authentication type is unsupported")
	}
}

func applyModelArkSignature(builder *httpcli.HttpRequestBuilder, engine *iapiserver.EngineInstance, method, requestPath string, body []byte, now time.Time) error {
	accessKey, _ := engine.AuthConfig["access_key"].(string)
	secretKey, _ := engine.AuthConfig["secret_key"].(string)
	if accessKey == "" || secretKey == "" {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "access_key and secret_key are required")
	}
	parsed, err := url.Parse(engine.BaseURL)
	if err != nil || parsed.Host == "" {
		return errors.NewStatus(code.ErrAIAppEngineAuthConfigInvalid, "engine base URL is invalid")
	}
	region := engine.Region
	if region == "" {
		region = "ap-southeast-1"
	}
	date, xDate := now.Format("20060102"), now.Format("20060102T150405Z")
	bodyHash := sha256Hex(body)
	canonicalURI := path.Clean("/" + strings.TrimLeft(requestPath, "/"))
	signedHeaders := "content-type;host;x-content-sha256;x-date"
	canonicalHeaders := "content-type:application/json\n" + "host:" + parsed.Host + "\n" + "x-content-sha256:" + bodyHash + "\n" + "x-date:" + xDate + "\n"
	canonicalRequest := method + "\n" + canonicalURI + "\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + bodyHash
	scope := date + "/" + region + "/ark/request"
	stringToSign := "HMAC-SHA256\n" + xDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	kDate := hmacSHA256([]byte(secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, "ark")
	kSigning := hmacSHA256(kService, "request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	builder.AddHeaderParam("Host", parsed.Host)
	builder.AddHeaderParam("Content-Type", "application/json")
	builder.AddHeaderParam("X-Date", xDate)
	builder.AddHeaderParam("X-Content-Sha256", bodyHash)
	builder.AddHeaderParam("Authorization", fmt.Sprintf("HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", accessKey, scope, signedHeaders, signature))
	return nil
}

func failureSummary(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 512 {
		body = body[:512]
	}
	if body == "" {
		return "provider request was rejected"
	}
	return body
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
