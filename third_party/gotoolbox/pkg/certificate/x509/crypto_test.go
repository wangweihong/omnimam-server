package x509

import "testing"

var (
	_testSign  = "wwhvw"
	_wrongSign = "wrong"
)

func TestPrivateSignPublicVerify(t *testing.T) {
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

func TestPrivateSignPublicVerifyFail(t *testing.T) {
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
		t.Fatal("wrong sign pass")
	}
}

func TestPublicSignPrivateVerify(t *testing.T) {
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

func TestPublicSignPrivateVerifyFail(t *testing.T) {
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
		t.Fatal("wrong sign pass")
	}

}
