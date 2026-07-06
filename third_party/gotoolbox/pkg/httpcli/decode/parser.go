package decode

import (
	"encoding/json"
	"encoding/xml"

	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

func NewDefaultParesrFactory() ParserFactory {
	factory := NewParserFactory()
	factory.RegisterParser(&JsonParser{})
	factory.RegisterParser(&DockerManifestParser{})
	factory.RegisterParser(&XmlParser{})
	factory.RegisterParser(&OctetStreamParser{})
	return factory
}

type ContentParser interface {
	CanParse(contentType string) bool
	Unmarshal(data []byte, v any) error
	// 获取支持的MIME类型列表
	SupportedTypes() []string
}

// ParserFactory 解析器工厂接口
type ParserFactory interface {
	RegisterParser(parser ContentParser)
	GetParser(contentType string) (ContentParser, error)

	AllParsers() []ContentParser
}

type parserFactoryImpl struct {
	parsers []ContentParser
}

func NewParserFactory() ParserFactory {
	return &parserFactoryImpl{
		parsers: make([]ContentParser, 0),
	}
}

func (f *parserFactoryImpl) RegisterParser(parser ContentParser) {
	f.parsers = append(f.parsers, parser)
}
func (f *parserFactoryImpl) GetParser(contentType string) (ContentParser, error) {
	normalized := normalizeContentType(contentType)

	// 优先匹配精确类型
	for _, p := range f.parsers {
		if p.CanParse(normalized) {
			return p, nil
		}
	}

	// 次优先匹配类型前缀
	for _, p := range f.parsers {
		for _, t := range p.SupportedTypes() {
			if strings.HasPrefix(normalized, t) {
				return p, nil
			}
		}
	}

	return nil, errors.Errorf("no parser found for content-type: %s", contentType)
}

func (f *parserFactoryImpl) AllParsers() []ContentParser {
	return f.parsers
}

type JsonParser struct{}

func (p *JsonParser) CanParse(contentType string) bool {
	return contentType == "application/json" ||
		strings.HasSuffix(contentType, "+json")
}

func (p *JsonParser) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (p *JsonParser) SupportedTypes() []string {
	return []string{"application/json"}
}

type DockerManifestParser struct {
}

func (p *DockerManifestParser) CanParse(contentType string) bool {
	return strings.Contains(contentType, "vnd.docker.distribution.manifest") ||
		strings.Contains(contentType, "vnd.docker.container.image") ||
		strings.Contains(contentType, "vnd.docker.image.rootfs")
}
func (p *DockerManifestParser) Unmarshal(data []byte, v any) error {
	// 特殊处理：验证Docker manifest结构
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	if _, ok := manifest["schemaVersion"]; !ok {
		return errors.New("invalid Docker manifest: missing schemaVersion")
	}
	return json.Unmarshal(data, v)
}

func (p *DockerManifestParser) SupportedTypes() []string {
	return []string{
		"application/vnd.docker.distribution.manifest.v1",
		"application/vnd.docker.distribution.manifest.v2",
		"application/vnd.docker.distribution.manifest.list.v2",
		"application/vnd.docker.container.image.v1",
	}
}

// XML解析器
type XmlParser struct{}

func (p *XmlParser) CanParse(contentType string) bool {
	return contentType == "application/xml" ||
		contentType == "text/xml"
}
func (p *XmlParser) Unmarshal(data []byte, v any) error {
	return xml.Unmarshal(data, v)
}
func (p *XmlParser) SupportedTypes() []string {
	return []string{"application/xml", "text/xml"}
}

// 二进制流解析器
type OctetStreamParser struct{}

func (p *OctetStreamParser) CanParse(contentType string) bool {
	return contentType == "application/octet-stream"
}
func (p *OctetStreamParser) Unmarshal(data []byte, v any) error {
	if target, ok := v.(*[]byte); ok {
		*target = data
		return nil
	}
	return errors.New("target must be *[]byte for octet-stream")
}
func (p *OctetStreamParser) SupportedTypes() []string {
	return []string{"application/octet-stream"}
}

func normalizeContentType(header string) string {
	if idx := strings.Index(header, ";"); idx != -1 {
		header = header[:idx]
	}
	return strings.ToLower(strings.TrimSpace(header))
}
