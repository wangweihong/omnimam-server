package codec

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestAAA(t *testing.T) {
	data := `
/compile.sh: line 30: root/.bashrc: No such file or directory
/sbin/ldconfig.real: /usr/local/lib/librocksdb.so.6.14 is not a symbolic link

/sbin/ldconfig.real: /usr/local/lib/libiscsi.so.8 is not a symbolic link

/sbin/ldconfig.real: /lib/libcurl.so.4 is not a symbolic link

/sbin/ldconfig.real: /lib/libvirt-admin.so.0 is not a symbolic link

/sbin/ldconfig.real: /lib/libvirt-qemu.so.0 is not a symbolic link

/sbin/ldconfig.real: /lib/libnuma.so.1 is not a symbolic link

/sbin/ldconfig.real: /lib/libvirt-lxc.so.0 is not a symbolic link

/sbin/ldconfig.real: /lib/libvirt.so.0 is not a symbolic link

npm WARN using --force I sure hope you know what you are doing.
`

	scanner := bufio.NewScanner(bytes.NewBuffer([]byte(data)))
	for scanner.Scan() {
		line := scanner.Text()
		//	fmt.Println("line:", line)
		// 去除空白字符
		trimmedLine := strings.TrimSpace(line)

		// 判断是否是非空行或者包含"/sbin/ldconfig.real"
		if trimmedLine != "" && !strings.Contains(trimmedLine, "/sbin/ldconfig.real") &&
			!strings.Contains(trimmedLine, "/compile.sh: line 30") {
			fmt.Println(trimmedLine)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
}

func TestBB(t *testing.T) {
	d := "# test1 \n* 任务: 20  \n* 状态: <font color=red >失败</font>  \n* 持续时间: 17秒 \n* 问题链接: /projects/无/issues/无 \n* 发起人: 无 \n* 流程信息: \n\t* 流程(超融合#41) : <font color=red >失败</font>\n"
	fmt.Printf(d)
}
