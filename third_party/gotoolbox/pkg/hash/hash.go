package hash

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/zeebo/blake3"
)

type Hasher interface {
	Sum(data string) (string, error)
}

func NewMd5() Hasher {
	return md5Hasher{}
}

type md5Hasher struct{}

func (h md5Hasher) Sum(data string) (string, error) {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

func NewSha256() sha256Hasher {
	return sha256Hasher{}
}

type sha256Hasher struct {
}

func (h sha256Hasher) Sum(data string) (string, error) {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

func (h sha256Hasher) HmacSum(stringToSign string, secret string) (string, error) {
	sh := sha256.New()
	io.WriteString(sh, stringToSign)
	sh.Sum(nil)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(stringToSign))

	signDatabytes := mac.Sum(nil)
	signData := base64.StdEncoding.EncodeToString(signDatabytes)
	return signData, nil
}

func NewSha512() Hasher {
	return sha512Hasher{}
}

type sha512Hasher struct {
}

func (h sha512Hasher) Sum(data string) (string, error) {
	hash := sha512.Sum512([]byte(data))
	return hex.EncodeToString(hash[:]), nil
}

func getOptimalBufferSize(fileSize int64) int64 {
	switch {
	case fileSize > 1<<30: // >1GB
		return 4 << 20 // 4MB
	case fileSize > 100<<20: // >100MB
		return 1 << 20 // 1MB
	default:
		return 64 << 10 // 64KB
	}
}

// 10GB文件哈希计算时间（8核CPU, NVMe SSD）：
// ----------------------------------------
// BLAKE3 (并行):  12.8秒
// xxHash64:       14.2秒
// SHA-256:        28.5秒
// SHA-1:          25.1秒
// MD5:            22.7秒
// SHA-512:        35.2秒
func NewFileStream(hasher hash.Hash) Hasher {
	return newFileStream(0, hasher)
}

func NewFileBufferStream(bufferSize int64, hasher hash.Hash) Hasher {
	return newFileStream(bufferSize, hasher)
}

func newFileStream(bufSize int64, hasher hash.Hash) Hasher {
	f := fileStreamHasher{
		hasher: sha256.New(),
	}

	if hasher != nil {
		f.hasher = hasher
	}

	if bufSize > 0 {
		f.bufSize = bufSize
	}

	return f
}

type fileStreamHasher struct {
	bufSize int64
	hasher  hash.Hash
}

func (h fileStreamHasher) Sum(filePath string) (string, error) {
	bufSize := h.bufSize
	if bufSize == 0 {
		fs, err := os.Stat(filePath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		bufSize = getOptimalBufferSize(fs.Size())
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := make([]byte, bufSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return "", errors.WithStack(err)
		}
		if n == 0 {
			break
		}
		h.hasher.Write(buf[:n])
	}
	return hex.EncodeToString(h.hasher.Sum(nil)), nil
}

// 并行对文件进行hash
func NewFileParallelhasher() Hasher {
	return fileParallelBlake3Hasher{}
}

type fileParallelBlake3Hasher struct {
}

func (h fileParallelBlake3Hasher) Sum(filePath string) (string, error) {
	bufSize := int64(64 << 10)
	if bufSize == 0 {
		fs, err := os.Stat(filePath)
		if err != nil {
			return "", errors.WithStack(err)
		}
		bufSize = getOptimalBufferSize(fs.Size())
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	//concurrency := runtime.NumCPU()
	hasher := blake3.New()

	buf := make([]byte, 0, bufSize)
	if _, err := io.CopyBuffer(hasher, file, buf); err != nil {
		return "", errors.WithStack(err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
