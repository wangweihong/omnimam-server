package identity

import (
	"crypto"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/bytemare/ksf"
	"github.com/bytemare/opaque"
)

const (
	opaqueContext         = "omnimam/identity/opaque/v1"
	opaqueMaxMessageBytes = 16 * 1024
)

// OpaqueAdapter 是 Identity 对 OPAQUE 实现的消费方边界；它只接收协议二进制消息，不接收密码。
type OpaqueAdapter interface {
	RegistrationResponse(request, credentialIdentifier []byte) ([]byte, error)
	GenerateKE2(ke1, registrationRecord, credentialIdentifier []byte) ([]byte, []byte, error)
	VerifyKE3(ke3, serverMAC []byte) error
	FakeRecord(credentialIdentifier []byte) ([]byte, error)
	ValidateRegistrationRecord(record []byte) error
}

type opaqueServerAdapter struct {
	conf   *opaque.Configuration
	server *opaque.Server
}

func newOpaqueAdapter(setupHex string) (OpaqueAdapter, error) {
	conf := opaque.DefaultConfiguration()
	conf.Context = []byte(opaqueContext)
	conf.KSF = ksf.Argon2id
	conf.KDF = crypto.SHA512
	conf.MAC = crypto.SHA512
	conf.Hash = crypto.SHA512
	server, err := conf.Server()
	if err != nil {
		return nil, err
	}
	var material *opaque.ServerKeyMaterial
	if strings.TrimSpace(setupHex) != "" {
		encoded, err := hex.DecodeString(strings.TrimSpace(setupHex))
		if err != nil {
			return nil, err
		}
		material, err = conf.DecodeServerKeyMaterial(encoded)
		if err != nil {
			return nil, err
		}
	} else {
		privateKey, publicKey := conf.KeyGen()
		material = &opaque.ServerKeyMaterial{PrivateKey: privateKey, PublicKeyBytes: publicKey.Encode(), OPRFGlobalSeed: conf.GenerateOPRFSeed()}
	}
	if err := server.SetKeyMaterial(material); err != nil {
		return nil, err
	}
	return &opaqueServerAdapter{conf: conf, server: server}, nil
}

func (a *opaqueServerAdapter) RegistrationResponse(request, credentialIdentifier []byte) ([]byte, error) {
	if err := validateOpaqueMessage(request); err != nil {
		return nil, err
	}
	parsed, err := a.server.Deserialize.RegistrationRequest(request)
	if err != nil {
		return nil, err
	}
	response, err := a.server.RegistrationResponse(parsed, credentialIdentifier, nil)
	if err != nil {
		return nil, err
	}
	return response.Serialize(), nil
}

func (a *opaqueServerAdapter) GenerateKE2(ke1, registrationRecord, credentialIdentifier []byte) ([]byte, []byte, error) {
	if err := validateOpaqueMessage(ke1); err != nil {
		return nil, nil, err
	}
	if err := a.ValidateRegistrationRecord(registrationRecord); err != nil {
		return nil, nil, err
	}
	parsedKE1, err := a.server.Deserialize.KE1(ke1)
	if err != nil {
		return nil, nil, err
	}
	parsedRecord, err := a.server.Deserialize.RegistrationRecord(registrationRecord)
	if err != nil {
		return nil, nil, err
	}
	ke2, output, err := a.server.GenerateKE2(parsedKE1, &opaque.ClientRecord{RegistrationRecord: parsedRecord, CredentialIdentifier: credentialIdentifier})
	if err != nil {
		return nil, nil, err
	}
	return ke2.Serialize(), append([]byte(nil), output.ClientMAC...), nil
}

func (a *opaqueServerAdapter) VerifyKE3(ke3, serverMAC []byte) error {
	if err := validateOpaqueMessage(ke3); err != nil {
		return err
	}
	parsed, err := a.server.Deserialize.KE3(ke3)
	if err != nil {
		return err
	}
	return a.server.LoginFinish(parsed, serverMAC)
}

func (a *opaqueServerAdapter) FakeRecord(credentialIdentifier []byte) ([]byte, error) {
	record, err := a.conf.GetFakeRecord(credentialIdentifier)
	if err != nil {
		return nil, err
	}
	return record.RegistrationRecord.Serialize(), nil
}

func (a *opaqueServerAdapter) ValidateRegistrationRecord(record []byte) error {
	if err := validateOpaqueMessage(record); err != nil {
		return err
	}
	_, err := a.server.Deserialize.RegistrationRecord(record)
	return err
}

func validateOpaqueMessage(value []byte) error {
	if len(value) == 0 || len(value) > opaqueMaxMessageBytes {
		return errors.New("opaque message size is invalid")
	}
	return nil
}

func decodeOpaqueMessage(value string) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("opaque message is empty")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}
	if err := validateOpaqueMessage(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func encodeOpaqueMessage(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

var _ OpaqueAdapter = (*opaqueServerAdapter)(nil)
