package saml_test

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSimulateSAMLResponseReturn(t *testing.T) {
	r := gin.Default()

	// 定义嵌入式的 HTML 模板
	const formHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>SAML Response</title>
</head>
<body>
    <form id="samlForm" method="post" action="{{ .ActionURL }}">
        <input type="hidden" name="SAMLResponse" value="{{ .SAMLResponse }}">
        <noscript>
            <p>JavaScript is disabled. Please click the Submit button below to continue.</p>
            <input type="submit" value="Submit">
        </noscript>
    </form>
    <script type="text/javascript">
        document.getElementById("samlForm").submit();
    </script>
</body>
</html>
	`

	// 创建模板
	tmpl, err := template.New("form").Parse(formHTML)
	if err != nil {
		panic(err)
	}

	r.LoadHTMLGlob("./testdata/*")
	// 路由处理
	r.GET("/saml-response", func(c *gin.Context) {
		// 模拟 SAML 响应数据 (通常是Base64编码的)
		samlResponse := "PHNhbWxwOlJlc3BvbnNlIE...base64-encoded-data..."

		// 返回一个 HTML 表单
		c.HTML(http.StatusOK, "form.html", gin.H{
			"SAMLResponse": samlResponse,
			"ActionURL":    "http://127.0.0.1:8080/saml/callback", // SP 的接收端点
		})
	})

	r.GET("/saml-response2", func(c *gin.Context) {
		// 模拟 SAML 响应数据 (通常是Base64编码的)
		samlResponse := "PHNhbWxwOlJlc3BvbnNlIE...base64-encoded-data..."

		// 使用内嵌模板生成 HTML 内容
		c.Status(http.StatusOK)
		c.Header("Content-Type", "text/html; charset=utf-8")
		err := tmpl.Execute(c.Writer, gin.H{
			"SAMLResponse": samlResponse,
			"ActionURL":    "https://127.0.0.1/saml/callback", // SP 的接收端点
		})
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to render template: %v", err)
		}
	})
	r.GET("/dashboard", func(c *gin.Context) {

	})
	r.POST("/saml/callback", func(c *gin.Context) {
		// 模拟从 SAML 响应中提取到的 token
		token := "example_token"

		// 渲染 HTML 页面，将 token 写入 localStorage 并重定向
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`
			<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<title>Redirecting...</title>
			</head>
			<body>
				<script type="text/javascript">
					// 将 token 写入 localStorage
					localStorage.setItem('authToken', '`+token+`');

					// 重定向到目标页面
					window.location.href = '/dashboard';
				</script>
				<noscript>
					<meta http-equiv="refresh" content="0;url=/dashboard" />
				</noscript>
			</body>
			</html>
		`))
	})

	r.POST("/test", func(c *gin.Context) {
		c.JSON(200, "ok")
	})
	// 运行服务器
	r.Run(":8080")
}

func TestAAA(t *testing.T) {
	d, _ := os.ReadFile("./testdata/myservice.cert")
	sd := fmt.Sprintf(string(base64.StdEncoding.EncodeToString(d)))

	fmt.Println(sd)
	d2, _ := os.ReadFile("./testdata/myservice.key")
	fmt.Println(string(base64.StdEncoding.EncodeToString(d2)))
}
