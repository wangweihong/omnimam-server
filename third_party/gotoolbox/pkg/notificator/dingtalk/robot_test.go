package dingtalk_test

import (
	"fmt"
	"testing"

	"github.com/wangweihong/gotoolbox/pkg/notificator/dingtalk"
)

func TestAAA(t *testing.T) {

	markdown := dingtalk.Markdown{}
	markdown.Title = "master分支自动编译打包流程-x86"
	markdown.Text = fmt.Sprintf("# %v \n"+
		"* 任务: %v  \n"+
		"* 状态: %v  \n"+
		"* 持续时间: %v \n"+
		"* 问题链接: %v \n"+
		"* 发起人: %v \n",
		"测试通知",
		"任务1",
		"<font color=green >成功</font>",
		"30s",
		"www.baidu.com",
		"test",
	)
	dintalkURL := "https://oapi.dingtalk.com/robot/send?access_token=75ae9f56085e148bb99e4917276deee6ebb0ba7fa44b33b3b2fff116d647976c"
	//secret := "SEC53ba637dc10ae242a8a99bb66c15dc386a87de9918f6ac7cd24e19a130612a61"
	secret := ""
	keyword := "状态"
	if err := dingtalk.SendRobot(dingtalk.NewMarkdownMessage(markdown), dintalkURL, secret, keyword); err != nil {
		t.Fatal(err)
	}
}
