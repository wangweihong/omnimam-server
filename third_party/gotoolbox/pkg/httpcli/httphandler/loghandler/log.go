package loghandler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
)

func RequestHandler(req *http.Request) (err error) {
	// 拷贝http request用于处理, 避免污染原请求
	reqClone := req.Clone(req.Context())
	if req.Body != nil {
		if isStream(req.Header) {
			reqClone.Body = ioutil.NopCloser(strings.NewReader("{stream: *****}"))
		} else {
			bodyBytes, err := ioutil.ReadAll(req.Body)
			if err != nil {
				return err
			}
			req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			reqClone.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		defer reqClone.Body.Close()
	}

	if err := logRequest(*reqClone); err != nil {
		log.Printf("[WARN] failed to get request body: %s", err)
	}

	return nil
}

func ResponseHandler(resp *http.Response) error {
	respClone := http.Response{
		Status:           resp.Status,
		StatusCode:       resp.StatusCode,
		Proto:            resp.Proto,
		ProtoMajor:       resp.ProtoMajor,
		ProtoMinor:       resp.ProtoMinor,
		Header:           resp.Header,
		ContentLength:    resp.ContentLength,
		TransferEncoding: resp.TransferEncoding,
		Close:            resp.Close,
		Uncompressed:     resp.Uncompressed,
		Trailer:          resp.Trailer,
	}

	if resp.Body != nil {
		if isStream(resp.Header) {
			respClone.Body = ioutil.NopCloser(strings.NewReader("{stream: *****}"))
		} else {
			bodyBytes, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			respClone.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		defer respClone.Body.Close()
	}

	if err := logResponse(respClone); err != nil {
		log.Printf("[WARN] failed to get response body: %s", err)
	}
	return nil
}

// logRequest will log the HTTP Request details.
// If the body is JSON, it will attempt to be pretty-formatted.
func logRequest(req http.Request) error {
	log.Printf("[DEBUG] API Request URL: %s %s", req.Method, req.URL)
	log.Printf("[DEBUG] API Request Headers:\n%s", FormatHeaders(req.Header, "\n"))

	contentType := req.Header.Get("Content-Type")
	if req.Body == nil {
		return nil
	}

	defer req.Body.Close()

	var bs bytes.Buffer
	_, err := io.Copy(&bs, req.Body)
	if err != nil {
		return err
	}

	body := bs.Bytes()
	index := findJSONIndex(body)
	if index == -1 {
		return nil
	}

	// Handle request contentType
	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := formatJSON(body[index:])
		log.Printf("[DEBUG] API Request Body: %s", debugInfo)
	} else {
		log.Printf("[DEBUG] Not logging because the request body isn't JSON")
	}

	return nil
}

// logResponse will log the HTTP Response details.
// If the body is JSON, it will attempt to be pretty-formatted.
func logResponse(resp http.Response) error {
	log.Printf("[DEBUG] API Response Code: %d", resp.StatusCode)
	log.Printf("[DEBUG] API Response Headers:\n%s", FormatHeaders(resp.Header, "\n"))

	contentType := resp.Header.Get("Content-Type")
	if resp.Body == nil {
		return nil
	}
	defer resp.Body.Close()

	var bs bytes.Buffer
	_, err := io.Copy(&bs, resp.Body)
	if err != nil {
		return err
	}

	body := bs.Bytes()
	index := findJSONIndex(body)
	if index == -1 {
		return nil
	}

	if strings.HasPrefix(contentType, "application/json") {
		debugInfo := formatJSON(body[index:])
		log.Printf("[DEBUG] API Response Body: %s", debugInfo)
	} else {
		log.Printf("[DEBUG] Not logging because the response body isn't JSON")
	}

	return nil
}

// FormatHeaders processes a headers object plus a deliminator, returning a string.
func FormatHeaders(headers http.Header, separator string) string {
	redactedHeaders := redactHeaders(headers)
	sort.Strings(redactedHeaders)

	return strings.Join(redactedHeaders, separator)
}

// redactHeaders processes a headers object, returning a redacted list.
func redactHeaders(headers http.Header) (processedHeaders []string) {
	// sensitiveWords is a list of headers that need to be redacted.
	sensitiveWords := []string{"token", "authorization"}

	for name, header := range headers {
		for _, v := range header {
			if isSliceContainsStr(sensitiveWords, name) {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, "***"))
			} else {
				processedHeaders = append(processedHeaders, fmt.Sprintf("%v: %v", name, v))
			}
		}
	}
	return
}

// formatJSON will try to pretty-format a JSON body.
// It will also mask known fields which contain sensitive information.
func formatJSON(raw []byte) string {
	var data map[string]any

	if len(raw) == 0 {
		return ""
	}

	err := json.Unmarshal(raw, &data)
	if err != nil {
		log.Printf("[DEBUG] Unable to parse JSON: %s", err)
		return string(raw)
	}

	pretty, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("[DEBUG] Unable to re-marshal JSON: %s", err)
		return string(raw)
	}

	return string(pretty)
}

func findJSONIndex(raw []byte) int {
	index := -1
	for i, v := range raw {
		if v == '{' {
			index = i
			break
		}
	}

	return index
}

func isSliceContainsStr(array []string, str string) bool {
	str = strings.ToLower(str)
	for _, s := range array {
		s = strings.ToLower(s)
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

func isStream(header http.Header) bool {
	contentType := header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	return strings.Contains(contentType, "octet-stream")
}
