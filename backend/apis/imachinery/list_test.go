package imachinery

import (
	"context"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestBasicQueryParamToQueryPagination(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	tests := []struct {
		name       string
		params     BasicQueryParam
		wantLimit  int
		wantOffset int
	}{
		{name: "default first page", params: BasicQueryParam{}, wantLimit: 500, wantOffset: 0},
		{name: "explicit first page", params: BasicQueryParam{PagingParams: PagingParams{PageSize: 20}}, wantLimit: 20, wantOffset: 0},
		{name: "explicit second page", params: BasicQueryParam{PagingParams: PagingParams{PageNum: 1, PageSize: 20}}, wantLimit: 20, wantOffset: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := tt.params.ToQuery(context.Background(), db.Table("items"), nil)
			limitClause, ok := query.Statement.Clauses["LIMIT"]
			if !ok {
				t.Fatal("LIMIT clause is missing")
			}
			limit, ok := limitClause.Expression.(clause.Limit)
			if !ok || limit.Limit == nil {
				t.Fatalf("LIMIT clause = %#v", limitClause.Expression)
			}
			if *limit.Limit != tt.wantLimit || limit.Offset != tt.wantOffset {
				t.Fatalf("pagination = LIMIT %d OFFSET %d, want LIMIT %d OFFSET %d", *limit.Limit, limit.Offset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}
