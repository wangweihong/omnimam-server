package template

import (
	"bytes"

	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/gotoolbox/pkg/log"
)

type ProcessorInterface interface {
	Name() string
}

var _ ProcessorInterface = &FileProcessor{}
var _ ProcessorInterface = &DirectoryProcessor{}

type FileProcessor struct {
	TemplateName    string
	TemplateText    *template.Template
	Context         map[string]any
	FilePath        string // filepath to write after template has parsed
	FileMode        os.FileMode
	TemplateData    string // template data
	TemplatePath    string // template path
	TemplateDir     string
	TemplateSubPath string
	err             error
}

func (p *FileProcessor) Error() error {
	return p.err
}

func (p *FileProcessor) Parse() (string, error) {
	var data bytes.Buffer

	if p.TemplatePath == "" {
		if p.TemplateDir == "" || p.TemplateSubPath == "" {
			return "", errors.Errorf("template %v TemplateDir | p.TemplateSubPath is empty when TemplatePath not set", p.Name())
		}
		p.TemplatePath = filepath.Join(p.TemplateDir, p.TemplateSubPath)
	}

	if p.TemplateText == nil && p.TemplateData == "" {
		return "", errors.Errorf("template %v TemplateText  & TemplateData is Empty", p.Name())
	}

	if p.TemplateText == nil {
		if p.TemplateData != "" {
			p.TemplateText = template.Must(template.New(p.Name()).Parse(p.TemplateData))
		} else {
			templateByteData, err := ioutil.ReadFile(p.TemplatePath)
			if err != nil {
				return "", err
			}
			p.TemplateText = template.Must(template.New(p.Name()).Parse(string(templateByteData)))
		}
	}

	if err := p.TemplateText.Execute(&data, p.Context); err != nil {
		return "", errors.Errorf("parse template %v fail:%v", p.TemplateText.Name(), err)
	}
	return strings.TrimPrefix(data.String(), "\n"), nil
}

func (p *FileProcessor) SetContexts(context map[string]any) *FileProcessor {
	for k, v := range context {
		p.Context[k] = v
	}
	return p
}

func (p *FileProcessor) SetContext(key string, value any) *FileProcessor {
	if p.Context == nil {
		p.Context = make(map[string]any)
	}
	p.Context[key] = value
	return p
}

func (p *FileProcessor) SetFileMode(fm os.FileMode) *FileProcessor {
	p.FileMode = fm
	return p
}

func (p FileProcessor) Name() string {
	if p.TemplateText != nil {
		return p.TemplateText.Name()
	}
	return p.TemplateName
}

func (p *FileProcessor) SetFilePath(filepath string) *FileProcessor {
	p.FilePath = filepath
	return p
}

func (p *FileProcessor) LocateToDiskForTest(appendData ...string) error {
	return p.LocateToDisk(appendData...).Error()
}

func (p *FileProcessor) LocateToDisk(appendData ...string) *FileProcessor {
	if p.FilePath == "" {
		p.err = errors.Errorf("template %v has not set locate file name", p.Name())
		return p
	}

	//if !path.IsAbs(p.FilePath) {
	//	p.err = errors.Errorf("template %v locate file name %v is not absolute path", p.Name(), p.FilePath)
	//	return p
	//}

	data, err := p.Parse()
	if err != nil {
		p.err = errors.Errorf("template %v parse error:%v", p.Name(), err.Error())
		return p
	}

	for _, ad := range appendData {
		if strings.TrimSpace(ad) != "" {
			data += "\n---\n" + ad
		}
	}

	if err := os.MkdirAll(filepath.Dir(p.FilePath), 0o755); err != nil {
		p.err = errors.Errorf("template %v mkdir %v error :%v", p.Name(), p.FilePath, err.Error())
		return p
	}

	if p.FileMode == 0 {
		p.FileMode = 0o644
	}

	if err := ioutil.WriteFile(p.FilePath, []byte(data), p.FileMode); err != nil {
		p.err = errors.Errorf("template %v save to file %v error :%v", p.Name(), p.FilePath, err.Error())
		return p
	}

	return p
}

type DirectoryProcessor struct {
	TemplateName    string
	Context         map[string]any // context used parse all template beyond the template dir
	TemplateDir     string         // filepath to write after template has parsed
	LocateParsedDir string         // which dir parsed files load
	err             error
}

func (p *DirectoryProcessor) LocateToDiskForTest() error {
	return p.LocateToDisk().Error()
}

func (p *DirectoryProcessor) LocateToDisk() *DirectoryProcessor {
	if p.TemplateDir == "" {
		p.err = errors.Errorf("template %v has invalid TemplateDir %v", p.TemplateName, p.TemplateDir)
		return p
	}

	fi, err := os.Stat(p.TemplateDir)
	if err != nil {
		p.err = errors.Errorf("template %v stat TemplateDir %v fail:%v", p.TemplateName, p.TemplateDir, err)
		return p
	}

	if !fi.IsDir() {
		p.err = errors.Errorf("template %v TemplateDir %v is not dir", p.TemplateName, p.TemplateDir)
		return p
	}

	// if p.LocateParsedDir == "" || !filepath.IsAbs(p.LocateParsedDir) {
	if p.LocateParsedDir == "" {
		p.err = errors.Errorf("template %v has invalid LocateParsedDir %v", p.TemplateName, p.LocateParsedDir)
		return p
	}

	if err := os.MkdirAll(p.LocateParsedDir, 0o755); err != nil {
		p.err = errors.Errorf("template %v mkdir %v error :%v", p.TemplateName, p.LocateParsedDir, err.Error())
		return p
	}

	// path is absolute path of file/dir
	// walk subdir too.
	if err := filepath.Walk(p.TemplateDir, p.parseAndWriteToDisk); err != nil {
		p.err = errors.Errorf(
			"templateProc %v parseAndWriteToDisk %v to disk %v　fail:%v",
			p.TemplateName,
			p.TemplateDir,
			p.LocateParsedDir,
			err,
		)
		return p
	}
	if err := filepath.Walk(p.TemplateDir, p.writeYamlToDisk); err != nil {
		p.err = errors.Errorf(
			"templateProc %v parseAndWriteToDisk %v to located disk %v　fail:%v",
			p.TemplateName,
			p.TemplateDir,
			p.LocateParsedDir,
			err,
		)
		return p
	}

	return p
}

func (p *DirectoryProcessor) SetContexts(context map[string]any) *DirectoryProcessor {
	if p.Context == nil {
		p.Context = make(map[string]any)
	}
	for k, v := range context {
		p.Context[k] = v
	}
	return p
}

func (p *DirectoryProcessor) parseAndWriteToDisk(path string, info os.FileInfo, err error) error {
	log.Debugf("-----------------------")
	log.Debugf("path:%v", path)
	log.Debugf("isDir:%v", info.IsDir())
	log.Debugf("info.Name:%v", info.Name())
	// parse template and write to file
	if !info.IsDir() {
		if strings.HasSuffix(info.Name(), ".template") {
			yalmPathToDisk := strings.TrimSuffix(path, ".template")
			templateByteData, err := ioutil.ReadFile(path)
			if err != nil {
				return errors.Errorf("read file %v err:%v", path, err)
			}

			var data bytes.Buffer
			if err := template.Must(template.New(info.Name()).Parse(string(templateByteData))).Execute(&data, p.Context); err != nil {
				return errors.Errorf("parse Template file %v err:%v", path, err)
			}
			dataWriteToCache := data.String()
			if err := ioutil.WriteFile(yalmPathToDisk, []byte(dataWriteToCache), 0o644); err != nil {
				return errors.Errorf(
					"templateProc %v template file %v save to file %v error :%v",
					p.TemplateName,
					path,
					yalmPathToDisk,
					err.Error(),
				)
			}
		}
	}

	return nil
}

var fileSuffixToWrite = []string{".yaml", ".yml", ".sh", ".config"}

func isFileSuffixToWrite(fileName string) bool {
	for _, v := range fileSuffixToWrite {
		if strings.HasSuffix(fileName, v) {
			return true
		}
	}
	return false
}

func (p *DirectoryProcessor) writeYamlToDisk(path string, info os.FileInfo, err error) error {
	log.Debugf("-----------------------")
	log.Debugf("path:%v", path)
	log.Debugf("isDir:%v", info.IsDir())
	log.Debugf("info.Name:%v", info.Name())

	if !info.IsDir() {
		if !isFileSuffixToWrite(info.Name()) {
			log.Debugf("ignore file :%v", path)
			return nil
		}
		// 是否存在子目录
		templateDir := filepath.Dir(path)
		subPath := strings.TrimPrefix(templateDir, p.TemplateDir)
		log.Debugf("SubPath:%v", subPath)
		locateDir := p.LocateParsedDir
		if subPath != "" {
			locateDir = filepath.Join(p.LocateParsedDir, subPath)
			if err := os.MkdirAll(locateDir, 0o755); err != nil {
				return errors.Errorf("template %v mkdir %v error :%v", p.TemplateName, locateDir, err.Error())
			}
		}
		log.Debugf("locateDir: %v", locateDir)

		parsedAbsPath := filepath.Join(locateDir, info.Name())
		templateByteData, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Errorf("read file %v err:%v", path, err)
		}

		dataWriteToCache := string(templateByteData)

		if err := ioutil.WriteFile(parsedAbsPath, []byte(dataWriteToCache), 0o644); err != nil {
			return errors.Errorf(
				"templateProc %v template file %v save to file %v error :%v",
				p.TemplateName,
				path,
				parsedAbsPath,
				err.Error(),
			)
		}
	}

	return nil
}

func (p *DirectoryProcessor) SetContext(key string, value any) *DirectoryProcessor {
	if p.Context == nil {
		p.Context = make(map[string]any)
	}
	p.Context[key] = value
	return p
}

func (p *DirectoryProcessor) Error() error {
	return p.err
}

func (p DirectoryProcessor) Name() string {
	return p.TemplateName
}
