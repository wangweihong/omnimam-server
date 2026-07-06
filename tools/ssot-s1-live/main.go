package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

const authoringDir = ".agent_workspace/stages/s1_product_slice"

type fileInfo struct {
	Path     string    `json:"path"`
	Title    string    `json:"title"`
	Kind     string    `json:"kind"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type treeNode struct {
	Type         string     `json:"type"`
	Name         string     `json:"name"`
	Path         string     `json:"path"`
	Title        string     `json:"title"`
	DisplayTitle string     `json:"display_title"`
	Kind         string     `json:"kind"`
	Size         int64      `json:"size"`
	Modified     time.Time  `json:"modified"`
	Children     []treeNode `json:"children"`
}

type indexResponse struct {
	Root        string     `json:"root"`
	Authoring   string     `json:"authoring"`
	GeneratedAt time.Time  `json:"generated_at"`
	Files       []fileInfo `json:"files"`
	Tree        []treeNode `json:"tree"`
}

type docResponse struct {
	Path     string    `json:"path"`
	Title    string    `json:"title"`
	Kind     string    `json:"kind"`
	Content  string    `json:"content"`
	Modified time.Time `json:"modified"`
}

type saveDocRequest struct {
	Content *string `json:"content"`
}

type searchMatch struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Line    int    `json:"line"`
	Snippet string `json:"snippet"`
}

type searchResponse struct {
	Query   string        `json:"query"`
	Matches []searchMatch `json:"matches"`
}

type app struct {
	root       string
	authoring  string
	logger     *slog.Logger
	now        func() time.Time
	maxDocSize int64
}

func main() {
	var root string
	var addr string

	flag.StringVar(&root, "root", ".", "repository or S1 worktree root")
	flag.StringVar(&addr, "addr", "127.0.0.1:9971", "address to listen on")
	flag.Parse()

	if err := run(root, addr); err != nil {
		fmt.Fprintf(os.Stderr, "ssot-s1-live: %v\n", err)
		os.Exit(1)
	}
}

func run(root, addr string) error {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	authoring := filepath.Join(resolvedRoot, authoringDir)

	app := &app{
		root:       resolvedRoot,
		authoring:  authoring,
		logger:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
		now:        time.Now,
		maxDocSize: 2 << 20,
	}

	mux := http.NewServeMux()
	app.routes(mux)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		app.logger.Info("starting S1 live server", "url", "http://"+addr, "root", resolvedRoot, "authoring", authoring)
		errc <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if err := server.Close(); err != nil {
			return fmt.Errorf("close server: %w", err)
		}
		return nil
	case err := <-errc:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}
}

func (a *app) routes(mux *http.ServeMux) {
	mux.HandleFunc("/", a.handleHome)
	mux.HandleFunc("/api/index", a.handleIndex)
	mux.HandleFunc("/api/doc", a.handleDoc)
	mux.HandleFunc("/api/search", a.handleSearch)
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	files, err := a.indexFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, indexResponse{
		Root:        a.root,
		Authoring:   a.authoring,
		GeneratedAt: a.now().UTC(),
		Files:       files,
		Tree:        buildTree(files),
	})
}

func (a *app) handleDoc(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleGetDoc(w, r)
	case http.MethodPut:
		a.handlePutDoc(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *app) handleGetDoc(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveDocPath(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	if info.Size() > a.maxDocSize {
		http.Error(w, "document too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, "read document", http.StatusInternalServerError)
		return
	}
	writeJSON(w, docResponse{
		Path:     filepath.ToSlash(rel),
		Title:    titleFor(rel, string(data)),
		Kind:     kindFor(rel),
		Content:  string(data),
		Modified: info.ModTime().UTC(),
	})
}

func (a *app) handlePutDoc(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("path")
	full, err := a.resolveDocPath(rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !supportedDoc(rel) {
		http.Error(w, "unsupported document type", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(full)
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "document path is a directory", http.StatusBadRequest)
		return
	}

	var req saveDocRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, a.maxDocSize+4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, "invalid save request", http.StatusBadRequest)
		return
	}
	if req.Content == nil {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}
	if int64(len([]byte(*req.Content))) > a.maxDocSize {
		http.Error(w, "document too large", http.StatusRequestEntityTooLarge)
		return
	}
	if err := os.WriteFile(full, []byte(*req.Content), info.Mode().Perm()); err != nil {
		http.Error(w, "write document", http.StatusInternalServerError)
		return
	}

	info, err = os.Stat(full)
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	writeJSON(w, docResponse{
		Path:     filepath.ToSlash(rel),
		Title:    titleFor(rel, *req.Content),
		Kind:     kindFor(rel),
		Content:  *req.Content,
		Modified: info.ModTime().UTC(),
	})
}

func (a *app) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, searchResponse{Query: query, Matches: []searchMatch{}})
		return
	}
	files, err := a.indexFiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	needle := strings.ToLower(query)
	var matches []searchMatch
	for _, file := range files {
		full, err := a.resolveDocPath(file.Path)
		if err != nil {
			continue
		}
		info, err := os.Stat(full)
		if err != nil || info.Size() > a.maxDocSize {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), needle) {
				matches = append(matches, searchMatch{
					Path:    file.Path,
					Title:   file.Title,
					Line:    i + 1,
					Snippet: strings.TrimSpace(line),
				})
				if len(matches) >= 200 {
					writeJSON(w, searchResponse{Query: query, Matches: matches})
					return
				}
			}
		}
	}
	writeJSON(w, searchResponse{Query: query, Matches: matches})
}

func (a *app) indexFiles() ([]fileInfo, error) {
	if _, err := os.Stat(a.authoring); err != nil {
		if os.IsNotExist(err) {
			return []fileInfo{}, nil
		}
		return nil, fmt.Errorf("stat authoring root: %w", err)
	}

	var files []fileInfo
	err := filepath.WalkDir(a.authoring, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(a.authoring, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}
		if !supportedDoc(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("file info: %w", err)
		}
		title := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
		if info.Size() <= a.maxDocSize {
			if data, err := os.ReadFile(path); err == nil {
				title = titleFor(rel, string(data))
			}
		}
		files = append(files, fileInfo{
			Path:     filepath.ToSlash(rel),
			Title:    title,
			Kind:     kindFor(rel),
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk authoring docs: %w", err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

type treeBuilderNode struct {
	node     treeNode
	children map[string]*treeBuilderNode
}

func buildTree(files []fileInfo) []treeNode {
	root := &treeBuilderNode{children: map[string]*treeBuilderNode{}}
	for _, file := range files {
		addFileToTree(root, file)
	}
	return sortedTreeChildren(root)
}

func addFileToTree(root *treeBuilderNode, file fileInfo) {
	parts := strings.Split(filepath.ToSlash(file.Path), "/")
	current := root
	var currentPath string
	for _, part := range parts[:len(parts)-1] {
		currentPath += part + "/"
		child, ok := current.children[part]
		if !ok {
			child = &treeBuilderNode{
				node: treeNode{
					Type:         "directory",
					Name:         part,
					Path:         currentPath,
					Title:        part,
					DisplayTitle: part,
					Kind:         "Directory",
					Children:     []treeNode{},
				},
				children: map[string]*treeBuilderNode{},
			}
			current.children[part] = child
		}
		if file.Modified.After(child.node.Modified) {
			child.node.Modified = file.Modified
		}
		current = child
	}

	name := parts[len(parts)-1]
	current.children[name] = &treeBuilderNode{
		node: treeNode{
			Type:         "file",
			Name:         name,
			Path:         file.Path,
			Title:        file.Title,
			DisplayTitle: file.Title,
			Kind:         file.Kind,
			Size:         file.Size,
			Modified:     file.Modified,
			Children:     []treeNode{},
		},
	}
}

func sortedTreeChildren(parent *treeBuilderNode) []treeNode {
	nodes := make([]*treeBuilderNode, 0, len(parent.children))
	for _, child := range parent.children {
		nodes = append(nodes, child)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].node.Type != nodes[j].node.Type {
			return nodes[i].node.Type == "directory"
		}
		return nodes[i].node.Path < nodes[j].node.Path
	})

	result := make([]treeNode, 0, len(nodes))
	for _, child := range nodes {
		child.node.Children = sortedTreeChildren(child)
		result = append(result, child.node)
	}
	return result
}

func (a *app) resolveDocPath(rel string) (string, error) {
	rel = filepath.Clean(filepath.FromSlash(strings.TrimSpace(rel)))
	if rel == "." || rel == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(rel) || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("invalid document path")
	}
	full := filepath.Join(a.authoring, rel)
	root, err := filepath.Abs(a.authoring)
	if err != nil {
		return "", fmt.Errorf("resolve authoring root: %w", err)
	}
	target, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolve document path: %w", err)
	}
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("document path escapes authoring root")
	}
	evalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve authoring root symlinks: %w", err)
	}
	if evalTarget, err := filepath.EvalSymlinks(target); err == nil {
		if evalTarget != evalRoot && !strings.HasPrefix(evalTarget, evalRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("document path escapes authoring root")
		}
	}
	return target, nil
}

func supportedDoc(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".yaml", ".yml", ".sql", ".txt":
		return true
	default:
		return false
	}
}

func kindFor(path string) string {
	path = filepath.ToSlash(path)
	switch {
	case strings.HasPrefix(path, "features/"):
		return "Product Slice"
	case strings.HasPrefix(path, "prototype-evidence/"):
		return "Prototype Evidence"
	case path == "feature-map.md":
		return "Feature Map"
	default:
		return "Document"
	}
}

func titleFor(path, content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimLeft(line, "#"))
		}
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}

const indexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>SSOT S1 Live</title>
  <style>
    :root { color-scheme: light; --line:#d7dde8; --ink:#172033; --muted:#667085; --panel:#f7f9fc; --accent:#2563eb; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: var(--ink); background: white; }
    .layout { display: grid; grid-template-columns: minmax(260px, 340px) 1fr; min-height: 100vh; }
    aside { border-right: 1px solid var(--line); background: var(--panel); padding: 18px; overflow: auto; }
    main { padding: 22px 28px; overflow: auto; }
    h1 { margin: 0 0 4px; font-size: 20px; }
    h2 { margin-top: 28px; border-bottom: 1px solid var(--line); padding-bottom: 8px; }
    .meta { color: var(--muted); font-size: 12px; margin-bottom: 16px; }
    .search { width: 100%; padding: 10px 12px; border: 1px solid var(--line); border-radius: 8px; margin: 12px 0; }
    .doc-list { display: flex; flex-direction: column; gap: 4px; }
    .tree-row { --depth: 0; width: 100%; border: 1px solid transparent; background: transparent; text-align: left; padding: 8px 8px 8px calc(8px + var(--depth) * 16px); border-radius: 8px; cursor: pointer; color: var(--ink); }
    .tree-row:hover, .tree-row.active { border-color: #b9c6dc; background: white; }
    .dir-button { font-weight: 650; display: flex; align-items: center; gap: 6px; }
    .tree-caret { width: 12px; display: inline-block; color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .file-button { display: block; }
    .kind { display: block; color: var(--muted); font-size: 11px; margin-top: 2px; }
    .toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 16px; }
    .edit-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
    .tool-button { border: 1px solid var(--line); background: white; color: var(--ink); border-radius: 8px; padding: 8px 10px; cursor: pointer; }
    .tool-button:hover, .tool-button.active { border-color: #b9c6dc; background: #eef4ff; }
    .tool-button:disabled { cursor: not-allowed; color: var(--muted); background: #f2f4f7; }
    .save-status { min-width: 74px; color: var(--muted); font-size: 12px; }
    .path { color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12px; }
    pre { padding: 12px; background: #101828; color: #f9fafb; overflow: auto; border-radius: 8px; }
    code { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
    .markdown { max-width: 1040px; line-height: 1.58; }
    .editor { width: 100%; min-height: calc(100vh - 138px); resize: vertical; border: 1px solid var(--line); border-radius: 8px; padding: 12px; font: 13px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; color: var(--ink); background: #fcfdff; }
    .hidden { display: none; }
    .markdown table { border-collapse: collapse; width: 100%; }
    .markdown th, .markdown td { border: 1px solid var(--line); padding: 8px; text-align: left; }
    .markdown blockquote { border-left: 4px solid var(--line); margin-left: 0; padding-left: 12px; color: var(--muted); }
    .mermaid { padding: 16px; border: 1px solid var(--line); border-radius: 8px; background: white; margin: 14px 0; overflow: auto; }
    .empty { padding: 28px; border: 1px dashed var(--line); border-radius: 8px; color: var(--muted); }
    mark { background: #fef3c7; }
    @media (max-width: 780px) { .layout { grid-template-columns: 1fr; } aside { border-right: 0; border-bottom: 1px solid var(--line); max-height: 42vh; } }
  </style>
</head>
<body>
  <div class="layout">
    <aside>
      <h1>SSOT S1 Live</h1>
      <div class="meta" id="rootMeta">Loading...</div>
      <input class="search" id="search" placeholder="Search S1 docs" />
      <div class="doc-list" id="docs"></div>
    </aside>
    <main>
      <div class="toolbar">
        <div>
          <h1 id="title">Select a document</h1>
          <div class="path" id="path"></div>
        </div>
        <div class="edit-actions">
          <button class="tool-button active" id="previewBtn" type="button">Preview</button>
          <button class="tool-button" id="editBtn" type="button">Edit</button>
          <button class="tool-button" id="saveBtn" type="button" disabled>Save</button>
          <span class="save-status" id="saveStatus"></span>
        </div>
      </div>
      <article class="markdown" id="content">
        <div class="empty">S1 authoring documents will appear here.</div>
      </article>
      <textarea class="editor hidden" id="editor" spellcheck="false"></textarea>
    </main>
  </div>
  <script type="module">
    import mermaid from "https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs";
    mermaid.initialize({ startOnLoad: false, securityLevel: "strict" });

    const docsEl = document.getElementById("docs");
    const searchEl = document.getElementById("search");
    const titleEl = document.getElementById("title");
    const pathEl = document.getElementById("path");
    const contentEl = document.getElementById("content");
    const editorEl = document.getElementById("editor");
    const rootMetaEl = document.getElementById("rootMeta");
    const previewBtn = document.getElementById("previewBtn");
    const editBtn = document.getElementById("editBtn");
    const saveBtn = document.getElementById("saveBtn");
    const saveStatusEl = document.getElementById("saveStatus");
    let allFiles = [];
    let docTree = [];
    let activePath = "";
    let currentDoc = null;
    let originalContent = "";
    let editMode = false;
    const expandedDirs = new Set();
    const tick = String.fromCharCode(96);
    const fenceMarker = tick + tick + tick;
    const inlineCodePattern = new RegExp(tick + "([^" + tick + "]+)" + tick, "g");

    const escapeHTML = (value) => String(value ?? "")
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;");

    async function loadIndex() {
      const res = await fetch("/api/index");
      const index = await res.json();
      allFiles = index.files || [];
      docTree = index.tree || [];
      rootMetaEl.textContent = String(allFiles.length) + " docs · " + index.authoring;
      renderList();
      const first = allFiles.find((f) => f.path === "index.md") || allFiles[0];
      if (first) await loadDoc(first.path);
    }

    function renderList() {
      docsEl.innerHTML = "";
      const q = searchEl.value.trim().toLowerCase();
      const nodes = q ? filterTree(docTree, q) : docTree;
      if (!nodes.length) {
        docsEl.innerHTML = '<div class="empty">No S1 docs found.</div>';
        return;
      }
      for (const node of nodes) {
        renderTreeNode(node, 0, Boolean(q));
      }
    }

    function renderTreeNode(node, depth, forceExpanded) {
      if (node.type === "directory") {
        const expanded = forceExpanded || expandedDirs.has(node.path) || (activePath && activePath.startsWith(node.path));
        const button = document.createElement("button");
        button.className = "tree-row dir-button";
        button.style.setProperty("--depth", String(depth));
        button.innerHTML = '<span class="tree-caret">' + (expanded ? "v" : ">") + '</span><span>' + escapeHTML(treeLabel(node)) + '</span>';
        button.addEventListener("click", () => {
          if (expandedDirs.has(node.path)) expandedDirs.delete(node.path);
          else expandedDirs.add(node.path);
          renderList();
        });
        docsEl.appendChild(button);
        if (expanded) {
          for (const child of node.children || []) {
            renderTreeNode(child, depth + 1, forceExpanded);
          }
        }
        return;
      }

      const button = document.createElement("button");
      button.className = "tree-row file-button" + (node.path === activePath ? " active" : "");
      button.style.setProperty("--depth", String(depth));
      button.innerHTML = escapeHTML(treeLabel(node));
      button.addEventListener("click", () => loadDoc(node.path));
      docsEl.appendChild(button);
    }

    async function loadDoc(path) {
      if (isDirty() && !window.confirm("Discard unsaved changes?")) return;
      activePath = path;
      editMode = false;
      renderList();
      const res = await fetch("/api/doc?path=" + encodeURIComponent(path));
      if (!res.ok) {
        currentDoc = null;
        originalContent = "";
        editorEl.value = "";
        updateEditorControls();
        contentEl.innerHTML = '<div class="empty">Unable to load document.</div>';
        return;
      }
      const doc = await res.json();
      currentDoc = doc;
      originalContent = doc.content;
      editorEl.value = doc.content;
      titleEl.textContent = doc.title;
      pathEl.textContent = doc.path;
      saveStatusEl.textContent = "";
      await renderCurrentDocument();
      updateEditorControls();
    }

    async function renderCurrentDocument() {
      if (!currentDoc) return;
      if (editMode) {
        contentEl.classList.add("hidden");
        editorEl.classList.remove("hidden");
        editorEl.focus();
        return;
      }
      editorEl.classList.add("hidden");
      contentEl.classList.remove("hidden");
      contentEl.innerHTML = renderDocument({ ...currentDoc, content: editorEl.value });
      await mermaid.run({ querySelector: ".mermaid" });
    }

    function isDirty() {
      return currentDoc && editorEl.value !== originalContent;
    }

    function updateEditorControls() {
      previewBtn.disabled = !currentDoc;
      editBtn.disabled = !currentDoc;
      saveBtn.disabled = !currentDoc || !isDirty();
      previewBtn.classList.toggle("active", !editMode);
      editBtn.classList.toggle("active", editMode);
    }

    async function saveDoc() {
      if (!currentDoc || !isDirty()) return;
      saveBtn.disabled = true;
      saveStatusEl.textContent = "Saving...";
      const res = await fetch("/api/doc?path=" + encodeURIComponent(currentDoc.path), {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: editorEl.value }),
      });
      if (!res.ok) {
        saveStatusEl.textContent = "Save failed";
        updateEditorControls();
        return;
      }
      const doc = await res.json();
      currentDoc = doc;
      originalContent = doc.content;
      editorEl.value = doc.content;
      titleEl.textContent = doc.title;
      pathEl.textContent = doc.path;
      saveStatusEl.textContent = "Saved";
      await renderCurrentDocument();
      updateEditorControls();
      window.setTimeout(() => {
        if (!isDirty()) saveStatusEl.textContent = "";
      }, 1200);
    }

    function filterTree(nodes, q) {
      const result = [];
      for (const node of nodes) {
        if (node.type === "file") {
          if (fileMatches(node, q)) result.push(node);
          continue;
        }
        const children = filterTree(node.children || [], q);
        if (children.length) result.push({ ...node, children });
      }
      return result;
    }

    function fileMatches(file, q) {
      return (treeLabel(file) + " " + displayTitle(file) + " " + file.title + " " + file.kind + " " + file.path).toLowerCase().includes(q);
    }

    function treeLabel(node) {
      return node.name || node.path;
    }

    function displayTitle(file) {
      return file.display_title || file.title || file.path;
    }

    function renderDocument(doc) {
      if (isMarkdownPath(doc.path)) {
        return renderMarkdown(doc.content);
      }
      return '<pre><code>' + escapeHTML(doc.content) + '</code></pre>';
    }

    function isMarkdownPath(path) {
      const lower = path.toLowerCase();
      return lower.endsWith(".md") || lower.endsWith(".markdown");
    }

    function renderMarkdown(source) {
      let html = "";
      const lines = source.split("\n");
      let inFence = false;
      let fenceLang = "";
      let fence = [];
      const flushParagraph = (text) => text ? "<p>" + inline(text) + "</p>" : "";
      let paragraph = "";

      for (let i = 0; i < lines.length; i++) {
        const rawLine = lines[i];
        const line = rawLine.replace(/\r$/, "");
        if (line.startsWith(fenceMarker) && line.trim() === line) {
          const fenceLangMatch = line.slice(3).trim().match(/^(\w+)?$/);
          const fenceStart = fenceLangMatch ? [line, fenceLangMatch[1]] : null;
          if (!fenceStart) {
            paragraph += (paragraph ? " " : "") + line.trim();
            continue;
          }
          if (inFence) {
            const body = fence.join("\n");
            if (fenceLang === "mermaid") {
              html += '<div class="mermaid">' + escapeHTML(body) + '</div>';
            } else {
              html += '<pre><code>' + escapeHTML(body) + '</code></pre>';
            }
            inFence = false; fenceLang = ""; fence = [];
          } else {
            html += flushParagraph(paragraph); paragraph = "";
            inFence = true; fenceLang = (fenceStart[1] || "").toLowerCase(); fence = [];
          }
          continue;
        }
        if (inFence) { fence.push(line); continue; }
        if (!line.trim()) { html += flushParagraph(paragraph); paragraph = ""; continue; }
        if (isTableStart(lines, i)) {
          html += flushParagraph(paragraph); paragraph = "";
          const table = renderTable(lines, i);
          html += table.html;
          i = table.nextIndex - 1;
          continue;
        }
        const heading = line.match(/^(#{1,6})\s+(.+)$/);
        if (heading) {
          html += flushParagraph(paragraph); paragraph = "";
          const level = heading[1].length;
          html += '<h' + level + '>' + inline(heading[2]) + '</h' + level + '>';
          continue;
        }
        const bullet = line.match(/^[-*]\s+(.+)$/);
        if (bullet) {
          html += flushParagraph(paragraph); paragraph = "";
          html += '<ul><li>' + inline(bullet[1]) + '</li></ul>';
          continue;
        }
        paragraph += (paragraph ? " " : "") + line.trim();
      }
      html += flushParagraph(paragraph);
      return html || '<div class="empty">Empty document.</div>';
    }

    function isTableStart(lines, index) {
      if (index + 1 >= lines.length) return false;
      const header = lines[index].replace(/\r$/, "");
      if (!header.includes("|")) return false;
      const headerCells = parseTableRow(header);
      const separatorCells = parseTableRow(lines[index + 1].replace(/\r$/, ""));
      return headerCells.length > 0
        && headerCells.length === separatorCells.length
        && isTableSeparator(lines[index + 1]);
    }

    function isTableSeparator(line) {
      const cells = parseTableRow(line.replace(/\r$/, ""));
      return cells.length > 0 && cells.every((cell) => /^:?-{3,}:?$/.test(cell));
    }

    function parseTableRow(line) {
      let body = line.trim();
      if (body.startsWith("|")) body = body.slice(1);
      if (body.endsWith("|")) body = body.slice(0, -1);
      return body.split("|").map((cell) => cell.trim());
    }

    function renderTable(lines, startIndex) {
      const headerCells = parseTableRow(lines[startIndex].replace(/\r$/, ""));
      const alignments = parseTableRow(lines[startIndex + 1].replace(/\r$/, "")).map(tableAlignment);
      let nextIndex = startIndex + 2;
      const bodyRows = [];
      while (nextIndex < lines.length) {
        const line = lines[nextIndex].replace(/\r$/, "");
        if (!line.trim() || !line.includes("|") || isTableSeparator(line)) break;
        const rowCells = parseTableRow(line);
        if (rowCells.length !== headerCells.length) break;
        bodyRows.push(rowCells);
        nextIndex++;
      }
      const header = headerCells.map((cell, index) => tableCell("th", cell, alignments[index])).join("");
      const body = bodyRows.map((row) => "<tr>" + row.map((cell, index) => tableCell("td", cell, alignments[index])).join("") + "</tr>").join("");
      return {
        html: "<table><thead><tr>" + header + "</tr></thead><tbody>" + body + "</tbody></table>",
        nextIndex: nextIndex,
      };
    }

    function tableAlignment(separatorCell) {
      if (/^:-{3,}:$/.test(separatorCell)) return "center";
      if (/^-{3,}:$/.test(separatorCell)) return "right";
      if (/^:-{3,}$/.test(separatorCell)) return "left";
      return "";
    }

    function tableCell(tag, value, alignment) {
      const align = alignment ? ' style="text-align: ' + alignment + '"' : "";
      return "<" + tag + align + ">" + inline(value) + "</" + tag + ">";
    }

    function inline(text) {
      return escapeHTML(text)
        .replace(inlineCodePattern, "<code>$1</code>")
        .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
    }

    previewBtn.addEventListener("click", async () => {
      editMode = false;
      await renderCurrentDocument();
      updateEditorControls();
    });
    editBtn.addEventListener("click", async () => {
      editMode = true;
      await renderCurrentDocument();
      updateEditorControls();
    });
    saveBtn.addEventListener("click", () => saveDoc());
    editorEl.addEventListener("input", () => {
      saveStatusEl.textContent = isDirty() ? "Unsaved" : "";
      updateEditorControls();
    });
    window.addEventListener("beforeunload", (event) => {
      if (!isDirty()) return;
      event.preventDefault();
      event.returnValue = "";
    });
    searchEl.addEventListener("input", () => renderList());
    loadIndex().catch((err) => {
      rootMetaEl.textContent = "Failed to load";
      contentEl.innerHTML = '<div class="empty">' + escapeHTML(String(err)) + '</div>';
    });
  </script>
</body>
</html>
`
