package tecentsms

import (
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20190711"
	gerrors "github.com/wangweihong/gotoolbox/pkg/errors"
	"github.com/wangweihong/gotoolbox/pkg/sets"
	"github.com/wangweihong/gotoolbox/pkg/sliceutil"
)

type Config struct {
	SecretID   string `json:"secret_id"`
	SecretKey  string `json:"secret_key"`
	SDKAppID   string `json:"sdk_app_id"`
	TemplateID string `json:"template_id"`
	Sign       string `json:"sign"`
}

var ipv6 = false

func Send(config Config, ep string, phones []string, TemplateArgs []string) (*sms.SendSmsResponse, error) {
	if !sets.NewString(phones...).AllHasPrefix("+86") {
		return nil, gerrors.Errorf("phone must container +86")
	}

	if sliceutil.StringSlice(TemplateArgs).LenExceedCount(12) > 0 {
		return nil, gerrors.Errorf("each template args length must not exceed 12 charactor")
	}

	if !ipv6 {
		ret, err := send(config, "", phones, TemplateArgs)
		if err != nil {
			ret, err = send(config, "sms.ipv6.tencentcloudapi.com", phones, TemplateArgs)
			if err == nil {
				ipv6 = true
				return ret, nil
			}
		}
		return ret, err
	} else {
		ret, err := send(config, "sms.ipv6.tencentcloudapi.com", phones, TemplateArgs)
		if err != nil {
			ret, err = send(config, "", phones, TemplateArgs)
			if err == nil {
				ipv6 = false
				return ret, nil
			}
		}
		return ret, err
	}
}

func send(config Config, ep string, phones []string, TemplateArgs []string) (*sms.SendSmsResponse, error) {
	// 必要步骤：
	// 实例化一个认证对象，入参需要传入腾讯云账户密钥对secretId，secretKey。
	// 这里采用的是从环境变量读取的方式，需要在环境变量中先设置这两个值。
	// 你也可以直接在代码中写死密钥对，但是小心不要将代码复制、上传或者分享给他人，
	// 以免泄露密钥对危及你的财产安全。
	credential := common.NewCredential(
		config.SecretID,
		config.SecretKey,
	)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.ReqMethod = "POST"
	/* SDK有默认的超时时间，非必要请不要进行调整
	 * 如有需要请在代码中查阅以获取最新的默认值 */
	//cpf.HttpProfile.ReqTimeout = 5

	/* SDK会自动指定域名。通常是不需要特地指定域名的，但是如果你访问的是金融区的服务
	 * 则必须手动指定域名，例如sms的上海金融区域名： sms.ap-shanghai-fsi.tencentcloudapi.com */
	cpf.HttpProfile.Endpoint = ep
	/* SDK默认用TC3-HMAC-SHA256进行签名，非必要请不要修改这个字段 */
	cpf.SignMethod = "HmacSHA1"
	client, _ := sms.NewClient(credential, "", cpf)
	request := sms.NewSendSmsRequest()
	/* 基本类型的设置:
	 * SDK采用的是指针风格指定参数，即使对于基本类型你也需要用指针来对参数赋值。
	 * SDK提供对基本类型的指针引用封装函数
	 * 帮助链接：
	 * 短信控制台: https://console.cloud.tencent.com/sms/smslist
	 * sms helper: https://cloud.tencent.com/document/product/382/3773 */

	/* 短信应用ID: 短信SdkAppid在 [短信控制台] 添加应用后生成的实际SdkAppid，示例如1400006666 */
	request.SmsSdkAppid = common.StringPtr(config.SDKAppID)
	/* 短信签名内容: 使用 UTF-8 编码，必须填写已审核通过的签名，签名信息可登录 [短信控制台] 查看 */
	//request.Sign = common.StringPtr("272132")
	request.Sign = common.StringPtr(config.Sign)
	/* 国际/港澳台短信 senderid: 国内短信填空，默认未开通，如需开通请联系 [sms helper] */
	//request.SenderId = common.StringPtr("xxx")
	/* 用户的 session 内容: 可以携带用户侧 ID 等上下文信息，server 会原样返回 */
	//request.SessionContext = common.StringPtr("xxx")
	/* 短信码号扩展号: 默认未开通，如需开通请联系 [sms helper] */
	//request.ExtendCode = common.StringPtr("0")
	/* 模板参数: 若无模板参数，则设置为空*/
	// 每一模板参数的长度不能超过12个字符
	request.TemplateParamSet = common.StringPtrs(TemplateArgs)
	/* 模板 ID: 必须填写已审核通过的模板 ID。模板ID可登录 [短信控制台] 查看 */
	request.TemplateID = common.StringPtr(config.TemplateID)
	/* 下发手机号码，采用 e.164 标准，+[国家或地区码][手机号]
	 * 示例如：+8613711112222， 其中前面有一个+号 ，86为国家码，13711112222为手机号，最多不要超过200个手机号*/
	request.PhoneNumberSet = common.StringPtrs(phones)
	// 通过client对象调用想要访问的接口，需要传入请求对象
	response, err := client.SendSms(request)
	// 处理异常
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		return nil, err
	}
	// 非SDK异常，直接失败。实际代码中可以加入其他的处理。
	if err != nil {
		return nil, err
	}
	return response, nil
}
