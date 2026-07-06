package validator

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"

	validator "github.com/go-playground/validator/v10"
	"github.com/wangweihong/gotoolbox/pkg/stringutil"
)

const (
	maxDescriptionLength = 256
	nameRegixPattern     = `^[a-zA-Z0-9]+(?:[_-][a-zA-Z0-9]+)*$`
)

// ValidateFile checks if a given string is an existing file.
func ValidateFile(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return true
	}

	return false
}

// ValidateDescription checks if a given description is illegal.
func ValidateDescription(fl validator.FieldLevel) bool {
	description := fl.Field().String()

	return len(description) <= maxDescriptionLength
}

// ValidateName checks if a given name is illegal.
func ValidateName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	re := regexp.MustCompile(nameRegixPattern)
	return re.MatchString(name)
}

// ValidatePort checks if a given port is illegal.
func ValidatePort(fl validator.FieldLevel) bool {
	port := fl.Field().Int()
	return port > 0 && port < 65535
}

// ValidatePort checks if a given port is illegal.
func ValidatePortUsed(fl validator.FieldLevel) bool {
	port := fl.Field().Int()
	address := ":" + strconv.Itoa(int(port))

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return false
	}
	tcpSocket, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return true
	}
	tcpSocket.Close()
	return false
}

// ValidatePort checks if a given port is illegal.
func ValidatePorts(fl validator.FieldLevel) bool {
	portInf := fl.Field().Interface()
	ports, ok := portInf.([]int)
	if ok {
		for _, port := range ports {
			if port < 0 || port > 65535 {
				return false
			}
		}
		return true
	}
	return false
}

// ValidateURL checks if a given url is illegal.
func ValidateURL(fl validator.FieldLevel) bool {
	name := fl.Field().String()

	//FIXME
	if !stringutil.HasAnyPrefix(name, "https:?/", "http://") {
		return false
	}

	return true
}

// // ValidateNamespaceScopeResource 检验命名空间级资源是否合法
// func ValidateNamespaceScopeResource(fl validator.FieldLevel) bool {
// 	if fl.Field().Interface() == nil {
// 		return false
// 	}

// 	// 如果是iapiserver.ResourceGetRequest结构则直接判断
// 	if gr, ok := fl.Field().Interface().(iapiserver.ResourceGetRequest); ok {
// 		return !(gr.Namespace == "" || gr.Name == "")
// 	}
// 	// 这里是因为之前没法找到获取检测的结构体字段临时想到的方法
// 	// fieldVal := fl.Field()
// 	// if fieldVal.Kind() != reflect.Ptr {
// 	// 	fieldVal = reflectutil.CreatePointerToValue(fl.Field())
// 	// }
// 	if !fl.Field().CanAddr() {
// 		return false
// 	}

// 	// 即使结构体中定义的指针,fl.Field()获得解引用的类型。因此需要通过fl.Field().Addr()
// 	cm, ok := fl.Field().Addr().Interface().(metav1.Object)
// 	if !ok || cm == nil {
// 		return false
// 	}

// 	if cm.GetNamespace() == "" || cm.GetName() == "" {
// 		return false
// 	}

// 	return true
// }

// // ValidateClusterScopeResource 检验集群级资源是否合法
// func ValidateClusterScopeResource(fl validator.FieldLevel) bool {
// 	if fl.Field().Interface() == nil {
// 		return false
// 	}

// 	if gr, ok := fl.Field().Interface().(iapiserver.ResourceGetRequest); ok {
// 		return !(gr.Name == "")
// 	}

// 	if !fl.Field().CanAddr() {
// 		return false
// 	}

// 	cm, ok := fl.Field().Addr().Interface().(metav1.Object)
// 	if !ok || cm == nil {
// 		return false
// 	}

// 	if cm.GetName() == "" {
// 		return false
// 	}

// 	return true
// }

// func ValidateBatchNamespaceScopeResource(fl validator.FieldLevel) bool {
// 	field := fl.Field()

// 	if field.Kind() != reflect.Slice {
// 		return false
// 	}

// 	for i := range field.Len() {
// 		elem := field.Index(i)

// 		elemValidator := validator.New()
// 		elemFl := elemValidator.FieldLevel(elem.Interface())

// 		// 对每个元素执行原验证逻辑
// 		if !ValidateNamespaceScopeResource(elemFl) {
// 			return false
// 		}
// 	}
// 	return true
// }

// func ValidateBatchClusterScopeResource(fl validator.FieldLevel) bool {
// 	field := fl.Field()

// 	if field.Kind() != reflect.Slice {
// 		return false
// 	}

// 	for i := 0; i < field.Len(); i++ {
// 		elem := field.Index(i)

// 		cm, ok := elem.Interface().(metav1.Object)
// 		if !ok || cm == nil {
// 			return false
// 		}

// 		if cm.GetName() == "" {
// 			return false
// 		}
// 	}
// 	return true
// }

const DNS1123LabelMaxLength int = 63
const dns1123LabelFmt string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
const dns1123SubdomainFmt string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"

var dns1123LabelRegexp = regexp.MustCompile("^" + dns1123LabelFmt + "$")

func IsDNS1123Label(value string) error {
	if value == "" {
		return fmt.Errorf("name is empty")
	}

	if len(value) > DNS1123LabelMaxLength {
		return fmt.Errorf("name too long")
	}
	if !dns1123LabelRegexp.MatchString(value) {
		return fmt.Errorf("not match name pattern: %v", dns1123LabelRegexp.String())
	}
	return nil
}

func ValidateDNSName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	return IsDNS1123Label(name) != nil
}

func ValidateCIDR(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	_, _, err := net.ParseCIDR(name)
	return err != nil
}

func ValidateIP(fl validator.FieldLevel) bool {
	ip := fl.Field().String()
	return net.ParseIP(ip) != nil
}

func ValidateIPs(fl validator.FieldLevel) bool {
	ipInf := fl.Field().Interface()
	ips, ok := ipInf.([]string)
	if ok {
		for _, ip := range ips {
			if net.ParseIP(ip) == nil {
				return false
			}
		}
		return true
	}
	return false
}
