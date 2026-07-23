package taskname

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		params      map[string]string
		wantEnglish string
		wantChinese string
		wantError   bool
	}{
		{name: "fixed system name", key: AssetThumbnail, wantEnglish: "Generate asset thumbnail", wantChinese: "生成素材缩略图"},
		{name: "parameterized system name", key: RepresentationGenerate, params: map[string]string{"representation_type": "thumbnail"}, wantEnglish: "Generate thumbnail", wantChinese: "生成 thumbnail 表现形式"},
		{name: "unknown key", key: "unknown", wantError: true},
		{name: "missing parameter", key: RepresentationGenerate, wantError: true},
		{name: "unexpected parameter", key: AssetThumbnail, params: map[string]string{"extra": "value"}, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Resolve(tt.key, tt.params)
			if tt.wantError {
				if err == nil {
					t.Fatal("Resolve() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got[LanguageEnglish] != tt.wantEnglish || got[LanguageChinese] != tt.wantChinese {
				t.Fatalf("Resolve() = %#v", got)
			}
		})
	}
}

func TestResolveReturnsClone(t *testing.T) {
	first, err := Resolve(ApplicationRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	first[LanguageEnglish] = "changed"
	second, err := Resolve(ApplicationRun, nil)
	if err != nil {
		t.Fatal(err)
	}
	if second[LanguageEnglish] != "Application Platform Run" {
		t.Fatalf("catalog was mutated: %#v", second)
	}
}
