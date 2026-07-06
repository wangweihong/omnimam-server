package postgresql

import (
	"context"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"
	"gorm.io/gorm"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
)

type ott struct {
	ds *datastore
}

func newOneTimeToken(ds *datastore) *ott {
	return &ott{ds: ds}
}

func (s *ott) GetByHash(ctx context.Context, hash string) (*iapiserver.OneTimeToken, error) {
	var meta iapiserver.OneTimeToken

	err := s.ds.db.Model(&iapiserver.OneTimeToken{}).
		Where("payload_hash = ?", hash).
		First(&meta).Error
	return &meta, err
}

func (s *ott) Add(ctx context.Context, data *iapiserver.OneTimeToken) (*iapiserver.OneTimeToken, error) {
	err := s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(data).Error; err != nil {
			return errors.WithStack(err)
		}

		return nil
	})

	return data, err
}

func (s *ott) Delete(ctx context.Context, id string) error {
	return s.ds.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Where("id = ?", id).Delete(&iapiserver.OneTimeToken{}).Error
	})
}

func (s *ott) CleanupExpiredTokens(ctx context.Context) error {
	return s.ds.db.WithContext(ctx).
		Where("expires_at < ? OR used = true", time.Now()).
		Delete(&iapiserver.OneTimeToken{}).Error
}
