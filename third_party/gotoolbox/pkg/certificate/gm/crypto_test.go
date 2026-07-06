package gm

import "testing"

var (
	_testSign  = "wwhvw"
	_wrongSign = "wrong"
)

func TestGMPrivateSignPublicVerify(t *testing.T) {
	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	gmCrypto, err := NewCertificateCrypto(key)
	if err != nil {
		t.Fatal(err)
	}

	signature, err := gmCrypto.PrivateSign([]byte(_testSign))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("signature:%v", string(signature))
	if err := gmCrypto.PublicVerify([]byte(_testSign), signature); err != nil {
		t.Fatal(err)
	}

}

func TestGMPrivateSignPublicVerifyFail(t *testing.T) {
	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	gmCrypto, err := NewCertificateCrypto(key)
	if err != nil {
		t.Fatal(err)
	}

	signature, err := gmCrypto.PrivateSign([]byte(_testSign))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("signature:%v", string(signature))
	if err := gmCrypto.PublicVerify([]byte(_wrongSign), signature); err == nil {
		t.Fatal("wrong test verify pass")
	}

}

func TestGMPublicSignPrivateVerify(t *testing.T) {
	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	gmCrypto, err := NewCertificateCrypto(key)
	if err != nil {
		t.Fatal(err)
	}

	signature, err := gmCrypto.PublicSign([]byte(_testSign))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("signature:%v", string(signature))
	if err := gmCrypto.PrivateVerify([]byte(_testSign), signature); err != nil {
		t.Fatal(err)
	}

}

func TestGMPublicSignPrivateVerifyFail(t *testing.T) {
	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	gmCrypto, err := NewCertificateCrypto(key)
	if err != nil {
		t.Fatal(err)
	}

	signature, err := gmCrypto.PublicSign([]byte(_testSign))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("signature:%v", string(signature))
	if err := gmCrypto.PrivateVerify([]byte(_wrongSign), signature); err == nil {
		t.Fatal("wrong signature pass")
	}
}
