package excel_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	excel "github.com/wangweihong/gotoolbox/pkg/excel"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

type ServerCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type ThinTerminalCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	OS                 string    `json:"os" excel:"操作系统"`
	SNCode             string    `json:"sn_code" excel:"SN码"`
	CPUNum             int       `json:"cpu_num" excel:"CPU核数"`
	ExpandScreen       bool      `json:"expand_screen" excel:"扩展屏"`
	Resolution2K       bool      `json:"resolution_2k" excel:"2K"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type FatTerminalCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	OS                 string    `json:"os" excel:"操作系统"`
	SNCode             string    `json:"sn_code" excel:"SN码"`
	CPUNum             int       `json:"cpu_core" excel:"CPU核数"`
	MemorySize         uint64    `json:"memory_size" excel:"内存"`
	DiskSize           uint64    `json:"disk_size" excel:"磁盘"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type DiskCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Type               string    `json:"type" excel:"类型"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	InterfaceType      string    `json:"interface_type" excel:"接口类型"`
	CacheDiskSupport   bool      `json:"cache_disk_support" excel:"缓存盘支持"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type CPUCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	CoreNum            int       `json:"core_num" excel:"核数"`
	ClockSpeed         string    `json:"clock_speed" excel:"主频"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type MemoryCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	Size               uint64    `json:"size" excel:"内存容量"`
	Specification      string    `json:"specification" excel:"内存规格"`
	Speed              string    `json:"speed" excel:"内存速度"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type RaidCardCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	JbodDirectPassage  bool      `json:"jbod_direct_passage" excel:"jbody直通"`
	FirmwareVersion    string    `json:"firmware_version" excel:"固件版本"`
	DriverVersion      string    `json:"driver_version" excel:"驱动版本"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type HBACardCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	FirmwareVersion    string    `json:"firmware_version" excel:"固件版本"`
	DriverVersion      string    `json:"driver_version" excel:"驱动版本"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type NetworkCardCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	FirmwareVersion    string    `json:"firmware_version" excel:"固件版本"`
	DriverVersion      string    `json:"driver_version" excel:"驱动版本"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type GPUCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	CoreNum            int       `json:"core_num" excel:"核心数"`
	SupportVGPU        bool      `json:"support_vgpu" excel:"VGPU支持"`
	FirmwareVersion    string    `json:"firmware_version" excel:"固件版本"`
	DriverVersion      string    `json:"driver_version" excel:"驱动版本"`
	CUDANum            int       `json:"cuda_num" excel:"CUDA数"`
	VRAMSize           uint64    `json:"vram_size" excel:"显存容量"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type VirtualMachineOSCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	Support3D          bool      `json:"support_3d" excel:"支持3D"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type TerminalOSCompatibilityData struct {
	ID                  int       `json:"id" pg:",pk" excel:"序号"`
	ProductType         string    `json:"product_type" excel:"产品类型"`
	ProductNumber       string    `json:"product_number" excel:"产品型号"`
	Manufacturer        string    `json:"manufacturer" excel:"厂商"`
	Architecture        string    `json:"architecture" excel:"CPU架构"`
	TerminalSupportList []string  `json:"terminal_support_list" excel:"终端支持列表"`
	Compatibility       bool      `json:"compatibility" excel:"兼容情况"`
	Verified            bool      `json:"verified" excel:"是否验证"`
	UpdateTime          time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions  []string  `json:"compatible_versions" excel:"兼容版本"`
}

type TerminalSoftwareCompatibilityData struct {
	ID                     int       `json:"id" pg:",pk" excel:"序号"`
	ProductType            string    `json:"product_type" excel:"产品类型"`
	ProductNumber          string    `json:"product_number" excel:"产品型号"`
	Manufacturer           string    `json:"manufacturer" excel:"厂商"`
	Architecture           string    `json:"architecture" excel:"CPU架构"`
	Type                   string    `json:"type" excel:"类型"`
	Version                string    `json:"version" excel:"版本"`
	PlatformVersionSupport []string  `json:"platform_version_support" excel:"平台版本支持列表"`
	Compatibility          bool      `json:"compatibility" excel:"兼容情况"`
	Verified               bool      `json:"verified" excel:"是否验证"`
	UpdateTime             time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions     []string  `json:"compatible_versions" excel:"兼容版本"`
}

type SoftwareDistributeCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	Type               string    `json:"type" excel:"软件类型"`
	OsType             string    `json:"os_type" excel:"支持操作系统"`
	DefaultInstallPath string    `json:"default_install_path" excel:"默认安装路径"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

type ApplicationSoftwareCompatibilityData struct {
	ID                 int       `json:"id" pg:",pk" excel:"序号"`
	ProductType        string    `json:"product_type" excel:"产品类型"`
	ProductNumber      string    `json:"product_number" excel:"产品型号"`
	Manufacturer       string    `json:"manufacturer" excel:"厂商"`
	Architecture       string    `json:"architecture" excel:"CPU架构"`
	Type               string    `json:"type" excel:"软件类型"`
	OsType             string    `json:"os_type" excel:"支持操作系统"`
	DefaultInstallPath string    `json:"default_install_path" excel:"默认安装路径"`
	Compatibility      bool      `json:"compatibility" excel:"兼容情况"`
	Verified           bool      `json:"verified" excel:"是否验证"`
	UpdateTime         time.Time `json:"update_time" excel:"更新时间"`
	CompatibleVersions []string  `json:"compatible_versions" excel:"兼容版本"`
}

func parseBool(value string, _ reflect.Type) (interface{}, error) {
	// 支持空字符串、大小写、中英文
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "t", "1", "是", "yes", "y", "兼容", "验证", "支持":
		return true, nil
	case "false", "f", "0", "否", "no", "n", "":
		return false, nil
	default:
		return nil, fmt.Errorf("invalid bool value: %s", value)
	}
}

func TestImport(t *testing.T) {
	Convey("TestSetField", t, func() {
		//d:=VirtualMachineOSCompatibilityData
		registry := excel.NewimportParserRegistry()
		registry.RegisterFieldParser("VirtualMachineOSCompatibilityData", "CompatibleVersions", func(value string, _ reflect.Type) (interface{}, error) {
			// 特殊的分隔符处理
			if value == "" || value == "[]" {
				return []string{}, nil
			}
			value = stringutil.TrimAnySuffix(stringutil.TrimAnyPrefix(value, "["), "]")
			return strings.Split(value, ","), nil
		})
		//增加类型注册器
		registry.RegisterTypeParser(reflect.TypeOf(true), parseBool)
		registry.RegisterTypeParser(reflect.TypeOf(time.Time{}), excel.ParseTime)

		d, err := excel.ImportFromFile[VirtualMachineOSCompatibilityData](context.Background(), registry, "虚拟机OS", "./testdata/test.xlsx")
		So(err, ShouldBeNil)
		So(len(d), ShouldEqual, 1)
		So(d[0].Verified, ShouldBeTrue)
		t.Log(d)
	})
}

func TestExport(t *testing.T) {
	Convey("TestExport", t, func() {
		os.Remove("./testdata/output.xlsx")
		d := VirtualMachineOSCompatibilityData{
			ID:                 123,
			ProductType:        "测试",
			ProductNumber:      "abc",
			Manufacturer:       "huawei",
			Architecture:       "",
			Support3D:          true,
			Compatibility:      true,
			Verified:           true,
			UpdateTime:         time.Now(),
			CompatibleVersions: nil,
		}
		exportRegistry := excel.NewExportRegistry()
		exportRegistry.RegisterFieldExporter(
			reflect.TypeOf(VirtualMachineOSCompatibilityData{}),
			"Support3D",
			func(value interface{}) (string, error) {
				price := value.(bool)
				if price {
					return "支持", nil
				}
				return "不支持", nil
			},
		)
		exportRegistry.RegisterTypeExporter(
			reflect.TypeOf(""),
			func(value interface{}) (string, error) {
				price := value.(string)
				if price == "" {
					return "N/A", nil
				}
				return price, nil
			},
		)
		err := excel.ExportToFile(context.Background(), exportRegistry, "./testdata/output.xlsx", []VirtualMachineOSCompatibilityData{d}, "虚拟机OS", "兼容版本")
		So(err, ShouldBeNil)
	})
}
