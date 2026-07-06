package genericoptions

import (
	"github.com/spf13/pflag"
)

// CacheOptions defines options for cache .
type CacheOptions struct {
}

// NewCacheOptions create a `zero` value instance.
func NewCacheOptions() *CacheOptions {
	return &CacheOptions{}
}

// Validate verifies flags passed to CacheOptions.
func (o *CacheOptions) Validate() []error {
	errs := []error{}

	return errs
}

// AddFlags adds flags related to cache storage for a specific APIServer to the specified FlagSet.
func (o *CacheOptions) AddFlags(fs *pflag.FlagSet) {
}
