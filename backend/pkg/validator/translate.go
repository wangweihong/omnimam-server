package validator

import "fmt"

type Translator interface {
	ZH() string
	EN() string
}

type NameTranslator struct{}

func (t NameTranslator) ZH() string {
	return fmt.Sprintf("必须匹配正则表达式 %s", nameRegixPattern)
}

func (t NameTranslator) EN() string {
	return fmt.Sprintf("name must match pattern:%s", nameRegixPattern)
}

type DescriptionTranslator struct{}

func (t DescriptionTranslator) ZH() string {
	return fmt.Sprintf("长度必须小于%d个字符", maxDescriptionLength)
}

func (t DescriptionTranslator) EN() string {
	return fmt.Sprintf("must be less than %d characters", maxDescriptionLength)
}

type IDTranslator struct{}

func (t IDTranslator) ZH() string {
	return "未指定对象"
}

func (t IDTranslator) EN() string {
	return "id missing"
}

type URLInvaliTranslator struct{}

func (t URLInvaliTranslator) ZH() string {
	return "地址不正确"
}

func (t URLInvaliTranslator) EN() string {
	return "wrong http url"
}

type PortRangeTranslator struct{}

func (t PortRangeTranslator) ZH() string {
	return "无效的端口号,有效范围(0,63335)"
}

func (t PortRangeTranslator) EN() string {
	return "invalid port, valid range (0,65536)"
}

type PortsRangeTranslator struct{}

func (t PortsRangeTranslator) ZH() string {
	return "无效的端口号,有效范围(0,63335)"
}

func (t PortsRangeTranslator) EN() string {
	return "invalid port, valid range (0,65536)"
}

type PortUsedTranslator struct{}

func (t PortUsedTranslator) ZH() string {
	return "端口已被使用"
}

func (t PortUsedTranslator) EN() string {
	return "port has been used"
}

type DNSTranslator struct{}

func (t DNSTranslator) ZH() string {
	return "无效的DNS标签"
}

func (t DNSTranslator) EN() string {
	return "invalid dns label"
}

type CIDRTranslator struct{}

func (t CIDRTranslator) ZH() string {
	return "无效的CIDR"
}

func (t CIDRTranslator) EN() string {
	return "invalid cidr"
}
