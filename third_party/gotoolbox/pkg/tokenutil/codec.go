package tokenutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/wangweihong/gotoolbox/pkg/errors"
)

type Options struct {
	URL url.URL
	//Key           *rsa.PrivateKey
	Key           any
	MaxIssueDelay time.Duration
}

var MaxIssueDelay = time.Second * 90

func DefaultJWTTrackedRequestCodec(opts Options) JWTTrackedRequestCodec {
	delay := MaxIssueDelay
	if opts.MaxIssueDelay != 0 {
		delay = opts.MaxIssueDelay
	}
	return JWTTrackedRequestCodec{
		SigningMethod: DefaultJWTSigningMethod,
		Audience:      opts.URL.String(),
		Issuer:        opts.URL.String(),
		MaxAge:        delay,
		Key:           opts.Key,
	}
}

func NewRSAJWTCodec(key *rsa.PrivateKey, maxAge time.Duration) JWTTrackedRequestCodec {
	if maxAge == 0 {
		maxAge = MaxIssueDelay
	}
	if key == nil {
		key, _ = rsa.GenerateKey(rand.Reader, 256)
	}

	return JWTTrackedRequestCodec{
		SigningMethod: jwt.SigningMethodRS256,
		Audience:      "",
		Issuer:        "",
		MaxAge:        maxAge,
		Key:           key,
	}
}

func NewHMACJWTCodec(key []byte, maxAge time.Duration) JWTTrackedRequestCodec {
	if maxAge == 0 {
		maxAge = MaxIssueDelay
	}
	if key == nil {
		hmacKey := make([]byte, 32)
		rand.Read(hmacKey)
		key = hmacKey
	}

	return JWTTrackedRequestCodec{
		SigningMethod: jwt.SigningMethodHS256,
		Audience:      "",
		Issuer:        "",
		MaxAge:        maxAge,
		Key:           key,
	}
}

func NewECDSAJWTCodec(key *ecdsa.PrivateKey, maxAge time.Duration) JWTTrackedRequestCodec {
	if maxAge == 0 {
		maxAge = MaxIssueDelay
	}

	if key == nil {
		key, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	return JWTTrackedRequestCodec{
		SigningMethod: jwt.SigningMethodES256,
		Audience:      "",
		Issuer:        "",
		MaxAge:        maxAge,
		Key:           key,
	}
}

type TrackedRequest struct {
	// 随机ID, 用于追踪每个请求
	Index string `json:"-"`
	Value any    `json:"value"`
}

// TrackedRequestCodec handles encoding and decoding of a TrackedRequest.
type TrackedRequestCodec interface {
	// Encode returns an encoded string representing the TrackedRequest.
	Encode(value TrackedRequest) (string, error)

	// Decode returns a Tracked request from an encoded string.
	Decode(signed string) (*TrackedRequest, error)
}

var DefaultJWTSigningMethod = jwt.SigningMethodRS256

// JWTTrackedRequestCodec encodes TrackedRequests as signed JWTs
type JWTTrackedRequestCodec struct {
	SigningMethod jwt.SigningMethod
	Audience      string
	Issuer        string
	MaxAge        time.Duration
	//Key           *rsa.PrivateKey
	// rsa.PrivateKey, ecdsa.PrivateKey,[]byte
	Key any
}

var _ TrackedRequestCodec = JWTTrackedRequestCodec{}

// JWTClaims represents the JWT claims for a tracked request.
type JWTClaims struct {
	jwt.RegisteredClaims
	TrackedRequest
}

// Encode returns an encoded string representing the TrackedRequest.
func (s JWTTrackedRequestCodec) Encode(value TrackedRequest) (string, error) {
	now := time.Now().UTC()

	if s.Key == nil {
		return "", errors.Errorf("codec key is nil")
	}
	claims := JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Audience:  jwt.ClaimStrings{s.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(s.MaxAge)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    s.Issuer,
			NotBefore: jwt.NewNumericDate(now),
			Subject:   value.Index,
		},
		TrackedRequest: value,
	}
	token := jwt.NewWithClaims(s.SigningMethod, claims)
	return token.SignedString(s.Key)
}

// Decode returns a Tracked request from an encoded string.
func (s JWTTrackedRequestCodec) Decode(signed string) (*TrackedRequest, error) {
	parser := jwt.Parser{
		ValidMethods: []string{s.SigningMethod.Alg()},
	}
	claims := JWTClaims{}
	if s.Key == nil {
		return nil, errors.Errorf("codec key is nil")
	}
	_, err := parser.ParseWithClaims(signed, &claims, func(*jwt.Token) (any, error) {
		switch k := s.Key.(type) {
		case []byte:
			return k, nil
		case *rsa.PrivateKey:
			return k.Public(), nil
		case *ecdsa.PrivateKey:
			return k.Public(), nil
		default:
			return nil, errors.Errorf("unsupported private key type")
		}
		// return s.Key.Public(), nil
	})
	if err != nil {
		return nil, err
	}
	if !claims.VerifyAudience(s.Audience, false) {
		return nil, fmt.Errorf("expected audience %q, got %q", s.Audience, claims.Audience)
	}

	if !claims.VerifyIssuer(s.Issuer, false) {
		return nil, fmt.Errorf("expected issuer %q, got %q", s.Issuer, claims.Issuer)
	}

	claims.TrackedRequest.Index = claims.Subject
	return &claims.TrackedRequest, nil
}
