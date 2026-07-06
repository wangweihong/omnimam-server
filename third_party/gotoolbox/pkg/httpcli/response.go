package httpcli

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli/decode"
)

type HttpResponse struct {
	Response *http.Response
	Request  *HttpRequest
}

func NewHttpResponse(req *HttpRequest, response *http.Response) *HttpResponse {
	return &HttpResponse{
		Response: response,
		Request:  req,
	}
}

func (r *HttpResponse) GetStatusCode() int {
	return r.Response.StatusCode
}

func (r *HttpResponse) GetStatus() string {
	return r.Response.Status
}

func (r *HttpResponse) GetHeaders() map[string]string {
	headerParams := map[string]string{}
	for key, values := range r.Response.Header {
		if values == nil || len(values) <= 0 {
			continue
		}
		headerParams[key] = values[0]
	}
	return headerParams
}

func (r *HttpResponse) GetBody() string {
	body, err := io.ReadAll(r.Response.Body)
	if err != nil {
		return ""
	}

	// 1. 将HTTP响应主体替换成buffer, 读取HTTP响应主题数据后，关闭HTTP响应主体避免泄露
	// 2. 其次允许多次读取响应主体的内容
	if err := r.Response.Body.Close(); err == nil {
		r.Response.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	return string(body)
}

func (r *HttpResponse) GetHeader(key string) string {
	header := r.Response.Header
	return header.Get(key)
}

func (r *HttpResponse) Decode(resp any) error {
	if resp == nil {
		return nil
	}

	data := r.GetBody()
	if data == "" {
		return errors.New("body data is empty")
	}

	ct := r.GetHeader(decode.ContentType)
	// mm := decode.NewMarshalMapping()
	// _ = mm.Register(decode.ApplicationJson, json.Unmarshal)
	// _ = mm.Register(decode.ApplicationXml, xml.Unmarshal)
	// err := mm.UnmarshalManifest(r.GetHeader(decode.ContentType), byteData, resp)

	parser, err := globalParserFactory.GetParser(ct)
	if err != nil {
		if decode.IsJsonBased(ct) {
			return json.Unmarshal([]byte(data), resp)
		} else if decode.IsXmlBased(ct) {
			return xml.Unmarshal([]byte(data), resp)
		} else {
			return errors.WithStack(err)
		}
	}
	err = parser.Unmarshal([]byte(data), resp)
	return errors.WithStack(err)
}

func (r *HttpResponse) DownloadFile(saveDir string) error {
	fn, err := GetFileName(r.Response.Header)
	if err != nil {
		return errors.WithStack(err)
	}

	savePath := filepath.Join(saveDir, fn)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return errors.Errorf("mkdirAll %v error:%v", saveDir, err)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return errors.Errorf("create file %v error:%v", saveDir, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, bytes.NewBuffer([]byte(r.GetBody()))); err != nil {
		// 删除不完整文件
		os.Remove(savePath)
		return errors.Errorf("write file %v error:%v", saveDir, err)
	}
	return nil
}

func GetFileName(responseHeader http.Header) (string, error) {
	if cd := responseHeader.Get("Content-Disposition"); cd != "" {
		if matches := regexp.MustCompile(`filename\*?=['"]?(?:UTF-\d['"]*)?([^;\n"']*)['"]?`).FindStringSubmatch(cd); len(matches) > 1 {
			if decoded, err := url.QueryUnescape(matches[1]); err == nil {
				return decoded, nil
			}
			return matches[1], nil
		}
	}
	return "", errors.Errorf("not filename exists in response header")
}

func DownloadFile(responseHeader http.Header, body []byte, saveDir string) error {
	fn, err := GetFileName(responseHeader)
	if err != nil {
		return errors.WithStack(err)
	}

	savePath := filepath.Join(saveDir, fn)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return errors.Errorf("mkdirAll %v error:%v", saveDir, err)
	}

	file, err := os.Create(savePath)
	if err != nil {
		return errors.Errorf("create file %v error:%v", saveDir, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, bytes.NewBuffer(body)); err != nil {
		// 删除不完整文件
		os.Remove(savePath)
		return errors.Errorf("write file %v error:%v", saveDir, err)
	}
	return nil
}

var globalParserFactory decode.ParserFactory

func init() {
	globalParserFactory = decode.NewDefaultParesrFactory()
}
