package identity

import (
	"crypto"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
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
		material, err = decodeOpaqueServerKeyMaterial(conf, encoded)
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

// decodeOpaqueServerKeyMaterial accepts the library's legal empty identity vector.
// bytemare/opaque v0.18.0 rejects that vector while decoding, although the server
// treats an empty identity as the public key according to RFC 9807.
func decodeOpaqueServerKeyMaterial(conf *opaque.Configuration, data []byte) (*opaque.ServerKeyMaterial, error) {
	if len(data) < 1 || opaque.Group(data[0]) != conf.AKE {
		return nil, fmt.Errorf("invalid OPAQUE server key group")
	}
	offset := 1
	readVector := func(allowEmpty bool) ([]byte, error) {
		if len(data)-offset < 2 {
			return nil, fmt.Errorf("invalid OPAQUE server key vector header")
		}
		length := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if len(data)-offset < length || (!allowEmpty && length == 0) {
			return nil, fmt.Errorf("invalid OPAQUE server key vector length")
		}
		value := append([]byte(nil), data[offset:offset+length]...)
		offset += length
		return value, nil
	}
	privateKeyBytes, err := readVector(false)
	if err != nil {
		return nil, err
	}
	publicKeyBytes, err := readVector(false)
	if err != nil {
		return nil, err
	}
	seed, err := readVector(true)
	if err != nil {
		return nil, err
	}
	identity, err := readVector(true)
	if err != nil || offset != len(data) {
		return nil, fmt.Errorf("invalid OPAQUE server key encoding")
	}
	privateKey, err := opaque.DeserializeScalar(conf.AKE.Group(), privateKeyBytes)
	if err != nil {
		return nil, err
	}
	publicKey, err := opaque.DeserializeElement(conf.AKE.Group(), publicKeyBytes)
	if err != nil {
		return nil, err
	}
	if !publicKey.Equal(conf.AKE.Group().Base().Multiply(privateKey)) {
		return nil, fmt.Errorf("OPAQUE server public key does not match private key")
	}
	if len(seed) != 0 && len(seed) != conf.Hash.Size() {
		return nil, fmt.Errorf("invalid OPAQUE OPRF seed length")
	}
	return &opaque.ServerKeyMaterial{PrivateKey: privateKey, PublicKeyBytes: publicKeyBytes, OPRFGlobalSeed: seed, Identity: identity}, nil
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
