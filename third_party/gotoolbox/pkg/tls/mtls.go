package tls

type MTLSCert struct {
	GeneratableKeyCert `mapstructure:",squash"`

	ClientCAData string `json:"client-ca-data" mapstructure:"client-ca-data"`
	ClientCAPath string `json:"client-ca-path" mapstructure:"client-ca-path"`
}

// CopyAndHide deepcopy cert and hide cert and key.
func (s *MTLSCert) CopyAndHide() *MTLSCert {
	o := s.DeepCopy()
	if s.GeneratableKeyCert.CertData.Cert != "" {
		s.GeneratableKeyCert.CertData.Cert = "-"
	}

	if s.CertData.Key != "" {
		s.CertData.Key = "-"
	}

	if s.ClientCAData != "" {
		s.ClientCAData = "-"
	}
	return o
}

func (s *MTLSCert) deepCopyInto(out *MTLSCert) {
	*out = *s
	out.GeneratableKeyCert = *s.GeneratableKeyCert.DeepCopy()
}

func (s *MTLSCert) DeepCopy() *MTLSCert {
	if s == nil {
		return nil
	}
	out := new(MTLSCert)
	s.deepCopyInto(out)
	return out
}
