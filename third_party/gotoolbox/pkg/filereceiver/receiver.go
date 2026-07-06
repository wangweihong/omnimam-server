package filereceiver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/wangweihong/gotoolbox/pkg/mathutil"
)

const (
	filePartitionExt = ".part"

	PartitionSize1MB = 1024 * 1024
)

type fileReceiver struct {
	// 文件路径
	path string
	// 文件总大小
	totalSize int64
	// 分片大小
	partitionSize int64
	// 分片索引 int64
	partitionIndex int64
	// 文件是否已经创建
	fileCreate bool
}

type FileReceiver interface {
	Receive(data []byte, index int64) error
}

func NewFileReceiver(dir, filename string, totalSize, partitionSize int64) (FileReceiver, error) {
	path := filepath.Join(dir, filename)

	if path == "" {
		return nil, fmt.Errorf("invalid filepath")
	}

	if totalSize == 0 {
		return nil, fmt.Errorf("totalSize cannot be 0")
	}

	if partitionSize == 0 {
		return nil, fmt.Errorf("partition Size cannot be 0")
	}

	partitionIndex := totalSize / partitionSize
	if totalSize%partitionSize != 0 {
		partitionIndex += 1
	}
	return &fileReceiver{
		path:           path,
		totalSize:      totalSize,
		partitionSize:  partitionSize,
		partitionIndex: partitionIndex,
	}, nil
}

func (f *fileReceiver) Receive(data []byte, index int64) error {
	if index > f.partitionIndex {
		return fmt.Errorf("invalid partition index")
	}

	dataLen := int64(len(data))
	if dataLen > f.partitionSize {
		return fmt.Errorf("data exceed partition size %vk", mathutil.ParseSizeByteToStr(uint64(f.partitionSize)))
	}

	offset := index * f.partitionSize

	if offset+dataLen > f.totalSize {
		return fmt.Errorf("exceed file size")
	}

	var fp *os.File
	var err error
	if !f.fileCreate {
		fp, err = os.Create(f.path)
		if err != nil {
			return err
		}
		if err := fp.Truncate(f.totalSize); err != nil {
			return err
		}
		f.fileCreate = true
	} else {
		fp, err = os.OpenFile(f.path, os.O_RDWR, 0755)
		if err != nil {
			return err
		}
	}

	if _, err := fp.Seek(offset, 0); err != nil {
		return err
	}

	n, err := fp.Write(data)
	if err != nil {
		return err
	}

	if int64(n) != dataLen {
		return fmt.Errorf("write data len no match datalen")
	}

	return nil
}
