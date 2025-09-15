package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"

	dag "github.com/gosuda/boxo-starter-kit/02-dag-ipld/pkg"
	unixfs "github.com/gosuda/boxo-starter-kit/03-unixfs/pkg"
)

// Gateway represents an HTTP gateway for IPFS content
type Gateway struct {
	dagWrapper   *dag.DagWrapper
	unixfsSystem *unixfs.UnixFsWrapper
	port         int
	server       *http.Server
}

// GatewayConfig configures the gateway
type GatewayConfig struct {
	Port int // HTTP port to listen on (default: 8080)
}

// NewGateway creates a new HTTP gateway
func NewGateway(dagWrapper *dag.DagWrapper, unixfsSystem *unixfs.UnixFsWrapper, config GatewayConfig) *Gateway {
	if config.Port == 0 {
		config.Port = 8080
	}

	gateway := &Gateway{
		dagWrapper:   dagWrapper,
		unixfsSystem: unixfsSystem,
		port:         config.Port,
	}

	// Create HTTP server with routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", gateway.handleRoot)
	mux.HandleFunc("/ipfs/", gateway.handleIPFS)
	mux.HandleFunc("/api/v0/", gateway.handleAPI)

	gateway.server = &http.Server{
		Addr:           fmt.Sprintf(":%d", config.Port),
		Handler:        mux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	return gateway
}

// Start starts the gateway server
func (g *Gateway) Start() error {
	fmt.Printf("🌐 Gateway starting on http://localhost:%d\n", g.port)
	fmt.Printf("   Try: http://localhost:%d/ipfs/<cid>\n", g.port)
	return g.server.ListenAndServe()
}

// Stop stops the gateway server
func (g *Gateway) Stop() error {
	if g.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return g.server.Shutdown(ctx)
	}
	return nil
}

// handleRoot serves the gateway homepage
func (g *Gateway) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
    <title>IPFS Gateway</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 50px auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 40px; }
        .section { margin: 30px 0; }
        .code { background: #f5f5f5; padding: 10px; border-radius: 4px; }
        .example { margin: 10px 0; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <div class="header">
        <h1>🌐 IPFS Gateway</h1>
        <p>HTTP Gateway for IPFS Content</p>
    </div>

    <div class="section">
        <h2>📖 How to Use</h2>
        <p>Access IPFS content via HTTP:</p>
        <div class="code">http://localhost:%d/ipfs/&lt;CID&gt;</div>
    </div>

    <div class="section">
        <h2>🔗 Example URLs</h2>
        <div class="example">
            <strong>Raw content:</strong><br>
            <a href="/ipfs/QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o">/ipfs/QmT78z... (example CID)</a>
        </div>
        <div class="example">
            <strong>Directory listing:</strong><br>
            <a href="/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn">/ipfs/QmUNL... (example directory)</a>
        </div>
    </div>

    <div class="section">
        <h2>🔧 API Endpoints</h2>
        <div class="example">
            <strong>Add content:</strong><br>
            <div class="code">curl -X POST http://localhost:%d/api/v0/add -F "file=@example.txt"</div>
        </div>
        <div class="example">
            <strong>Get content info:</strong><br>
            <div class="code">curl http://localhost:%d/api/v0/object/stat?cid=&lt;CID&gt;</div>
        </div>
    </div>

    <div class="section">
        <h2>ℹ️ About</h2>
        <p>This is an educational IPFS Gateway implementation demonstrating:</p>
        <ul>
            <li>HTTP access to IPFS content</li>
            <li>UnixFS directory listings</li>
            <li>Content-Type detection</li>
            <li>Basic API endpoints</li>
        </ul>
    </div>
</body>
</html>`, g.port, g.port, g.port)
}

// handleIPFS handles /ipfs/<cid> requests
func (g *Gateway) handleIPFS(w http.ResponseWriter, r *http.Request) {
	// Extract CID from path: /ipfs/<cid>/path/to/file
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 2 || pathParts[0] != "ipfs" {
		http.Error(w, "Invalid IPFS path", http.StatusBadRequest)
		return
	}

	cidStr := pathParts[1]
	subPath := strings.Join(pathParts[2:], "/")

	// Parse CID
	c, err := cid.Parse(cidStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid CID: %s", err), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if CID exists
	exists, err := g.dagWrapper.Has(ctx, c)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to check CID: %s", err), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Content not found", http.StatusNotFound)
		return
	}

	// Try to resolve as UnixFS first
	if g.unixfsSystem != nil {
		g.handleUnixFS(w, r, c, subPath)
		return
	}

	// Fallback to raw content
	g.handleRawContent(w, r, c)
}

// handleUnixFS handles UnixFS content (files and directories)
func (g *Gateway) handleUnixFS(w http.ResponseWriter, r *http.Request, c cid.Cid, subPath string) {
	ctx := r.Context()

	// Try to get as UnixFS node
	node, err := g.unixfsSystem.Get(ctx, c)
	if err == nil {
		// Navigate to subPath if needed
		if subPath != "" {
			node, err = g.navigateToPath(ctx, node, subPath)
			if err != nil {
				http.Error(w, fmt.Sprintf("Path not found: %s", err), http.StatusNotFound)
				return
			}
		}

		// Check node type and serve accordingly
		switch n := node.(type) {
		case files.File:
			defer n.Close()
			data, err := io.ReadAll(n)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to read file: %s", err), http.StatusInternalServerError)
				return
			}
			g.serveFile(w, r, data, subPath)
			return

		case files.Directory:
			defer n.Close()
			entries := g.collectDirectoryEntries(n)
			g.serveDirectoryListing(w, r, c, subPath, entries)
			return
		}
	}

	// Fallback to raw content
	g.handleRawContent(w, r, c)
}

// handleRawContent serves raw IPLD content
func (g *Gateway) handleRawContent(w http.ResponseWriter, r *http.Request, c cid.Cid) {
	ctx := r.Context()

	// Get raw data
	data, err := g.dagWrapper.GetRaw(ctx, c)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get content: %s", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // 1 year cache for immutable content

	// Serve content
	w.Write(data)
}

// serveFile serves a file with appropriate content type
func (g *Gateway) serveFile(w http.ResponseWriter, r *http.Request, data []byte, filename string) {
	// Detect content type
	contentType := "application/octet-stream"
	if filename != "" {
		if ct := mime.TypeByExtension(filepath.Ext(filename)); ct != "" {
			contentType = ct
		}
	}

	// Set headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	// Serve content
	w.Write(data)
}

// DirectoryEntry represents a directory entry for listing
type DirectoryEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

// navigateToPath navigates through UnixFS directory structure
func (g *Gateway) navigateToPath(ctx context.Context, node files.Node, path string) (files.Node, error) {
	if path == "" {
		return node, nil
	}

	segments := strings.Split(strings.Trim(path, "/"), "/")
	currentNode := node

	for _, segment := range segments {
		dir, ok := currentNode.(files.Directory)
		if !ok {
			return nil, fmt.Errorf("not a directory")
		}

		entries := dir.Entries()
		found := false

		for entries.Next() {
			if entries.Name() == segment {
				currentNode = entries.Node()
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("path segment not found: %s", segment)
		}
	}

	return currentNode, nil
}

// collectDirectoryEntries collects directory entries for listing
func (g *Gateway) collectDirectoryEntries(dir files.Directory) []DirectoryEntry {
	var entries []DirectoryEntry

	iter := dir.Entries()
	for iter.Next() {
		name := iter.Name()
		node := iter.Node()

		entry := DirectoryEntry{
			Name:  name,
			IsDir: false,
			Size:  0,
		}

		// Check if it's a directory or file
		if _, ok := node.(files.Directory); ok {
			entry.IsDir = true
		} else if file, ok := node.(files.File); ok {
			if size, err := file.Size(); err == nil {
				entry.Size = size
			}
		}

		entries = append(entries, entry)
	}

	return entries
}

// serveDirectoryListing serves an HTML directory listing
func (g *Gateway) serveDirectoryListing(w http.ResponseWriter, r *http.Request, rootCID cid.Cid, subPath string, entries []DirectoryEntry) {
	w.Header().Set("Content-Type", "text/html")

	// Build breadcrumb path
	breadcrumbs := []struct {
		Name string
		Path string
	}{
		{"Root", "/ipfs/" + rootCID.String()},
	}

	if subPath != "" {
		parts := strings.Split(subPath, "/")
		currentPath := "/ipfs/" + rootCID.String()
		for _, part := range parts {
			currentPath = currentPath + "/" + part
			breadcrumbs = append(breadcrumbs, struct {
				Name string
				Path string
			}{part, currentPath})
		}
	}

	// Render HTML template
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>Directory: {{.Path}}</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1000px; margin: 20px auto; padding: 20px; }
        .breadcrumb { margin-bottom: 20px; }
        .breadcrumb a { color: #0066cc; text-decoration: none; margin-right: 5px; }
        .breadcrumb a:hover { text-decoration: underline; }
        table { width: 100%; border-collapse: collapse; }
        th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
        th { background-color: #f5f5f5; }
        .name { max-width: 400px; word-break: break-all; }
        .size { text-align: right; }
        .type { color: #666; }
        a { color: #0066cc; text-decoration: none; }
        a:hover { text-decoration: underline; }
        .file::before { content: "📄 "; }
        .dir::before { content: "📁 "; }
    </style>
</head>
<body>
    <h1>📁 Directory Listing</h1>

    <div class="breadcrumb">
        {{range $i, $crumb := .Breadcrumbs}}
            {{if $i}} / {{end}}
            <a href="{{$crumb.Path}}">{{$crumb.Name}}</a>
        {{end}}
    </div>

    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Type</th>
                <th class="size">Size</th>
            </tr>
        </thead>
        <tbody>
            {{if .ParentPath}}
            <tr>
                <td><a href="{{.ParentPath}}" class="dir">../</a></td>
                <td class="type">directory</td>
                <td class="size">-</td>
            </tr>
            {{end}}
            {{range .Entries}}
            <tr>
                <td class="name">
                    <a href="{{$.CurrentPath}}/{{.Name}}" class="{{if .IsDir}}dir{{else}}file{{end}}">{{.Name}}</a>
                </td>
                <td class="type">{{if .IsDir}}directory{{else}}file{{end}}</td>
                <td class="size">{{if not .IsDir}}{{.Size}} bytes{{else}}-{{end}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>

    <hr>
    <p><small>IPFS Gateway - Educational Implementation</small></p>
</body>
</html>`

	t, err := template.New("directory").Parse(tmpl)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	// Prepare template data
	currentPath := "/ipfs/" + rootCID.String()
	if subPath != "" {
		currentPath = currentPath + "/" + subPath
	}

	var parentPath string
	if subPath != "" {
		parentParts := strings.Split(subPath, "/")
		if len(parentParts) > 1 {
			parentPath = "/ipfs/" + rootCID.String() + "/" + strings.Join(parentParts[:len(parentParts)-1], "/")
		} else {
			parentPath = "/ipfs/" + rootCID.String()
		}
	}

	data := struct {
		Path        string
		CurrentPath string
		ParentPath  string
		Breadcrumbs []struct {
			Name string
			Path string
		}
		Entries []DirectoryEntry
	}{
		Path:        currentPath,
		CurrentPath: currentPath,
		ParentPath:  parentPath,
		Breadcrumbs: breadcrumbs,
		Entries:     entries,
	}

	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}

// handleAPI handles basic API endpoints
func (g *Gateway) handleAPI(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 3 || pathParts[0] != "api" || pathParts[1] != "v0" {
		http.Error(w, "Invalid API path", http.StatusBadRequest)
		return
	}

	endpoint := pathParts[2]
	switch endpoint {
	case "add":
		g.handleAPIAdd(w, r)
	case "object":
		if len(pathParts) >= 4 && pathParts[3] == "stat" {
			g.handleAPIObjectStat(w, r)
		} else {
			http.Error(w, "Unknown object endpoint", http.StatusNotFound)
		}
	default:
		http.Error(w, "Unknown API endpoint", http.StatusNotFound)
	}
}

// handleAPIAdd handles file upload via API
func (g *Gateway) handleAPIAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32 MB max
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Store in UnixFS if available
	ctx := r.Context()
	var c cid.Cid

	if g.unixfsSystem != nil {
		// Use UnixFS to store file with metadata
		fileReader := strings.NewReader(string(data))
		fileNode := files.NewReaderFile(fileReader)
		c, err = g.unixfsSystem.Put(ctx, fileNode)
	} else {
		c, err = g.dagWrapper.PersistentWrapper.Put(ctx, data)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add file: %s", err), http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"Name": header.Filename,
		"Hash": c.String(),
		"Size": len(data),
	}
	json.NewEncoder(w).Encode(response)
}

// handleAPIObjectStat handles object stat requests
func (g *Gateway) handleAPIObjectStat(w http.ResponseWriter, r *http.Request) {
	cidStr := r.URL.Query().Get("cid")
	if cidStr == "" {
		http.Error(w, "Missing CID parameter", http.StatusBadRequest)
		return
	}

	c, err := cid.Parse(cidStr)
	if err != nil {
		http.Error(w, "Invalid CID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if exists
	exists, err := g.dagWrapper.Has(ctx, c)
	if err != nil {
		http.Error(w, "Failed to check CID", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}

	// Get object info
	data, err := g.dagWrapper.GetRaw(ctx, c)
	if err != nil {
		http.Error(w, "Failed to get object", http.StatusInternalServerError)
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"Hash":           c.String(),
		"DataSize":       len(data),
		"LinksSize":      0, // Simplified
		"CumulativeSize": len(data),
		"Type":           "file", // Simplified
	}
	json.NewEncoder(w).Encode(response)
}
