package iapiserver

import (
	"github.com/wangweihong/omnimam/backend/apis/imachinery"
)

const (
	AssetKindImage    = "image"
	AssetKindVideo    = "video"
	AssetKindAudio    = "audio"
	AssetKindWorkflow = "workflow"

	CategoryTypeImage    = "image"
	CategoryTypeWorkflow = "workflow"
)

type AssetLibrary struct {
	imachinery.ObjectMeta
}

func (AssetLibrary) TableName() string {
	return "asset_libraries"
}

type AssetCategory struct {
	imachinery.ObjectMeta
	LibraryID string `json:"library_id" gorm:"column:library_id;type:varchar(64);not null;index"`
	Type      string `json:"type"       gorm:"column:type;type:varchar(32);not null;default:image"`
	Dir       string `json:"dir"        gorm:"column:dir;type:varchar(128)"`
	SortOrder int    `json:"sort_order" gorm:"column:sort_order;default:0"`
}

func (AssetCategory) TableName() string {
	return "asset_categories"
}

type AssetItemClassification struct {
	Summary    string                        `json:"summary"`
	Categories map[string][]string           `json:"categories"`
	Tags       []string                      `json:"tags"`
	Flat       []AssetClassificationFlatItem `json:"flat"`
	Model      string                        `json:"model"`
	Provider   string                        `json:"provider"`
	UpdatedAt  int64                         `json:"updated_at"`
}

type AssetClassificationFlatItem struct {
	Dimension string `json:"dimension"`
	Label     string `json:"label"`
	Tag       string `json:"tag"`
}

type AssetItemRegistration struct {
	ProviderID   string `json:"provider_id"`
	ProjectName  string `json:"project_name"`
	TaskID       string `json:"task_id"`
	Status       string `json:"status"`
	Detail       string `json:"detail"`
	AssetURI     string `json:"asset_uri"`
	AssetID      string `json:"asset_id"`
	RegisteredAt int64  `json:"registered_at"`
}

type AssetItem struct {
	imachinery.ObjectMeta
	LibraryID  string `json:"library_id"  gorm:"column:library_id;type:varchar(64);not null;index"`
	CategoryID string `json:"category_id" gorm:"column:category_id;type:varchar(64);not null;index"`
	URL        string `json:"url"         gorm:"column:url;type:varchar(512);not null"`
	Kind       string `json:"kind"        gorm:"column:kind;type:varchar(32);not null;default:image"`
	Size       int64  `json:"size"        gorm:"column:size"`
	Format     string `json:"format"      gorm:"column:format;type:varchar(16)"`
	SortOrder  int    `json:"sort_order"  gorm:"column:sort_order;default:0"`
}

func (AssetItem) TableName() string {
	return "asset_items"
}

const (
	PromptLibraryType = "prompt"

	CanvasKindClassic = "classic"
	CanvasKindSmart   = "smart"
)

type PromptLibrary struct {
	imachinery.ObjectMeta
	System   bool `json:"system"   gorm:"column:system;type:boolean;not null;default:false"`
	Active   bool `json:"active"   gorm:"column:active;type:boolean;not null;default:false"`
	Readonly bool `json:"readonly" gorm:"column:readonly;type:boolean;not null;default:false"`
}

func (PromptLibrary) TableName() string {
	return "prompt_libraries"
}

type PromptCategory struct {
	imachinery.ObjectMeta
	LibraryID string `json:"library_id" gorm:"column:library_id;type:varchar(64);not null;index"`
}

func (PromptCategory) TableName() string {
	return "prompt_categories"
}

type PromptItem struct {
	imachinery.ObjectMeta
	LibraryID  string            `json:"library_id"       gorm:"column:library_id;type:varchar(64);not null;index"`
	CategoryID string            `json:"category_id"      gorm:"column:category_id;type:varchar(64);not null;index"`
	Positive   string            `json:"positive"         gorm:"column:positive;type:text;not null"`
	Negative   string            `json:"negative"         gorm:"column:negative;type:text"`
	Scene      string            `json:"scene"            gorm:"column:scene;type:varchar(500)"`
	Params     imachinery.Extend `json:"params,omitempty" gorm:"-"`
}

func (PromptItem) TableName() string {
	return "prompt_items"
}

type Project struct {
	imachinery.ObjectMeta
	SortOrder int `json:"sort_order" gorm:"column:sort_order;default:0"`
}

func (Project) TableName() string {
	return "projects"
}

type Canvas struct {
	imachinery.ObjectMeta
	Title       string  `json:"title"                 gorm:"column:title;type:varchar(80);not null"`
	Icon        string  `json:"icon"                  gorm:"column:icon;type:varchar(32)"`
	Kind        string  `json:"kind"                  gorm:"column:kind;type:varchar(16);not null;default:classic"`
	Owner       string  `json:"owner"                 gorm:"column:owner;type:varchar(40)"`
	Color       string  `json:"color"                 gorm:"column:color;type:varchar(16)"`
	Pinned      bool    `json:"pinned"                gorm:"column:pinned;type:boolean;not null;default:false"`
	ProjectID   string  `json:"project_id"            gorm:"column:project_id;type:varchar(64);not null;index"`
	DeletedAt   int64   `json:"deleted_at"            gorm:"column:deleted_at"`
	BoardX      float64 `json:"board_x"               gorm:"column:board_x"`
	BoardY      float64 `json:"board_y"               gorm:"column:board_y"`
	Nodes       any     `json:"nodes,omitempty"       gorm:"-"`
	Connections any     `json:"connections,omitempty" gorm:"-"`
	Viewport    any     `json:"viewport,omitempty"    gorm:"-"`
	Logs        any     `json:"logs,omitempty"        gorm:"-"`
	Settings    any     `json:"settings,omitempty"    gorm:"-"`
}

func (Canvas) TableName() string {
	return "canvases"
}
