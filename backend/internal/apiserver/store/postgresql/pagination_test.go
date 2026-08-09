package postgresql

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wangweihong/omnimam/backend/apis/iapiserver"
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

func TestCountAndFindPageKeepsCountUnpaginated(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel: logger.Info,
		}),
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	var items []struct{ ID string }
	_, err = CountAndFindPage(
		db.Table("items").Where("status = ?", "ready").Order("created_at DESC"),
		imachinery.PagingParams{PageNum: 1, PageSize: 20},
		&items,
	)
	if err != nil {
		t.Fatalf("CountAndFindPage() error = %v", err)
	}

	statements := strings.Split(strings.TrimSpace(output.String()), "\n")
	var countSQL, findSQL string
	for _, statement := range statements {
		switch {
		case strings.Contains(statement, "SELECT count(*)"):
			countSQL = statement
		case strings.Contains(statement, "SELECT *"):
			findSQL = statement
		}
	}
	if countSQL == "" || findSQL == "" {
		t.Fatalf("expected count and find SQL, got %q", output.String())
	}
	if strings.Contains(countSQL, "LIMIT") || strings.Contains(countSQL, "OFFSET") {
		t.Fatalf("count SQL must be unpaginated: %s", countSQL)
	}
	if !strings.Contains(findSQL, "LIMIT 20 OFFSET 20") {
		t.Fatalf("find SQL has unexpected pagination: %s", findSQL)
	}
}

func TestListStudioApplicationsUsesCaseInsensitiveKeyword(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel: logger.Info,
		}),
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	appStore := newAppStudioStore(&datastore{db: db})
	_, _, err = appStore.ListStudioApplications(t.Context(), &iapiserver.StudioApplicationListRequest{
		BasicQueryParam: imachinery.BasicQueryParam{
			PagingParams: imachinery.PagingParams{PageSize: 20},
			Keyword:      "unique",
		},
		OwnerUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("ListStudioApplications() error = %v", err)
	}
	if sql := output.String(); !strings.Contains(sql, "name ILIKE '%unique%' OR description ILIKE '%unique%'") {
		t.Fatalf("keyword SQL is not case-insensitive: %s", sql)
	}
}

func TestListAgentMessagesUsesStableNewestFirstOrder(t *testing.T) {
	var output bytes.Buffer
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=test dbname=test sslmode=disable"}), &gorm.Config{
		DryRun:               true,
		DisableAutomaticPing: true,
		Logger: logger.New(log.New(&output, "", 0), logger.Config{
			LogLevel: logger.Info,
		}),
	})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	agentStore := newAgentStore(&datastore{db: db})
	_, _, err = agentStore.ListAgentMessages(t.Context(), &iapiserver.AgentMessageListRequest{
		BasicQueryParam: imachinery.BasicQueryParam{
			PagingParams: imachinery.PagingParams{PageSize: 50},
		},
		SessionID: "session-1",
	}, "user-1")
	if err != nil {
		t.Fatalf("ListAgentMessages() error = %v", err)
	}

	const expectedOrder = "ORDER BY agent_messages.created_at DESC,agent_messages.id DESC"
	if sql := output.String(); !strings.Contains(sql, expectedOrder) {
		t.Fatalf("message history SQL is not stably ordered by creation and id: %s", sql)
	}
}
