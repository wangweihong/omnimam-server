package def

// TODO: 支持application/x-www-form-urlencoded
// 参数需要进行url encode
// formData := url.Values{}
// formData.Set("_method", "post")
// formData.Set("authenticity_token", token)
// data := formData.Encode()
type URLFormData struct {
	KVs map[string]string
}
