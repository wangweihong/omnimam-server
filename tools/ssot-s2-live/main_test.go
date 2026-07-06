package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIndexDocAndSearch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S2 Index\n\nmodule overview")
	writeTestFile(t, root, "features/chat/contract.yaml", "feature_id: chat\nmodule_contracts:\n  backend: {}\n")
	writeTestFile(t, root, "features/chat/review.md", "# Chat Contract Review\n\npermission matrix")
	writeTestFile(t, root, "features/chat/openapi.yaml", "openapi: 3.0.3\ninfo:\n  title: Chat Contract\n")
	writeTestFile(t, root, "features/chat/schema.sql", "create table chat_messages (\n  module_id text\n);\n")
	writeTestFile(t, root, "features/chat/handoff.s2-to-s3.md", "# S2 to S3 Handoff: chat\n")
	writeTestFile(t, root, "generated/openapi.merged.yaml", "openapi: 3.0.3\n")
	writeTestFile(t, root, "registry/permissions.yaml", "permissions: []\n")

	handler := newTestApp(root)

	indexReq := httptest.NewRequest(http.MethodGet, "/api/index", nil)
	indexRec := httptest.NewRecorder()
	handler.ServeHTTP(indexRec, indexReq)
	if indexRec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d: %s", indexRec.Code, http.StatusOK, indexRec.Body.String())
	}
	var index indexResponse
	if err := json.Unmarshal(indexRec.Body.Bytes(), &index); err != nil {
		t.Fatalf("decode index: %v", err)
	}
	if len(index.Files) != 8 {
		t.Fatalf("index files = %d, want 8", len(index.Files))
	}
	if len(index.Tree) != 4 {
		t.Fatalf("index tree nodes = %d, want 4", len(index.Tree))
	}
	wantRootOrder := []string{"features/", "generated/", "registry/", "index.md"}
	for i, want := range wantRootOrder {
		if index.Tree[i].Path != want {
			t.Fatalf("index tree root node %d = %q, want %q", i, index.Tree[i].Path, want)
		}
	}
	if index.Tree[3].Type != "file" {
		t.Fatalf("index tree should put root files after directories, got %q", index.Tree[3].Type)
	}
	featuresDir := findTreeNode(index.Tree, "features/")
	if featuresDir == nil || featuresDir.Type != "directory" {
		t.Fatalf("index tree should include features/ directory, got %#v", featuresDir)
	}
	chatDir := findTreeNode(index.Tree, "features/chat/")
	if chatDir == nil || chatDir.Type != "directory" {
		t.Fatalf("index tree should include features/chat/ directory, got %#v", chatDir)
	}
	if chatDir.Name != "chat" {
		t.Fatalf("features/chat/ name = %q, want chat", chatDir.Name)
	}
	openAPINode := findTreeNode(chatDir.Children, "features/chat/openapi.yaml")
	if openAPINode == nil {
		t.Fatal("features/chat/ children should include features/chat/openapi.yaml")
	}
	if openAPINode.Name != "openapi.yaml" {
		t.Fatalf("features/chat/openapi.yaml name = %q, want openapi.yaml", openAPINode.Name)
	}
	if openAPINode.DisplayTitle != "OpenAPI · chat" {
		t.Fatalf("openapi tree display title = %q, want OpenAPI · chat", openAPINode.DisplayTitle)
	}
	generatedDir := findTreeNode(index.Tree, "generated/")
	if generatedDir == nil || generatedDir.Type != "directory" {
		t.Fatalf("index tree should include generated/ directory, got %#v", generatedDir)
	}
	generatedNode := findTreeNode(generatedDir.Children, "generated/openapi.merged.yaml")
	if generatedNode == nil {
		t.Fatal("generated/ children should include generated/openapi.merged.yaml")
	}
	registryDir := findTreeNode(index.Tree, "registry/")
	if registryDir == nil || registryDir.Type != "directory" {
		t.Fatalf("index tree should include registry/ directory, got %#v", registryDir)
	}
	registryNode := findTreeNode(registryDir.Children, "registry/permissions.yaml")
	if registryNode == nil {
		t.Fatal("registry/ children should include registry/permissions.yaml")
	}
	openAPIFile := findFile(index.Files, "features/chat/openapi.yaml")
	if openAPIFile == nil {
		t.Fatal("index should include features/chat/openapi.yaml")
	}
	if openAPIFile.DisplayTitle != "OpenAPI · chat" {
		t.Fatalf("openapi display title = %q, want OpenAPI · chat", openAPIFile.DisplayTitle)
	}

	docReq := httptest.NewRequest(http.MethodGet, "/api/doc?path=features/chat/openapi.yaml", nil)
	docRec := httptest.NewRecorder()
	handler.ServeHTTP(docRec, docReq)
	if docRec.Code != http.StatusOK {
		t.Fatalf("doc status = %d, want %d: %s", docRec.Code, http.StatusOK, docRec.Body.String())
	}
	var doc docResponse
	if err := json.Unmarshal(docRec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode doc: %v", err)
	}
	if doc.Title != "openapi" {
		t.Fatalf("doc title = %q, want openapi", doc.Title)
	}
	if doc.Kind != "OpenAPI" {
		t.Fatalf("doc kind = %q, want OpenAPI", doc.Kind)
	}
	if doc.DisplayTitle != "OpenAPI · chat" {
		t.Fatalf("doc display title = %q, want OpenAPI · chat", doc.DisplayTitle)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/search?q=module", nil)
	searchRec := httptest.NewRecorder()
	handler.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d: %s", searchRec.Code, http.StatusOK, searchRec.Body.String())
	}
	var search searchResponse
	if err := json.Unmarshal(searchRec.Body.Bytes(), &search); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if len(search.Matches) != 3 {
		t.Fatalf("search matches = %d, want 3", len(search.Matches))
	}
}

func TestDisplayTitleForS2ContractFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		path  string
		title string
		want  string
	}{
		{name: "contract", path: "features/chat/contract.yaml", title: "contract", want: "Feature Contract · chat"},
		{name: "openapi", path: "features/chat/openapi.yaml", title: "openapi", want: "OpenAPI · chat"},
		{name: "schema", path: "features/chat/schema.sql", title: "schema", want: "DB Schema · chat"},
		{name: "review keeps heading", path: "features/chat/review.md", title: "Chat Contract Review", want: "Chat Contract Review"},
		{name: "handoff", path: "features/chat/handoff.s2-to-s3.md", title: "S2 to S3 Handoff: chat", want: "S2 to S3 Handoff · chat"},
		{name: "registry", path: "registry/permissions.yaml", title: "permissions", want: "Registry · permissions"},
		{name: "generated", path: "generated/openapi.merged.yaml", title: "openapi.merged", want: "Generated Artifact · openapi.merged"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := displayTitleFor(tt.path, tt.title); got != tt.want {
				t.Fatalf("displayTitleFor(%q, %q) = %q, want %q", tt.path, tt.title, got, tt.want)
			}
		})
	}
}

func TestHandleOpenAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := "openapi: 3.0.3\ninfo:\n  title: Chat Contract\n"
	writeTestFile(t, root, "features/chat/openapi.yaml", content)
	handler := newTestApp(root)

	req := httptest.NewRequest(http.MethodGet, "/api/openapi?path=features/chat/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != content {
		t.Fatalf("body = %q, want %q", rec.Body.String(), content)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/yaml") {
		t.Fatalf("Content-Type = %q, want application/yaml", got)
	}
}

func TestHandleGeneratedOpenAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	content := "openapi: 3.0.3\ninfo:\n  title: Merged Contract\n"
	writeTestFile(t, root, "generated/openapi.merged.yaml", content)
	handler := newTestApp(root)

	req := httptest.NewRequest(http.MethodGet, "/api/openapi?path=generated/openapi.merged.yaml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != content {
		t.Fatalf("body = %q, want %q", rec.Body.String(), content)
	}
}

func TestHandleOpenAPIOptions(t *testing.T) {
	t.Parallel()

	handler := newTestApp(t.TempDir())
	req := httptest.NewRequest(http.MethodOptions, "/api/openapi?path=features/chat/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
}

func TestOpenAPIEndpointRejectsInvalidPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "features/chat/openapi.yaml", "openapi: 3.0.3\n")
	writeTestFile(t, root, "generated/openapi.merged.yaml", "openapi: 3.0.3\n")
	writeTestFile(t, root, "features/chat/schema.sql", "create table chat_messages (id text);\n")
	handler := newTestApp(root)

	tests := []struct {
		name string
		path string
	}{
		{name: "schema file", path: "features/chat/schema.sql"},
		{name: "old openapi path", path: "openapi/chat.yaml"},
		{name: "parent traversal", path: "features/chat/../other/openapi.yaml"},
		{name: "markdown named openapi", path: "features/chat/openapi.md"},
		{name: "absolute path", path: filepath.Join(root, authoringDir, "features/chat/openapi.yaml")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/openapi?path="+tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S2 Index")
	handler := newTestApp(root)

	tests := []struct {
		name string
		path string
	}{
		{name: "parent traversal", path: "../AGENTS.md"},
		{name: "absolute path", path: filepath.Join(root, "AGENTS.md")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/doc?path="+tt.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, authoringDir), 0o755); err != nil {
		t.Fatalf("mkdir authoring: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("# Secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, authoringDir, "linked")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	handler := newTestApp(root)
	req := httptest.NewRequest(http.MethodGet, "/api/doc?path=linked/secret.md", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestEmptyAuthoringRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	handler := newTestApp(root)

	req := httptest.NewRequest(http.MethodGet, "/api/index", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"files": []`) {
		t.Fatalf("index response should contain empty files array: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tree": []`) {
		t.Fatalf("index response should contain empty tree array: %s", rec.Body.String())
	}
}

func TestIndexHTMLDefinesInlineCodePatternBeforeInline(t *testing.T) {
	t.Parallel()

	patternIndex := strings.Index(indexHTML, "const inlineCodePattern =")
	if patternIndex < 0 {
		t.Fatal("index HTML must define inlineCodePattern")
	}
	inlineIndex := strings.Index(indexHTML, "function inline(text)")
	if inlineIndex < 0 {
		t.Fatal("index HTML must define inline(text)")
	}
	renderIndex := strings.Index(indexHTML, "function renderMarkdown(source)")
	if renderIndex < 0 {
		t.Fatal("index HTML must define renderMarkdown(source)")
	}
	if patternIndex > inlineIndex || patternIndex > renderIndex {
		t.Fatal("inlineCodePattern must be defined in shared scope before markdown rendering helpers use it")
	}
}

func TestIndexHTMLRendersNonMarkdownAsCode(t *testing.T) {
	t.Parallel()

	renderDocIndex := strings.Index(indexHTML, "function renderDocument(doc)")
	if renderDocIndex < 0 {
		t.Fatal("index HTML must define renderDocument(doc)")
	}
	markdownCheckIndex := strings.Index(indexHTML, "function isMarkdownPath(path)")
	if markdownCheckIndex < 0 {
		t.Fatal("index HTML must define isMarkdownPath(path)")
	}
	codeRenderIndex := strings.Index(indexHTML, "'<pre><code>' + escapeHTML(doc.content)")
	if codeRenderIndex < 0 {
		t.Fatal("index HTML must render non-Markdown contract files as code")
	}
	if renderDocIndex > markdownCheckIndex || renderDocIndex > codeRenderIndex {
		t.Fatal("renderDocument must route contract files before markdown helpers handle content")
	}
}

func TestIndexHTMLIncludesSwaggerUITools(t *testing.T) {
	t.Parallel()

	required := []string{
		"function renderOpenAPITools(doc)",
		"docker.swagger.io/swaggerapi/swagger-ui",
		"SWAGGER_JSON_URL=",
		"/api/openapi?path=",
		"Open Swagger UI",
	}
	for _, snippet := range required {
		if !strings.Contains(indexHTML, snippet) {
			t.Fatalf("index HTML must include %s", snippet)
		}
	}
	if strings.Contains(indexHTML, "SWAGGER_JSON_URL=http://host.docker.internal:9972/api/doc") {
		t.Fatal("Swagger command must not use JSON document endpoint")
	}
}

func TestIndexHTMLUsesDisplayTitleForDocumentTitle(t *testing.T) {
	t.Parallel()

	required := []string{
		"function displayTitle(file)",
		"titleEl.textContent = displayTitle(doc);",
		"file.display_title || file.title || file.path",
	}
	for _, snippet := range required {
		if !strings.Contains(indexHTML, snippet) {
			t.Fatalf("index HTML must include %s", snippet)
		}
	}
}

func TestIndexHTMLIncludesDirectoryTreeNavigation(t *testing.T) {
	t.Parallel()

	required := []string{
		"let docTree = [];",
		"const expandedDirs = new Set();",
		"function renderTreeNode(node, depth, forceExpanded)",
		"function treeLabel(node)",
		"return node.name || node.path;",
		"escapeHTML(treeLabel(node))",
		`node.type === "directory"`,
		"function filterTree(nodes, q)",
		"button.addEventListener(\"click\", () => {",
	}
	for _, snippet := range required {
		if !strings.Contains(indexHTML, snippet) {
			t.Fatalf("index HTML must include %s", snippet)
		}
	}
	if strings.Contains(indexHTML, "displayTitle(node)") {
		t.Fatal("directory tree labels must use node.name instead of displayTitle(node)")
	}
	if strings.Contains(indexHTML, `treeLabel(node)) + '<span class="kind"`) {
		t.Fatal("file tree rows must display only the file name")
	}
}

func TestKindForS2ReviewFiles(t *testing.T) {
	t.Parallel()

	if got := kindFor("features/chat/review.md"); got != "Contract Review" {
		t.Fatalf("kindFor(features/chat/review.md) = %q, want Contract Review", got)
	}
}

func TestIndexHTMLPrefersReviewWhenIndexMissing(t *testing.T) {
	t.Parallel()

	preferredIndex := strings.Index(indexHTML, "function preferredDocument(files)")
	if preferredIndex < 0 {
		t.Fatal("index HTML must define preferredDocument(files)")
	}
	reviewIndex := strings.Index(indexHTML, `f.path.startsWith("features/") && f.path.endsWith("/review.md")`)
	if reviewIndex < 0 {
		t.Fatal("index HTML must prefer feature review documents")
	}
	if preferredIndex > reviewIndex {
		t.Fatal("preferredDocument must include review document preference")
	}
}

func TestIndexHTMLIncludesMarkdownTableParser(t *testing.T) {
	t.Parallel()

	required := []string{
		"function isTableStart(lines, index)",
		"function isTableSeparator(line)",
		"function parseTableRow(line)",
		"function renderTable(lines, startIndex)",
		"function tableAlignment(separatorCell)",
		"function tableCell(tag, value, alignment)",
	}
	for _, snippet := range required {
		if !strings.Contains(indexHTML, snippet) {
			t.Fatalf("index HTML must include %s", snippet)
		}
	}
}

func TestIndexHTMLParsesTablesBeforeParagraphFallback(t *testing.T) {
	t.Parallel()

	tableStartIndex := strings.Index(indexHTML, "if (isTableStart(lines, i))")
	if tableStartIndex < 0 {
		t.Fatal("renderMarkdown must detect table starts")
	}
	paragraphFallbackIndex := strings.LastIndex(indexHTML, `paragraph += (paragraph ? " " : "") + line.trim();`)
	if paragraphFallbackIndex < 0 {
		t.Fatal("renderMarkdown must include paragraph fallback")
	}
	if tableStartIndex > paragraphFallbackIndex {
		t.Fatal("table detection must run before paragraph fallback")
	}
}

func newTestApp(root string) http.Handler {
	mux := http.NewServeMux()
	app := &app{
		root:       root,
		authoring:  filepath.Join(root, authoringDir),
		now:        func() time.Time { return time.Unix(0, 0).UTC() },
		maxDocSize: 2 << 20,
	}
	app.routes(mux)
	return mux
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()

	path := filepath.Join(root, authoringDir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func findFile(files []fileInfo, path string) *fileInfo {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

func findTreeNode(nodes []treeNode, path string) *treeNode {
	for i := range nodes {
		if nodes[i].Path == path {
			return &nodes[i]
		}
		if found := findTreeNode(nodes[i].Children, path); found != nil {
			return found
		}
	}
	return nil
}
