package iapiserver

import "testing"

func TestStorageBackendListRequestUsesReleasedPagination(t *testing.T) {
	req := &StorageBackendListRequest{}
	req.SetDefaults()
	if req.PageSize != 50 {
		t.Fatalf("default page_size = %d, want 50", req.PageSize)
	}
	if err := req.Validate(); err != nil {
		t.Fatalf("default request validation: %v", err)
	}
	req.PageSize = 201
	if err := req.Validate(); err == nil {
		t.Fatal("page_size above 200 must be rejected")
	}
}

func TestStorageBackendUpdateRequestRequiresOneField(t *testing.T) {
	if err := (&StorageBackendUpdateRequest{}).Validate(); err == nil {
		t.Fatal("empty update must be rejected")
	}
	enabled := false
	if err := (&StorageBackendUpdateRequest{Enabled: &enabled}).Validate(); err != nil {
		t.Fatalf("enabled-only update must be accepted: %v", err)
	}
}
