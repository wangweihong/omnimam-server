package assetlibrary

import "testing"

func TestParseAssetSelector(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "label tag and group", input: `character.name=alice,#Hero,@group="Project A"`},
		{name: "or and set", input: `(style in (anime,photo);quality!=draft),!archived`},
		{name: "quoted reserved characters", input: `#"人物,汽车"`},
		{name: "unclosed group", input: `(style=anime`, wantErr: true},
		{name: "empty set value", input: `style in ()`, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expression, _, err := parseAssetSelector(test.input)
			if test.wantErr && err == nil {
				t.Fatalf("parseAssetSelector(%q) succeeded: %#v", test.input, expression)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("parseAssetSelector(%q): %v", test.input, err)
			}
		})
	}
}
