package decode

import (
	"encoding/json"
	"encoding/xml"
	"mime"
	"strings"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

const (
	ApplicationXml  = "application/xml"
	ApplicationJson = "application/json"
	ContentType     = "Content-Type"
)

// UnmarshalFunc implements manifest unmarshalling a given MediaType.
type UnmarshalFunc func([]byte, any) error

// NewMarshalMapping create a new MarshalMapping.
func NewMarshalMapping() *MarshalMapping {
	return &MarshalMapping{
		mappings: make(map[string]UnmarshalFunc),
	}
}

func NewDefaultMarshalMapping() *MarshalMapping {
	mm := &MarshalMapping{
		mappings: make(map[string]UnmarshalFunc),
	}
	// 核心JSON解析器
	_ = mm.Register("application/json", json.Unmarshal)

	// Docker镜像格式支持
	_ = mm.Register("application/vnd.docker.distribution.manifest.v1+json", json.Unmarshal)
	_ = mm.Register("application/vnd.docker.distribution.manifest.v2+json", json.Unmarshal)
	_ = mm.Register("application/vnd.docker.distribution.manifest.list.v2+json", json.Unmarshal)

	// 其他格式支持
	_ = mm.Register("application/xml", xml.Unmarshal)
	_ = mm.Register("text/xml", xml.Unmarshal)
	return mm
}

type MarshalMapping struct {
	lock     sync.RWMutex
	mappings map[string]UnmarshalFunc
}

// ManifestMediaTypes returns the supported media types for manifests.
func (mm *MarshalMapping) ManifestMediaTypes() (mediaTypes []string) {
	mm.lock.RLock()
	defer mm.lock.RUnlock()

	for t := range mm.mappings {
		if t != "" {
			mediaTypes = append(mediaTypes, t)
		}
	}
	return
}

// ManifestMediaTypes returns the supported media types for manifests.
func (mm *MarshalMapping) Register(mediaType string, u UnmarshalFunc) error {
	mm.lock.Lock()
	defer mm.lock.Unlock()

	if _, ok := mm.mappings[mediaType]; ok {
		return errors.Errorf("manifest media type registration would overwrite existing: %s", mediaType)
	}
	mm.mappings[mediaType] = u
	return nil
}

// UnmarshalManifest looks up manifest unmarshal functions based on MediaType.
func (mm *MarshalMapping) UnmarshalManifest(ctHeader string, p []byte, arg any) error {
	mm.lock.Lock()
	defer mm.lock.Unlock()

	var mediaType string
	if ctHeader != "" {
		var err error
		// 相对于直接读取Content-Type
		// mime.ParseMediaType会解析Content-Type并拆分成主类型和可选参数
		// "application/json;
		// charset=utf-8",mime.ParseMediaType将返回contentType为"application/json"，params为map[string]string{"charset":
		// "utf-8"}
		mediaType, _, err = mime.ParseMediaType(ctHeader)
		if err != nil {
			return err
		}
	}

	unmarshalFunc, ok := mm.mappings[mediaType]
	if !ok {
		return errors.Errorf("unsupported Content-Type %v", mediaType)
	}

	return unmarshalFunc(p, arg)
}

func IsJsonBased(contentType string) bool {
	jsonTypes := []string{
		"application/json",
		"application/vnd.docker.distribution.manifest.v1+json",
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
	}

	for _, t := range jsonTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}

func IsXmlBased(contentType string) bool {
	jsonTypes := []string{
		"application/xml",
		"text/xml",
	}

	for _, t := range jsonTypes {
		if strings.Contains(contentType, t) {
			return true
		}
	}
	return false
}
