package compress

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
	"github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/log"
	"github.com/wangweihong/gotoolbox/pkg/sets"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"

	"github.com/bodgit/sevenzip"
)

type Extractor interface {
	// FindDirPathInTar 找到指定文件在Tar包的路径(最短路径)
	FindDirPathInTar(archivePath string, targetFileName ...string) (string, error)
	// FindFileFormatPathInTar 找到指定文件格式后缀在Tar包的路径
	FindFileFormatPathInTar(archivePath string, formatType ...string) (string, error)
	// 将Tar中指定路径下的文件树解压
	ExtractTarGZDirectory(archivePath, targetDir, destPath string) error
}

func NewExtractor(fileName string) Extractor {
	if stringutil.HasAnySuffix(fileName, ".tar.gz", ".tgz") {
		return &TarExtractor{}
	} else if strings.HasSuffix(fileName, ".zip") {
		return &ZipExtractor{}
	} else if strings.HasSuffix(fileName, ".rar") {
		return &RarExtractor{}
	} else if strings.HasSuffix(fileName, ".7z") {
		return &Win7RExtractor{}
	}
	return &InvalidExtractor{}
}

type ZipExtractor struct{}

func (z *ZipExtractor) FindFileFormatPathInTar(archivePath string, formatType ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return stringutil.HasAnySuffix(fName, p2...)
	}, archivePath, formatType...)
}

func (z *ZipExtractor) FindDirPathInTar(archivePath string, targetName ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return sets.NewString(p2...).Has(fName)
	}, archivePath, targetName...)
}

func (z *ZipExtractor) ExtractTarGZDirectory(archivePath, targetDir, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	targetDir = strings.TrimPrefix(filepath.ToSlash(targetDir), "./")
	targetDir = strings.TrimSuffix(targetDir, "/") + "/"
	for _, f := range r.File {
		fileName := filepath.ToSlash(f.Name)

		if !strings.HasPrefix(fileName, targetDir) {
			continue
		}

		relPath := strings.TrimPrefix(fileName, targetDir)
		fullPath := filepath.Join(destPath, relPath)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return errors.Errorf("mkdir error: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return errors.Errorf("mkdir : %v", err)
		}

		if err := z.readFile(f, fullPath); err != nil {
			return err
		}
	}

	return nil
}

func (z *ZipExtractor) readFile(f *zip.File, fullPath string) error {
	rc, err := f.Open()
	if err != nil {
		return errors.Errorf("open zip file error: %v", err)
	}
	defer rc.Close()

	outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return errors.Errorf("open file error: %v", err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, rc); err != nil {
		return errors.Errorf("io copy error: %v", err)
	}
	return nil
}

func (z *ZipExtractor) findDirPathCondition(condition func(p1 string, p2 ...string) bool, archivePath string, targetNames ...string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	var shortestPath string
	for _, f := range r.File {
		if condition(path.Base(f.Name), targetNames...) {
			shortestPath = findShortPath(path.Dir(f.Name)+"/", shortestPath)
		}
	}
	if shortestPath != "" {
		return shortestPath, nil
	}

	return "", errors.Errorf("%v not found in archive", targetNames)
}

type RarExtractor struct{}

func (z *RarExtractor) FindFileFormatPathInTar(rarPath string, formatType ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return stringutil.HasAnySuffix(fName, p2...)
	}, rarPath, formatType...)
}

func (z *RarExtractor) FindDirPathInTar(rarPath string, targetNames ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return sets.NewString(p2...).Has(fName)
	}, rarPath, targetNames...)
}

func (z *RarExtractor) findDirPathCondition(condition func(p1 string, p2 ...string) bool, archivePath string, targetNames ...string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", errors.Errorf("open archive path: %v", err)
	}
	defer f.Close()
	r, err := rardecode.NewReader(f, "")
	if err != nil {
		return "", errors.Errorf("parse archive path: %v", err)
	}
	var shortestPath string
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.Errorf("read file error: %v", err)
		}
		if condition(path.Base(header.Name), targetNames...) {
			shortestPath = findShortPath(path.Dir(header.Name)+"/", shortestPath)
		}
	}
	if shortestPath != "" {
		return shortestPath, nil
	}

	return "", errors.Errorf("%v not found in archive", targetNames)
}

func (z *RarExtractor) ExtractTarGZDirectory(rarPath, targetDir, destPath string) error {
	f, err := os.Open(rarPath)
	if err != nil {
		return errors.Errorf("open rar file: %v", err)
	}
	defer f.Close()
	r, err := rardecode.NewReader(f, "")
	if err != nil {
		return errors.Errorf("new reader: %v", err)
	}
	targetDir = strings.TrimPrefix(filepath.ToSlash(targetDir), "./")
	targetDir = strings.TrimSuffix(targetDir, "/") + "/"
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Errorf("read: %v", err)
		}
		fileName := filepath.ToSlash(header.Name)
		if !strings.HasPrefix(fileName, targetDir) {
			continue
		}
		relPath := strings.TrimPrefix(fileName, targetDir)
		fullPath := filepath.Join(destPath, relPath)
		if header.IsDir {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return errors.Errorf("mkdir error: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return errors.Errorf("mkdir error: %v", err)
		}
		if err := z.readFile(r, fullPath); err != nil {
			return err
		}
	}
	return nil
}

func (z *RarExtractor) readFile(r io.Reader, fullPath string) error {
	outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return errors.Errorf("open file error: %v", err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, r); err != nil {
		return errors.Errorf("io.Copy error: %v", err)
	}
	return nil
}

type TarExtractor struct{}

func (z *TarExtractor) FindFileFormatPathInTar(archivePath string, formatType ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return stringutil.HasAnySuffix(fName, p2...)
	}, archivePath, formatType...)
}

func (z *TarExtractor) FindDirPathInTar(archivePath string, targetNames ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return stringutil.HasAnySuffix(fName, p2...)
	}, archivePath, targetNames...)
}

func (z *TarExtractor) findDirPathCondition(condition func(p1 string, p2 ...string) bool, archivePath string, targetNames ...string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var shortestPath string
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if condition(path.Base(header.Name), targetNames...) {
			shortestPath = findShortPath(path.Dir(header.Name)+"/", shortestPath)
		}
	}
	if shortestPath != "" {
		return shortestPath, nil
	}
	return "", errors.Errorf("%v not found in archive", targetNames)
}

func (z *TarExtractor) ExtractTarGZDirectory(archivePath, targetDir, destPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		log.Debugf("header.Name:%v,targetDir:%v", header.Name, targetDir)
		fp := strings.TrimPrefix(header.Name, "./")
		if !strings.HasPrefix(fp, targetDir) {
			log.Debugf("ignore header.Name:%v,targetDir:%v", fp, targetDir)
			continue
		}

		relPath := strings.TrimPrefix(fp, targetDir)
		fullPath := filepath.Join(destPath, relPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return errors.Errorf("mkdir error: %v", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return errors.Errorf("mkdir error: %v", err)
			}

			outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return errors.Errorf("open file error: %v", err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return errors.Errorf("copy file error: %v", err)
			}
			outFile.Close()
		default:
			log.Infof("skip non regular file: %s (type: %c)", header.Name, header.Typeflag)
		}
	}

	return nil
}

type Win7RExtractor struct {
}

func (z *Win7RExtractor) FindFileFormatPathInTar(archivePath string, formatType ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return stringutil.HasAnySuffix(fName, p2...)
	}, archivePath, formatType...)
}

func (z *Win7RExtractor) FindDirPathInTar(archivePath string, targetName ...string) (string, error) {
	return z.findDirPathCondition(func(fName string, p2 ...string) bool {
		return sets.NewString(p2...).Has(fName)
	}, archivePath, targetName...)
}

func (z *Win7RExtractor) findDirPathCondition(condition func(p1 string, p2 ...string) bool, archivePath string, targetNames ...string) (string, error) {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return "", errors.Errorf("open archivePath file: %v", err)
	}
	defer r.Close()
	var shortestPath string
	for _, f := range r.File {
		if condition(path.Base(f.Name), targetNames...) {
			shortestPath = findShortPath(path.Dir(f.Name)+"/", shortestPath)
		}
	}
	if shortestPath != "" {
		return shortestPath, nil
	}
	return "", errors.Errorf("no file name found: %s", targetNames)
}

func (z *Win7RExtractor) ExtractTarGZDirectory(archivePath, targetDir, destPath string) error {
	r, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return errors.Errorf("oepn archivePathfile error: %v", err)
	}
	defer r.Close()
	targetDir = strings.TrimPrefix(filepath.ToSlash(targetDir), "./")
	targetDir = strings.TrimSuffix(targetDir, "/") + "/"
	for _, f := range r.File {
		fileName := filepath.ToSlash(f.Name)
		if !strings.HasPrefix(fileName, targetDir) {
			continue
		}
		relPath := strings.TrimPrefix(fileName, targetDir)
		fullPath := filepath.Join(destPath, relPath)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return errors.Errorf("mdkir error: %v", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return errors.Errorf("mkdir error: %v", err)
		}
		if err := z.readFile(f, fullPath); err != nil {
			return err
		}
	}
	return nil
}

func (z *Win7RExtractor) readFile(f *sevenzip.File, fullPath string) error {
	rc, err := f.Open()
	if err != nil {
		return errors.Errorf("open file: %v", err)
	}
	defer rc.Close()
	outFile, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return errors.Errorf("open file: %v", err)
	}
	defer outFile.Close()
	if _, err := io.Copy(outFile, rc); err != nil {
		return errors.Errorf("io.copy: %v", err)
	}
	return nil
}

type InvalidExtractor struct {
}

func (z *InvalidExtractor) FindFileFormatPathInTar(archivePath string, formatType ...string) (string, error) {
	return "", errors.Errorf("invalid format")
}

func (z *InvalidExtractor) ExtractTarGZDirectory(archivePath, targetDir, destPath string) error {
	return errors.Errorf("invalid format")
}

func (z *InvalidExtractor) FindDirPathInTar(archivePath string, targetName ...string) (string, error) {
	return "", errors.Errorf("invalid format")
}

func findShortPath(targetDir, shortestPath string) string {
	ts := strings.Split(targetDir, "/")
	ss := strings.Split(shortestPath, "/")
	if shortestPath == "" || len(ts) < len(ss) {
		return targetDir
	}
	return shortestPath
}
