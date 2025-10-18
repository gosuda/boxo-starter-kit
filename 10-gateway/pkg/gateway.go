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

	dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
	unixfs "github.com/gosuda/boxo-starter-kit/06-unixfs-car/pkg"
)

// Gateway represents an HTTP gateway for IPFS content
type Gateway struct {
	dagWrapper   *dag.IpldWrapper
	unixfsSystem *unixfs.UnixFsWrapper
	port         int
	server       *http.Server
}

// GatewayConfig configures the gateway
type GatewayConfig struct {
	Port int // HTTP port to listen on (default: 8080)
}

// NewGateway creates a new HTTP gateway
func NewGateway(dagWrapper *dag.IpldWrapper, unixfsSystem *unixfs.UnixFsWrapper, config GatewayConfig) *Gateway {
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

	// Serve static files (HTML, CSS, JS) with proper MIME types
	mux.HandleFunc("/static/", gateway.handleStatic)

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

// handleStatic serves static files (CSS, JS) with proper MIME types
func (g *Gateway) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Extract filename from /static/filename
	filename := strings.TrimPrefix(r.URL.Path, "/static/")
	if filename == "" {
		http.NotFound(w, r)
		return
	}

	// Read file from embedded FS
	data, err := StaticFiles.ReadFile("static/" + filename)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set proper MIME type based on extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(filename, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(filename, ".js") {
		contentType = "application/javascript; charset=utf-8"
	} else if strings.HasSuffix(filename, ".html") {
		contentType = "text/html; charset=utf-8"
	} else if strings.HasSuffix(filename, ".json") {
		contentType = "application/json; charset=utf-8"
	} else if strings.HasSuffix(filename, ".png") {
		contentType = "image/png"
	} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
		contentType = "image/jpeg"
	} else if strings.HasSuffix(filename, ".svg") {
		contentType = "image/svg+xml"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Write(data)
}

// handleRoot serves the gateway homepage with full web UI
func (g *Gateway) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Serve index.html from embedded static files
	data, err := StaticFiles.ReadFile("static/index.html")
	if err != nil {
		// Debug: print error
		http.Error(w, fmt.Sprintf("Failed to load index.html: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
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

	fmt.Printf("📥 Request for CID: %s\n", c.String())

	// Try to resolve as UnixFS first (don't check HasBlock, just try to get it)
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

	fmt.Printf("📂 Attempting UnixFS Get for CID: %s\n", c.String())

	// Try to get as UnixFS node
	node, err := g.unixfsSystem.Get(ctx, c)
	if err != nil {
		fmt.Printf("❌ UnixFS Get failed: %v\n", err)
		// Fallback to raw content
		g.handleRawContent(w, r, c)
		return
	}

	fmt.Printf("✅ UnixFS Get succeeded, node type: %T\n", node)

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
		fmt.Printf("📄 Serving file, reading content...\n")
		defer n.Close()
		data, err := io.ReadAll(n)
		if err != nil {
			fmt.Printf("❌ Failed to read file: %v\n", err)
			http.Error(w, fmt.Sprintf("Failed to read file: %s", err), http.StatusInternalServerError)
			return
		}
		fmt.Printf("✅ Read %d bytes, serving file\n", len(data))
		g.serveFile(w, r, data, subPath)
		return

	case files.Directory:
		fmt.Printf("📁 Serving directory listing\n")
		defer n.Close()
		entries := g.collectDirectoryEntries(n)
		g.serveDirectoryListing(w, r, c, subPath, entries)
		return
	}

	// Fallback to raw content
	fmt.Printf("⚠️  Unknown node type, falling back to raw content\n")
	g.handleRawContent(w, r, c)
}

// handleRawContent serves raw IPLD content
func (g *Gateway) handleRawContent(w http.ResponseWriter, r *http.Request, c cid.Cid) {
	ctx := r.Context()

	// Get raw data
	data, err := g.dagWrapper.BlockServiceWrapper.GetBlockRaw(ctx, c)
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

	fmt.Printf("📤 Uploading file: %s (%d bytes)\n", header.Filename, len(data))

	if g.unixfsSystem != nil {
		// Use UnixFS to store file with metadata
		fileReader := strings.NewReader(string(data))
		fileNode := files.NewReaderFile(fileReader)
		c, err = g.unixfsSystem.Put(ctx, fileNode)
		if err != nil {
			fmt.Printf("❌ UnixFS Put failed: %v\n", err)
			http.Error(w, fmt.Sprintf("Failed to add file: %s", err), http.StatusInternalServerError)
			return
		}
		fmt.Printf("✅ UnixFS Put succeeded, CID: %s\n", c.String())
	} else {
		c, err = g.dagWrapper.BlockServiceWrapper.AddBlockRaw(ctx, data)
		if err != nil {
			fmt.Printf("❌ AddBlockRaw failed: %v\n", err)
			http.Error(w, fmt.Sprintf("Failed to add file: %s", err), http.StatusInternalServerError)
			return
		}
		fmt.Printf("✅ AddBlockRaw succeeded, CID: %s\n", c.String())
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"Name": header.Filename,
		"Hash": c.String(),
		"Size": len(data),
	}
	fmt.Printf("📋 Returning response: %+v\n", response)
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

	// Try to get actual file size via UnixFS first
	var actualSize int64
	var fileType string = "file"

	if g.unixfsSystem != nil {
		node, err := g.unixfsSystem.Get(ctx, c)
		if err == nil {
			// Successfully got as UnixFS node
			switch n := node.(type) {
			case files.File:
				defer n.Close()
				data, err := io.ReadAll(n)
				if err == nil {
					actualSize = int64(len(data))
				}
			case files.Directory:
				defer n.Close()
				fileType = "directory"
				actualSize = 0 // Directories don't have a simple size
			}
		}
	}

	// Fallback: get block size if UnixFS failed
	if actualSize == 0 && fileType == "file" {
		data, err := g.dagWrapper.BlockServiceWrapper.GetBlockRaw(ctx, c)
		if err != nil {
			http.Error(w, "Failed to get object", http.StatusInternalServerError)
			return
		}
		actualSize = int64(len(data))
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{
		"Hash":           c.String(),
		"DataSize":       actualSize,
		"CumulativeSize": actualSize,
		"Type":           fileType,
	}
	json.NewEncoder(w).Encode(response)
}
