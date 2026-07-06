package codec

import (
	"fmt"
	"strings"
)

type MapCodec interface {
	// Encode returns an encoded string representing the TrackedRequest.
	Encode(value map[string]any) (string, error)

	// Decode returns a Tracked request from an encoded string.
	Decode(signed string) (map[string]any, error)
}

type SimpleMapCodec struct {
}

var _ MapCodec = SimpleMapCodec{}

func (c SimpleMapCodec) Encode(value map[string]any) (string, error) {
	var pairs []string
	for k, v := range value {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(pairs, ","), nil
}

func (c SimpleMapCodec) Decode(s string) (map[string]any, error) {
	m := make(map[string]any)
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	return m, nil
}
