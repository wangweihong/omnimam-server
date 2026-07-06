package httpform

import (
	"bytes"
	"io"
	"mime/multipart"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/httpcli"
	"github.com/wangweihong/gotoolbox/pkg/httpcli/def"
)

const (
	FileFormKey     = "file"
	KeyFileFormKey  = "key_file"
	CertFileFormKey = "cert_file"
)

const (
	FileLimitSize = 10 << 20
)

func FormUpload(c *gin.Context, limitSize int64) (*multipart.FileHeader, *bytes.Buffer, error) {
	return FormUploadFileKey(c, limitSize, FileFormKey)
}

func FormUploadFileKey(c *gin.Context, limitSize int64, fileKey string) (*multipart.FileHeader, *bytes.Buffer, error) {
	fileHeader, err := c.FormFile(fileKey)
	if err != nil {
		return nil, nil, errors.Errorf("cannot get file from form:%v", err)
	}

	if fileHeader.Size > limitSize {
		return nil, nil, errors.Errorf("exceed file limit:%v", err)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, errors.Errorf("open file from form:%v", err)
	}
	defer file.Close()

	// 这里将数据写到内存, 考虑到内存压力, limitsize因小于10MB
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return nil, nil, errors.Errorf("read file from form:%v", err)
	}
	return fileHeader, buf, nil
}

const (
	maxFileSize = 100 * 1024 * 1024 // 100MB
	maxTextSize = 10 * 1024         // 10KB
)

// 转发请求表达，这里值得注意的是，并没有先读取文件表单数据到内存。而是采用流式拷贝
func ForwardForm(c *gin.Context, maxFileSize int64) (*httpcli.HttpRequestBuilder, map[string]any, error) {
	// 1. 创建HTTP请求构建器
	builder := httpcli.NewHttpRequestBuilder()
	fm := make(map[string]any)

	// 2. 设置请求头（排除Content-Type和Content-Length）
	for k, v := range c.Request.Header {
		if k != "Content-Type" && k != "Content-Length" {
			builder.AddBasicAuthHeaderParam(k, v[0])
		}
	}

	// 3. 获取multipart读取器
	reader, err := c.Request.MultipartReader()
	if err != nil {
		return nil, nil, errors.Errorf("failed to get multipart reader: %v", err)
	}

	// 4. 获取表达所有数据
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.Errorf("error reading part: %v", err)
		}

		formName := part.FormName()
		fileName := part.FileName()

		// 5. 根据数据类型创建FormData
		if fileName != "" {
			// 文件类型：流式转发

			if contentLength := part.Header.Get("Content-Length"); contentLength != "" {
				size, err := strconv.ParseInt(contentLength, 10, 64)
				if err == nil {
					if int64(size) > maxFileSize {
						return nil, nil, errors.Errorf("file size exceeds limit: %d > %d", size, maxFileSize)
					}

				}
			}

			// fileSize, err := getPartSize(part)
			// if err != nil {
			// 	return nil, errors.Errorf("error getting file size: %w", err)
			// }

			// if fileSize > maxFileSize {
			// 	return nil, errors.Errorf("file size exceeds limit: %d > %d", fileSize, maxFileSize)
			// }

			builder.AddFormParam(formName, &def.StreamFilePart{
				Reader:   part,
				Filename: fileName,
			})
		} else {
			// 文本类型：读取内容
			content, err := io.ReadAll(part)
			if err != nil {
				return nil, nil, errors.Errorf("error reading text part: %v", err)
			}

			if len(content) > maxTextSize {
				return nil, nil, errors.Errorf("text field size exceeds limit: %d > %d", len(content), maxTextSize)
			}

			builder.AddFormParam(formName, &def.MultiPart{
				Content: content,
			})

			fm[formName] = content
		}

		// 立即关闭当前part
		part.Close()
	}

	return builder, fm, nil
}

// // 辅助函数：获取part大小而不消耗Reader
// func getPartSize(part *multipart.Part) (int64, error) {
// 	size := int64(0)
// 	buf := make([]byte, 32*1024)

// 	for {
// 		n, err := part.Read(buf)
// 		size += int64(n)

// 		if err == io.EOF {
// 			// 重置读取位置
// 			_, seekErr := part.Seek(0, io.SeekStart)
// 			return size, seekErr
// 		}

// 		if err != nil {
// 			return 0, err
// 		}
// 	}
// }
