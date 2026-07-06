package database

import (
	"fmt"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/internal/apiserver/store"
	"github.com/wangweihong/omnimam/backend/internal/apiserver/store/postgresql"
	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions"
)

var (
	factory store.Factory
)

// GetDatabaseFactoryOr create database factory with the given config.
func GetDatabaseFactoryOr(opts *genericoptions.DatabaseOptions) (store.Factory, error) {
	if opts == nil && factory == nil {
		return nil, errors.Errorf("failed to get store factory")
	}

	var err error
	switch opts.Type {
	case "postgresql":
		factory, err = postgresql.GetPostgresSQLFactoryOr(&opts.PostgresSQL)
	default:
		err = fmt.Errorf("unsupport database type:%s", opts.Type)
	}

	if factory == nil || err != nil {
		return nil, fmt.Errorf(
			"failed to get  store factory, factory: %+v, error: %w",
			factory,
			err,
		)
	}

	return factory, nil
}
