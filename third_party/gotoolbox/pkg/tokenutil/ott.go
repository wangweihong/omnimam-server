package tokenutil

import (
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/wangweihong/gotoolbox/pkg/randutil"
)

type OneTimeTokenManager struct {
	data  map[string]struct{}
	lock  sync.Mutex
	codec TrackedRequestCodec
}

func NewOneTimeTokenManager(codec TrackedRequestCodec) *OneTimeTokenManager {
	return &OneTimeTokenManager{
		data:  make(map[string]struct{}),
		lock:  sync.Mutex{},
		codec: codec,
	}
}

func (m *OneTimeTokenManager) Get(signedString string) (*TrackedRequest, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exist := m.data[signedString]; !exist {
		return nil, fmt.Errorf("signed token not exist")
	}
	// remove after use
	delete(m.data, signedString)

	trackedRequest, err := m.codec.Decode(signedString)
	if err != nil {
		return nil, err
	}
	return trackedRequest, nil
}

func (m *OneTimeTokenManager) Put(data any) (string, error) {
	trackedRequest := TrackedRequest{
		Index: base64.RawURLEncoding.EncodeToString(randutil.RandBytes(42)),
		Value: data,
	}
	signedString, err := m.codec.Encode(trackedRequest)
	if err != nil {
		return "", err
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	m.data[signedString] = struct{}{}
	return signedString, nil
}
