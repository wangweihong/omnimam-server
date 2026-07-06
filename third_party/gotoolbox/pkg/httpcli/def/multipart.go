package def

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/typeutil"
)

var quoteEscape = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscape.Replace(s)
}

type FilePart struct {
	Headers textproto.MIMEHeader
	Content *os.File
}

func NewFilePart(content *os.File) *FilePart {
	return &FilePart{
		Content: content,
	}
}

func NewFilePartWithContentType(content *os.File, contentType string) *FilePart {
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Type", contentType)

	return &FilePart{
		Headers: headers,
		Content: content,
	}
}

func (f FilePart) Write(w *multipart.Writer, name string) error {
	var h textproto.MIMEHeader
	if f.Headers != nil {
		h = f.Headers
	} else {
		h = make(textproto.MIMEHeader)
	}

	filename := filepath.Base(f.Content.Name())
	if filename == "" {
		return errors.New("failed to obtain filename from: " + f.Content.Name())
	}

	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(name), escapeQuotes(filename)))

	if f.Headers.Get("Content-Type") == "" {
		h.Set("Content-Type", "application/octet-stream")
	}

	writer, err := w.CreatePart(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, f.Content)
	return err
}

type MultiPart struct {
	Content any
}

func NewMultiPart(content any) *MultiPart {
	return &MultiPart{
		Content: content,
	}
}

func (m MultiPart) Write(w *multipart.Writer, name string) error {
	err := w.WriteField(name, typeutil.ConvertInterfaceToString(m.Content))
	return err
}

type FormData interface {
	Write(*multipart.Writer, string) error
}

// FilePartitionPart 用于预先读取大文件进行分片，通过表单上传
type FilePartitionPart struct {
	Content  []byte
	FileName string
}

func NewFilePartitionPart(fileName string, content []byte) *FilePartitionPart {
	return &FilePartitionPart{
		FileName: fileName,
		Content:  content,
	}
}

func (m FilePartitionPart) Write(w *multipart.Writer, name string) error {
	writer, err := w.CreateFormFile(name, m.FileName)
	if err != nil {
		return err
	}

	if _, err := io.Copy(writer, bytes.NewReader(m.Content)); err != nil {
		return err
	}
	return err
}

// 实现StreamFilePart（流式文件包装器）
type StreamFilePart struct {
	io.Reader
	Filename string
}

func (f *StreamFilePart) Write(w *multipart.Writer, fieldname string) error {
	part, err := w.CreateFormFile(fieldname, f.Filename)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}
