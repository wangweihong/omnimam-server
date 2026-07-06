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
	writeTestFile(t, root, "index.md", "# S1 Index\n\nmodule overview")
	writeTestFile(t, root, "feature-map.md", "# Feature Map\n\nmodule search target")
	writeTestFile(t, root, "features/chat.md", "# Chat Feature\n\ncore use case")
	writeTestFile(t, root, "prototype-evidence/chat.md", "# Chat Evidence\n\naccepted prototype facts")

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
	if len(index.Files) != 4 {
		t.Fatalf("index files = %d, want 4", len(index.Files))
	}
	if len(index.Tree) != 4 {
		t.Fatalf("index tree nodes = %d, want 4", len(index.Tree))
	}
	if index.Tree[0].Path != "features/" || index.Tree[1].Path != "prototype-evidence/" {
		t.Fatalf("index tree should sort directories first, got %q then %q", index.Tree[0].Path, index.Tree[1].Path)
	}
	if index.Tree[2].Type != "file" || index.Tree[3].Type != "file" {
		t.Fatalf("index tree should put files after directories, got %q and %q", index.Tree[2].Type, index.Tree[3].Type)
	}
	indexFile := findTreeNode(index.Tree, "index.md")
	if indexFile == nil || indexFile.Type != "file" {
		t.Fatalf("index tree should include root file index.md, got %#v", indexFile)
	}
	featuresDir := findTreeNode(index.Tree, "features/")
	if featuresDir == nil {
		t.Fatal("index tree should include features/ directory")
	}
	if featuresDir.Type != "directory" {
		t.Fatalf("features/ type = %q, want directory", featuresDir.Type)
	}
	chatFile := findTreeNode(featuresDir.Children, "features/chat.md")
	if chatFile == nil {
		t.Fatal("features/ children should include features/chat.md")
	}
	if chatFile.Type != "file" || chatFile.Kind != "Product Slice" {
		t.Fatalf("features/chat.md node = %#v, want Product Slice file", chatFile)
	}
	if chatFile.Name != "chat.md" {
		t.Fatalf("features/chat.md name = %q, want chat.md", chatFile.Name)
	}
	evidenceFile := findFile(index.Files, "prototype-evidence/chat.md")
	if evidenceFile == nil {
		t.Fatal("index should include prototype-evidence/chat.md")
	}
	if evidenceFile.Kind != "Prototype Evidence" {
		t.Fatalf("evidence kind = %q, want Prototype Evidence", evidenceFile.Kind)
	}

	docReq := httptest.NewRequest(http.MethodGet, "/api/doc?path=feature-map.md", nil)
	docRec := httptest.NewRecorder()
	handler.ServeHTTP(docRec, docReq)
	if docRec.Code != http.StatusOK {
		t.Fatalf("doc status = %d, want %d: %s", docRec.Code, http.StatusOK, docRec.Body.String())
	}
	var doc docResponse
	if err := json.Unmarshal(docRec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("decode doc: %v", err)
	}
	if doc.Title != "Feature Map" {
		t.Fatalf("doc title = %q, want Feature Map", doc.Title)
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
	if len(search.Matches) != 2 {
		t.Fatalf("search matches = %d, want 2", len(search.Matches))
	}
}

func TestPutDocUpdatesExistingDocument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S1 Index\n\nbefore")
	handler := newTestApp(root)

	req := httptest.NewRequest(http.MethodPut, "/api/doc?path=index.md", strings.NewReader(`{"content":"# Updated Index\n\nafter"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var saved docResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved doc: %v", err)
	}
	if saved.Title != "Updated Index" {
		t.Fatalf("saved title = %q, want Updated Index", saved.Title)
	}
	if saved.Content != "# Updated Index\n\nafter" {
		t.Fatalf("saved content = %q", saved.Content)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/doc?path=index.md", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d: %s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var loaded docResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &loaded); err != nil {
		t.Fatalf("decode loaded doc: %v", err)
	}
	if loaded.Content != saved.Content {
		t.Fatalf("loaded content = %q, want %q", loaded.Content, saved.Content)
	}
}

func TestPutDocRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S1 Index")
	writeTestFile(t, root, "notes.bin", "binary")
	handler := newTestApp(root)

	tests := []struct {
		name string
		path string
		body string
		want int
	}{
		{name: "parent traversal", path: "../AGENTS.md", body: `{"content":"x"}`, want: http.StatusBadRequest},
		{name: "absolute path", path: filepath.Join(root, "index.md"), body: `{"content":"x"}`, want: http.StatusBadRequest},
		{name: "missing file", path: "missing.md", body: `{"content":"x"}`, want: http.StatusNotFound},
		{name: "unsupported file", path: "notes.bin", body: `{"content":"x"}`, want: http.StatusBadRequest},
		{name: "invalid json", path: "index.md", body: `{"content":`, want: http.StatusBadRequest},
		{name: "unknown field", path: "index.md", body: `{"content":"x","extra":true}`, want: http.StatusBadRequest},
		{name: "missing content", path: "index.md", body: `{}`, want: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/doc?path="+tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body.String())
			}
		})
	}
}

func TestPutDocRejectsTooLargeContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S1 Index")
	handler := newTestAppWithMaxDocSize(root, 8)

	req := httptest.NewRequest(http.MethodPut, "/api/doc?path=index.md", strings.NewReader(`{"content":"too large"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
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

func TestRejectsPathTraversal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestFile(t, root, "index.md", "# S1 Index")
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
	tests := []struct {
		name   string
		method string
		body   string
	}{
		{name: "get", method: http.MethodGet},
		{name: "put", method: http.MethodPut, body: `{"content":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/doc?path=linked/secret.md", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
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
		t.Fatal("index HTML must render non-Markdown authoring files as code")
	}
	if renderDocIndex > markdownCheckIndex || renderDocIndex > codeRenderIndex {
		t.Fatal("renderDocument must route non-Markdown files before markdown helpers handle content")
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

func TestIndexHTMLIncludesDocumentEditing(t *testing.T) {
	t.Parallel()

	required := []string{
		`id="previewBtn"`,
		`id="editBtn"`,
		`id="saveBtn"`,
		`id="editor"`,
		"function saveDoc()",
		"function isDirty()",
		`method: "PUT"`,
		`JSON.stringify({ content: editorEl.value })`,
		`Discard unsaved changes?`,
		`beforeunload`,
	}
	for _, snippet := range required {
		if !strings.Contains(indexHTML, snippet) {
			t.Fatalf("index HTML must include %s", snippet)
		}
	}
}

func TestS1LiveServerDocsAdvertiseEditableMode(t *testing.T) {
	t.Parallel()

	files := []string{
		"../../skills/ssot-product-workflow/SKILL.md",
		"../../skills/ssot-product-workflow/references/ssot-core.md",
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(data)
		if strings.Contains(content, "S1 preview server is readonly") ||
			strings.Contains(content, "preview server is readonly and must not edit Markdown") {
			t.Fatalf("%s must not describe S1 live server as readonly", file)
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
	return newTestAppWithMaxDocSize(root, 2<<20)
}

func newTestAppWithMaxDocSize(root string, maxDocSize int64) http.Handler {
	mux := http.NewServeMux()
	app := &app{
		root:       root,
		authoring:  filepath.Join(root, authoringDir),
		now:        func() time.Time { return time.Unix(0, 0).UTC() },
		maxDocSize: maxDocSize,
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
