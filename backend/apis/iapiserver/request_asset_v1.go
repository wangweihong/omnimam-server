package iapiserver

type ArtifactRegistrationRequest struct {
	ArtifactID       string  `json:"artifact_id" binding:"required,max=64"`
	ApplicationRunID string  `json:"application_run_id" binding:"required,max=64"`
	OwnerUserID      string  `json:"owner_user_id" binding:"required,max=128"`
	OutputName       string  `json:"output_name" binding:"required,max=255"`
	MediaType        string  `json:"media_type" binding:"required,oneof=image video audio text pdf other"`
	ContentRef       string  `json:"content_ref" binding:"required"`
	Format           string  `json:"format" binding:"omitempty,max=64"`
	SizeBytes        int64   `json:"size_bytes" binding:"min=0"`
	Width            int     `json:"width" binding:"min=0"`
	Height           int     `json:"height" binding:"min=0"`
	DurationSeconds  float64 `json:"duration_seconds" binding:"min=0"`
	SHA256           string  `json:"sha256"`
}
type ArtifactRegistrationResponse struct {
	ArtifactID         string     `json:"artifact_id"`
	ApplicationRunID   string     `json:"application_run_id"`
	RegistrationResult string     `json:"registration_result"`
	Asset              *UserAsset `json:"asset"`
}
type BatchAssetItem struct {
	ID string `json:"id" binding:"required"`
}
type BatchLabelRequest struct {
	Items          []BatchAssetItem  `json:"items" binding:"required,min=1,max=100,dive"`
	LabelsToUpsert map[string]string `json:"labels_to_upsert"`
	TagsToAdd      []string          `json:"tags_to_add" binding:"max=30"`
	TagsToRemove   []string          `json:"tags_to_remove" binding:"max=30"`
}
type BatchLabelData struct {
	ID              string            `json:"id"`
	Labels          map[string]string `json:"labels"`
	Tags            []string          `json:"tags"`
	LabelSources    map[string]string `json:"label_sources"`
	TagSources      map[string]string `json:"tag_sources"`
	ResourceVersion int64             `json:"resource_version"`
}
type BatchLabelResult struct {
	ID      string          `json:"id"`
	Success bool            `json:"success"`
	Data    *BatchLabelData `json:"data,omitempty"`
	Error   map[string]any  `json:"error,omitempty"`
}
type BatchLabelResponse struct {
	Total   int                `json:"total"`
	Success int                `json:"success"`
	Fail    int                `json:"fail"`
	Results []BatchLabelResult `json:"results"`
}
