package debug

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
)

var (
	_dir                   = "/var/data/prof"
	_profileChan           = make(chan struct{}, 1)
	_defaultProfileTimeout = 30 * time.Second
)

// StartProf 开始启动性能profile.
func StartProf(dir string) error {
	if dir == "" {
		dir = _dir
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if err := saveMemProf(dir); err != nil {
		return err
	}

	if err := saveBlockProfile(dir); err != nil {
		return err
	}

	if err := saveGoroutineProfile(dir); err != nil {
		return err
	}

	// cpu profile 放置在最后. 其他profile瞬间完成, cpu profile运行一段时间后才能采集到数据
	if err := startCpuProf(dir, _defaultProfileTimeout); err != nil {
		return err
	}

	// 等待cpu profile完成
	time.Sleep(_defaultProfileTimeout)

	return nil
}

// goroutine memory.
func saveMemProf(dir string) error {
	memProfFilePath := filepath.Join(dir, "mem.prof")
	f, err := os.Create(memProfFilePath)
	if err != nil {
		return errors.Errorf("create mem profile file %v error: %w", memProfFilePath, err)
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err := pprof.WriteHeapProfile(f); err != nil {
		return errors.Errorf("could not write memory profile %v: %w ", memProfFilePath, err)
	}
	return nil
}

// goroutine block.
func saveBlockProfile(dir string) error {
	blockProfFilePath := filepath.Join(dir, "block.prof")
	f, err := os.Create(blockProfFilePath)
	if err != nil {
		return errors.Errorf("create block profile file %v error: %w", blockProfFilePath, err)
	}
	defer f.Close()

	if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
		return errors.Errorf("could not write block profile %v: %w", blockProfFilePath, err)
	}
	return nil
}

// goroutine block.
func saveGoroutineProfile(dir string) error {
	goroutineProfFilePath := filepath.Join(dir, "goroutine.prof")
	f, err := os.Create(goroutineProfFilePath)
	if err != nil {
		return errors.Errorf("create goroutine profile file %v error: %w", goroutineProfFilePath, err)
	}
	defer f.Close()

	if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
		return errors.Errorf("could not write goroutine profile %v: %w ", goroutineProfFilePath, err)
	}
	return nil
}

// startCpuProf 开始cpu剖析.
func startCpuProf(dir string, timeout time.Duration) error {
	if timeout < 0 {
		timeout = 20 * time.Second
	}

	// 接收残留的StopProfile发来的信号, 避免直接关闭掉cpu profile goroutine
	select {
	case <-_profileChan:
	default:
	}

	cpuProfFilePath := filepath.Join(dir, "cpu.prof")
	f, err := os.Create(cpuProfFilePath)
	if err != nil {
		return errors.Errorf("create cpu profile file error: %w", err)
	}

	// 1. 启动后必须要显式调用pprof.StopCPUProfile(), 否则cpu profile一直在后台运行不会停止
	// 2. 如果f提前关闭, cpu profile数据不会落地
	// 3. 如果不显示调用StopCPUProfile, cpu profile会一直在后台运行
	// 4. 如果瞬间停止cpu profile, 很可能没有信息被剖析
	// 5. StartCPUProfile不能多次调用, 已启用后再次调用会报错:cpu profiling already in use
	if err := pprof.StartCPUProfile(f); err != nil {
		return errors.Errorf("can not start cpu profile, error:%w ", err)
	}
	// close cpu profile after
	go func() {
		select {
		case <-time.After(timeout):
		case <-_profileChan:
		}
		pprof.StopCPUProfile()
		f.Close()
	}()

	return nil
}
