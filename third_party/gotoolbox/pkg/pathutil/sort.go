package pathutil

import (
	"sort"
	"strings"
)

// 文件优先路径深度排序
type FileFirstDepthPath struct {
	Value string
	IsDir bool
}

type FileFirstDepthPaths []FileFirstDepthPath

func (p FileFirstDepthPaths) Len() int {
	return len(p)
}

func (p FileFirstDepthPaths) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p FileFirstDepthPaths) Less(i, j int) bool {
	// 先按深度排序
	depthI := strings.Count(p[i].Value, "/")
	depthJ := strings.Count(p[j].Value, "/")

	if depthI != depthJ {
		return depthI < depthJ
	}

	// 在相同深度下，文件排在目录前面
	return !p[i].IsDir && p[j].IsDir
}

func (p FileFirstDepthPaths) Sort() FileFirstDepthPaths {
	if p == nil {
		return nil
	}
	sort.Sort(p)
	return p
}

func (p FileFirstDepthPaths) ToSlice() []string {
	paths := make([]string, 0, len(p))
	for _, v := range p {
		paths = append(paths, v.Value)
	}
	return paths
}

func ToFileFirstDepthPaths(paths []string) FileFirstDepthPaths {
	var pathObjects FileFirstDepthPaths
	for _, p := range paths {
		isDir := strings.HasSuffix(p, "/")
		pathObjects = append(pathObjects, FileFirstDepthPath{Value: p, IsDir: isDir})
	}
	return pathObjects
}

// 文件优先路径深度排序
type DirLastDepthPath struct {
	Value string
	IsDir bool
}

type DirLastDepthPaths []DirLastDepthPath

func (p DirLastDepthPaths) Len() int {
	return len(p)
}

func (p DirLastDepthPaths) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p DirLastDepthPaths) Less(i, j int) bool {
	// 先按深度排序
	depthI := strings.Count(p[i].Value, "/")
	depthJ := strings.Count(p[j].Value, "/")

	if depthI != depthJ {
		return depthI > depthJ
	}

	// 在相同深度下，目录排在文件后面
	if p[i].IsDir != p[j].IsDir {
		return !p[i].IsDir
	}

	// 如果都是目录或都是文件，在相同深度下按照字典序排序
	return p[i].Value < p[j].Value
}

func (p DirLastDepthPaths) Sort() DirLastDepthPaths {
	if p == nil {
		return nil
	}
	sort.Sort(p)
	return p
}

func (p DirLastDepthPaths) ToSlice() []string {
	paths := make([]string, 0, len(p))
	for _, v := range p {
		paths = append(paths, v.Value)
	}
	return paths
}

func ToDirLastDepthPaths(paths []string) DirLastDepthPaths {
	var pathObjects DirLastDepthPaths
	for _, p := range paths {
		isDir := strings.HasSuffix(p, "/")
		pathObjects = append(pathObjects, DirLastDepthPath{Value: p, IsDir: isDir})
	}
	return pathObjects
}
