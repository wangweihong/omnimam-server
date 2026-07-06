//go:build !windows
// +build !windows

package osexec

import (
	"archive/tar"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/executil"
)

func GetFileChecksum(filePath string) (string, error) {
	output, err := executil.Execute("sha512sum", []string{"-b", filePath})
	if err != nil {
		return "", err
	}
	return strings.Split(string(output), " ")[0], nil
}

func CompressFile(filePath string) error {
	if _, err := executil.Execute("gzip", []string{filePath}); err != nil {
		return err
	}
	return nil
}

func DecompressFile(filePath string) error {
	if _, err := executil.Execute("gunzip", []string{filePath}); err != nil {
		return err
	}
	return nil
}

func CompressDir(sourceDir, targetFile string) error {
	tmpFile := targetFile + ".tmp"
	if _, err := executil.Execute("tar", []string{"cf", tmpFile, "-C", sourceDir, "."}); err != nil {
		return err
	}
	if _, err := executil.Execute("gzip", []string{tmpFile}); err != nil {
		return err
	}
	if _, err := executil.Execute("mv", []string{"-f", tmpFile + ".gz", targetFile}); err != nil {
		return err
	}
	return nil
}

// If sourceFile is inside targetDir, it would be deleted automatically
func DecompressDir(sourceFile, targetDir string) error {
	tmpDir := targetDir + ".tmp"
	if _, err := executil.Execute("rm", []string{"-rf", tmpDir}); err != nil {
		return err
	}
	if err := os.Mkdir(tmpDir, os.ModeDir|0700); err != nil {
		return err
	}
	if _, err := executil.Execute("tar", []string{"xf", sourceFile, "-C", tmpDir}); err != nil {
		return err
	}
	if _, err := executil.Execute("rm", []string{"-rf", targetDir}); err != nil {
		return err
	}
	if _, err := executil.Execute("mv", []string{"-f", tmpDir, targetDir}); err != nil {
		return err
	}
	return nil
}

func Copy(src, dst string) error {
	if _, err := executil.Execute("cp", []string{"-rp", src, dst}); err != nil {
		return err
	}
	return nil
}

func MoveFile(src, dst string) error {
	if _, err := executil.Execute("mv", []string{src, dst}); err != nil {
		return err
	}
	return nil
}

func Tar(src []string, dst string) error {
	// 创建tar文件
	fw, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fw.Close()

	// 通过fw创建一个tar.Writer
	tw := tar.NewWriter(fw)
	// 如果关闭失败会造成tar包不完整
	defer func() {
		if err := tw.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	for _, fileName := range src {
		fi, err := os.Stat(fileName)
		if err != nil {
			fmt.Println(err)
			continue
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		// 将tar的文件信息hdr写入到tw
		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		// 将文件数据写入
		fr, err := os.Open(fileName)
		if err != nil {
			return err
		}
		if _, err = io.Copy(tw, fr); err != nil {
			return err
		}
		fr.Close()
	}
	return nil
}

func Untar(src, dst string) ([]string, error) {
	var files []string
	fr, err := os.Open(src)
	if err != nil {
		return files, err
	}
	defer fr.Close()

	tr := tar.NewReader(fr)
	for hdr, err := tr.Next(); err != io.EOF; hdr, err = tr.Next() {
		if err != nil {
			return files, err
		}
		// 读取文件信息
		fi := hdr.FileInfo()
		files = append(files, fi.Name())

		// 创建一个空文件，用来写入解包后的数据
		dstPath := filepath.Join(dst, fi.Name())
		fw, err := os.Create(dstPath)
		if err != nil {
			return files, err
		}

		if _, err := io.Copy(fw, tr); err != nil {
			return files, err
		}
		os.Chmod(dstPath, fi.Mode().Perm())
		fw.Close()
	}
	return files, nil
}

func GetTarSize(file string) (uint64, error) {
	opts := []string{
		"-tvf",
		file,
	}
	output, err := executil.ExecuteTimeout("tar", opts, 3600)
	if err != nil {
		return 0, errors.Errorf("execute tar tvf %s error %s", file, err)
	}
	re := regexp.MustCompile("\\s+")
	scanner := bufio.NewScanner(strings.NewReader(output))
	var size uint64
	for scanner.Scan() {
		strList := re.Split(strings.TrimSpace(scanner.Text()), -1)
		if len(strList) < 3 {
			continue
		}
		fileSize, err := strconv.ParseUint(strList[2], 10, 64)
		if err != nil {
			return 0, errors.Errorf("parse %s to uint64 error %s", strList[2], err)
		}
		size += fileSize
	}
	return size, nil
}

// 替换文件中targetLine字符串所在的行
func ReplaceLineInFile(filename, targetLine, replacement string) error {
	file, err := os.OpenFile(filename, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	dir := filepath.Dir(filename)
	tmpFilePath := dir + "/tempfile"
	tempFile, err := os.Create(tmpFilePath)
	if err != nil {
		return err
	}
	defer tempFile.Close()

	// 创建读取器和写入器
	reader := bufio.NewReader(file)
	writer := bufio.NewWriter(tempFile)

	// 逐行读取原始文件内容，进行替换操作
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}

		if strings.Contains(line, targetLine) {
			line = replacement + "\n"
		}

		if _, err := writer.WriteString(line); err != nil {
			return err
		}

		if err == io.EOF {
			break
		}
	}

	// 刷新写入器缓冲区
	if err := writer.Flush(); err != nil {
		return err
	}

	// 关闭原始文件
	if err := file.Close(); err != nil {
		return err
	}

	// 重命名临时文件为原始文件
	if err := os.Rename(tempFile.Name(), filename); err != nil {
		return err
	}

	// 关闭临时文件
	if err := tempFile.Close(); err != nil {
		return err
	}

	return nil
}

// HashFileInChunks 文件分片进行hash
func HashFileInChunks(filePath string, chunkSize int64) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, chunkSize)
	for {
		bytesRead, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read file: %v", err)
		}

		hasher.Write(buffer[:bytesRead])
	}

	hashSum := hasher.Sum(nil)

	return hex.EncodeToString(hashSum), nil
}

func TraverseDir(rootDir string, process func(string) error) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return process(path)
		}
		return nil
	})
}

func LoadFile(path string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(d), nil
}
