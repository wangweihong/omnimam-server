package httpsvr

import (
	"bytes"
	"io"
	"mime/multipart"

	"github.com/gin-gonic/gin"
	"github.com/wangweihong/gotoolbox/pkg/errors"
)

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

	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, file); err != nil {
		return nil, nil, errors.Errorf("read file from form:%v", err)
	}
	return fileHeader, buf, nil
}
