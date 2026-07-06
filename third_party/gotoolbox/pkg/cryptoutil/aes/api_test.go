package aes

import "testing"

const (
	origin = "FRVITLHUKPROHKBU"
	//encrypted = "b7f4e6ac9de22516a4c3f6ef72db075a"
)

func TestEncrpytPassword(t *testing.T) {
	pen, err := EncyptPassword(origin)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before encrypt:%v, after:%v", origin, pen)

	p, err := DecryptPassword(pen)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before decrypt:%v, after:%v", pen, p)

	if origin != p {
		t.Logf("%v not match after encrypt/decrypt: %v", origin, p)
		t.Fail()
	}

}

func TestEncrpytPassword2(t *testing.T) {
	test := "GXQNDTBWOZYNSVYS"
	pen, err := EncyptPassword(test)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("before encrypt:%v, after:%v", test, pen)

	p, err := DecryptPassword("e6441d8af9e438f8f745961f2efb1fce")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("after encrypt:%v, before:%v", test, p)
}

func TestEncrpytPassword3(t *testing.T) {

	p, err := EncyptPassword("e6441d8af9e438f8f745961f2efb1fce")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(p)
}
