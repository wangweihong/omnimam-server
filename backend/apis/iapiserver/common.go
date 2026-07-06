package iapiserver

// +k8s:deepcopy-gen=true
type ClientTlsConfig struct {
	CaData string `json:"ca_data"`
}

func (c *ClientTlsConfig) DeepCopyInto(t *ClientTlsConfig) {
	t.CaData = c.CaData
}
