package httpcli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/filereceiver"
	"github.com/wangweihong/gotoolbox/pkg/mathutil"

	"github.com/wangweihong/gotoolbox/pkg/httpcli/def"

	"github.com/wangweihong/gotoolbox/pkg/httpcli"

	. "github.com/smartystreets/goconvey/convey"
)

func TestHttpRequest_GetURL(t *testing.T) {
	Convey("TestHttpRequest_GetURL", t, func() {
		Convey("不带协议", func() {
			r := httpcli.NewHttpRequestBuilder().
				WithEndpoint("www.example.com").
				WithPath("/{path}").
				AddQueryParam("q", 123).AddQueryParam("q2", true).
				AddPathParam("path", "abc").Build()
			So(r.GetFullRequestAddress(), ShouldEqual, "http://www.example.com/abc?q=123&q2=true")
		})
		Convey("http协议", func() {
			r := httpcli.NewHttpRequestBuilder().
				WithEndpoint("www.example.com").
				WithPath("/{path}").
				AddQueryParam("q", 123).AddQueryParam("q2", true).
				AddPathParam("path", "abc").Build()
			So(r.GetFullRequestAddress(), ShouldEqual, "http://www.example.com/abc?q=123&q2=true")
		})
		Convey("https协议", func() {
			r := httpcli.NewHttpRequestBuilder().
				WithEndpoint("https://www.example.com").
				WithPath("/{path}").
				AddQueryParam("q", 123).AddQueryParam("q2", true).
				AddPathParam("path", "abc").Build()
			So(r.GetFullRequestAddress(), ShouldEqual, "https://www.example.com/abc?q=123&q2=true")
		})
	})
}

func TestHttpRequest_Invoke(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		// 模拟响应
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, world!"))
	}))
	defer server.Close()

	Convey("TestHttpRequest_Invoke", t, func() {
		Convey("no timeout", func() {
			resp, err := httpcli.NewHttpRequestBuilder().
				GET().
				WithEndpoint(server.URL).
				Build().
				Invoke()
			So(err, ShouldBeNil)
			So(resp.GetBody(), ShouldEqual, "Hello, world!")
		})
		Convey("timeout", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := httpcli.NewHttpRequestBuilder().
				GET().
				WithEndpoint(server.URL).
				Build().
				InvokeWithContext(ctx)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "context deadline exceeded")
			So(resp, ShouldBeNil)
		})
	})
}

func rawUpload(f *os.File, targetURL string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base("./testdata/upload_file"))
	if err != nil {
		return err
	}

	if _, err := io.Copy(part, f); err != nil {
		return err
	}

	if err := writer.WriteField("filename", "upload_file"); err != nil {
		return err
	}

	// 不能使用defer close, 否则会报multipart: NextPart: EO
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", targetURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(responseBody))
	return nil
}

func TestUploadFileFromFormData(t *testing.T) {
	// 通过表单接收文件
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析查询参数
		queryParam := r.URL.Query().Get("param")
		fmt.Printf("Query Parameter: %s\n", queryParam)

		if strings.Contains(r.Header.Get("Content-Type"), "multipart\\/form-data; boundary=") {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, fmt.Sprintf("content-type %v doesn't contain %v", r.Header.Get("Content-Type"), "multipart/form-data; boundary="))
			return
		}
		// 解析上传的文件
		r.ParseMultipartForm(10 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error Retrieving the File\"\n")
			return
		}
		defer file.Close()
		fmt.Printf("Uploaded File: %+v\n", handler.Filename)
		fmt.Printf("File Size: %+v\n", handler.Size)
		fmt.Printf("MIME Header: %+v\n", handler.Header)

		dst := &bytes.Buffer{}
		if _, err := io.Copy(dst, file); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error saving the file\n")
			return
		}

		expect := "Upload from form data"
		if dst.String() != expect {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, fmt.Sprintf("file data retrive, expect %v, actual:%v", expect, dst.String()))
			return
		}

		//dst, err := os.Create(handler.Filename)
		//if err != nil {
		//	fmt.Println("Error creating the file")
		//	fmt.Println(err)
		//	return
		//}
		//defer dst.Close()
		//
		//if _, err := io.Copy(dst, file); err != nil {
		//	fmt.Println("Error saving the file")
		//	fmt.Println(err)
		//}

		fmt.Fprintf(w, "Successfully Uploaded File\n")
	}))
	defer server.Close()

	Convey("通过表单上传文件", t, func() {
		Convey("httpcli通过FormParam上传文件", func() {
			f, err := os.Open("./testdata/upload_file")
			So(err, ShouldBeNil)
			defer f.Close()

			r, err := httpcli.NewHttpRequestBuilder().
				POST().
				// 表单字段
				AddFormParam("filename", def.NewMultiPart("upload_file")).
				// 表单文件
				AddFormParam("file", def.NewFilePart(f)).
				WithEndpoint(server.URL).Build().
				Invoke()
			So(err, ShouldBeNil)
			So(r.GetBody(), ShouldResemble, "Successfully Uploaded File\n")
		})

		Convey("httpcli通过WithBody上传文件1", func() {
			f, err := os.Open("./testdata/upload_file")
			So(err, ShouldBeNil)
			defer f.Close()

			type FormData struct {
				// 表单文件
				File *def.FilePart `json:"file"`
				// 表单字段
				FileName *def.MultiPart `json:"filename"`
			}

			fd := FormData{
				File:     def.NewFilePart(f),
				FileName: def.NewMultiPart("upload_file"),
			}

			r, err := httpcli.NewHttpRequestBuilder().
				POST().
				WithBody("multipart", fd).
				WithEndpoint(server.URL).Build().
				Invoke()
			So(err, ShouldBeNil)
			So(r.GetBody(), ShouldResemble, "Successfully Uploaded File\n")
		})

		Convey("httpcli通过WithBody上传文件2", func() {
			f, err := os.Open("./testdata/upload_file")
			So(err, ShouldBeNil)
			defer f.Close()

			type FormData struct {
				// 表单文件
				File *os.File `json:"file"`
				// 表单字段
				FileName string `json:"filename"`
			}

			fd := FormData{
				File:     f,
				FileName: "upload_file",
			}

			r, err := httpcli.NewHttpRequestBuilder().
				POST().
				WithBody("multipart", fd).
				WithEndpoint(server.URL).Build().
				Invoke()
			So(err, ShouldBeNil)
			So(r.GetBody(), ShouldResemble, "Successfully Uploaded File\n")
		})

		Convey("原始HTTP请求", func() {
			f, err := os.Open("./testdata/upload_file")
			So(err, ShouldBeNil)
			defer f.Close()

			err = rawUpload(f, server.URL)
			So(err, ShouldBeNil)
		})
	})
}

func TestUploadChunkFileFromFormData(t *testing.T) {
	fs, _ := os.Stat("./testdata/bigfile")
	fr, err := filereceiver.NewFileReceiver("./testdata", "generate", fs.Size(), int64(256))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析上传的文件
		r.ParseMultipartForm(10 << 20)
		file, handler, err := r.FormFile("file")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error Retrieving the File\"\n")
			return
		}
		defer file.Close()
		partNumber := r.FormValue("partNumber")

		fmt.Printf("Uploaded File: %+v\n", handler.Filename)
		fmt.Printf("File Size: %+v\n", handler.Size)
		fmt.Printf("MIME Header: %+v\n", handler.Header)

		dst := &bytes.Buffer{}
		if _, err := io.Copy(dst, file); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error saving the file\n")
			return
		}

		index := mathutil.MustParseNum[int64](int64(0), partNumber)
		if err := fr.Receive(dst.Bytes(), index); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, err.Error())
			return
		}

		fmt.Fprintf(w, "Successfully Uploaded File\n")
	}))
	defer server.Close()

	Convey("通过表单分片分别上传文件切片，再封装成文件", t, func() {
		Convey("httpcli通过FormParam上传文件", func() {
			So(fs, ShouldNotBeNil)
			So(err, ShouldBeNil)
			So(fr, ShouldNotBeNil)
			f, err := os.Open("./testdata/bigfile")
			So(err, ShouldBeNil)
			defer f.Close()

			buffer := make([]byte, 256)
			partNumber := 0
			for {
				bytesRead, err := f.Read(buffer)
				//
				//if err != nil && err != io.EOF {
				//	return err
				//}
				if bytesRead == 0 {
					break
				}

				chunk := buffer[:bytesRead]
				_, err = httpcli.NewHttpRequestBuilder().
					POST().
					// 表单字段
					AddFormParam("partNumber", def.NewMultiPart(fmt.Sprintf("%d", partNumber))).
					// 表单文件
					AddFormParam("file", def.NewFilePartitionPart("generated", chunk)).
					WithEndpoint(server.URL).Build().
					Invoke()
				partNumber++

				So(err, ShouldBeNil)
			}
		})
	})
}

func TestUploadFormFile(t *testing.T) {
	// 接收文件
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dst, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error read the file\n")
			return
		}

		expect := "Upload from form data"
		if string(dst) != expect {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, fmt.Sprintf("file data retrive, expect %v, actual:%v", expect, string(dst)))
			return
		}

		fmt.Fprintf(w, "Successfully Uploaded File\n")
	}))
	defer server.Close()

	Convey("通过body上传文件", t, func() {
		f, err := os.Open("./testdata/upload_file")
		So(err, ShouldBeNil)
		defer f.Close()

		r, err := httpcli.NewHttpRequestBuilder().
			POST().
			WithBody("", *f).
			WithEndpoint(server.URL).Build().
			Invoke()
		So(err, ShouldBeNil)
		So(r.GetBody(), ShouldResemble, "Successfully Uploaded File\n")
	})
}

func TestHTTPS(t *testing.T) {
	// 接收文件
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Successfully Uploaded File\n")
	}))
	server.StartTLS()
	defer server.Close()

	Convey("服务器开启了TLS", t, func() {
		Convey("不跳过证书检测", func() {
			_, err := httpcli.NewHttpRequestBuilder().
				POST().
				WithEndpoint(server.URL).Build().
				Invoke(httpcli.CallOptionInsecure())
			So(err, ShouldNotBeNil)
		})
		Convey("跳过证书检测", func() {
			r, err := httpcli.NewHttpRequestBuilder().
				POST().
				WithEndpoint(server.URL).Build().
				Invoke(httpcli.CallOptionInsecure())
			So(err, ShouldBeNil)
			So(r.GetBody(), ShouldResemble, "Successfully Uploaded File\n")
		})
	})
}
