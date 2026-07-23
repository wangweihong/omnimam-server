package iapiserver

import (
	"encoding/json"
	"testing"
)

func TestPhysicalStorageFieldsRequireAdministratorProjection(t *testing.T) {
	tests := []struct {
		name      string
		value     any
		forbidden []string
		required  []string
	}{
		{
			name:      "persisted storage backend",
			value:     &StorageBackend{Root: "/srv/secret", Config: map[string]any{"access_key": "secret"}},
			forbidden: []string{"root", "config"},
		},
		{
			name:      "persisted blob",
			value:     &AssetBlob{StorageBackendID: "backend-1", ObjectKey: "blobs/secret"},
			forbidden: []string{"storage_backend_id", "object_key"},
		},
		{
			name:      "legacy asset",
			value:     &Asset{StorageBackendID: "backend-1", ObjectKey: "assets/secret"},
			forbidden: []string{"storage_backend_id", "object_key"},
		},
		{
			name:      "legacy thumbnail",
			value:     &AssetThumbnail{StorageBackendID: "backend-1", ObjectKey: "thumbnails/secret"},
			forbidden: []string{"storage_backend_id", "object_key"},
		},
		{
			name: "administrator blob detail",
			value: &AssetBlobDetail{
				StorageBackendID: "backend-1",
				ObjectKey:        "blobs/secret",
			},
			required: []string{"storage_backend_id", "object_key"},
		},
		{
			name: "administrator storage backend detail",
			value: &StorageBackendDetail{
				Root:   "/srv/secret",
				Config: map[string]any{"access_key": "secret"},
			},
			required: []string{"root", "config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatalf("marshal %s: %v", tt.name, err)
			}
			var fields map[string]any
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatalf("decode %s: %v", tt.name, err)
			}
			for _, field := range tt.forbidden {
				if _, ok := fields[field]; ok {
					t.Errorf("sensitive field %q leaked from %s: %s", field, tt.name, encoded)
				}
			}
			for _, field := range tt.required {
				if _, ok := fields[field]; !ok {
					t.Errorf("administrator field %q missing from %s: %s", field, tt.name, encoded)
				}
			}
		})
	}
}
