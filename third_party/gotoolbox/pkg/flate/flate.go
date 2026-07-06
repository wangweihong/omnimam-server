package flate

import (
	"compress/flate"
	"fmt"
	"io"
)

const flateUncompressLimit = 10 * 1024 * 1024 // 10MB

func NewSaferFlateReader(r io.Reader) io.ReadCloser {
	return &saferFlateReader{r: flate.NewReader(r)}
}

// 防止解压炸弹攻击（限制内存使用）
// SAML标准要求GET请求必须使用Deflate压缩算法
// 恶意攻击者可构造高度压缩的畸形数据（如1MB数据压缩后仅1KB，解压后膨胀到1GB），导致内存耗尽
type saferFlateReader struct {
	r     io.ReadCloser
	count int
}

func (r *saferFlateReader) Read(p []byte) (n int, err error) {
	if r.count+len(p) > flateUncompressLimit {
		return 0, fmt.Errorf("flate: uncompress limit exceeded (%d bytes)", flateUncompressLimit)
	}
	n, err = r.r.Read(p)
	r.count += n
	return n, err
}

func (r *saferFlateReader) Close() error {
	return r.r.Close()
}
