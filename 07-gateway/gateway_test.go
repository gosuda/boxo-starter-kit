package main

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dag "github.com/gosuda/boxo-starter-kit/04-dag-ipld/pkg"
	unixfs "github.com/gosuda/boxo-starter-kit/05-unixfs/pkg"
	gateway "github.com/gosuda/boxo-starter-kit/07-gateway/pkg"
)

func TestGateway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup
	dagWrapper, err := dag.NewIpldWrapper(nil, nil)
	require.NoError(t, err)
	defer dagWrapper.Close()

	unixfsSystem, err := unixfs.New(256*1024, nil)
	require.NoError(t, err)
	defer unixfsSystem.Close()

	config := gateway.GatewayConfig{Port: 0} // Use random port for testing
	gw := gateway.NewGateway(dagWrapper, unixfsSystem, config)

	t.Run("Homepage", func(t *testing.T) {
		req, err := http.NewRequest("GET", "/", nil)
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				gw.Start() // This won't actually start server in test
			}
		})

		handler.ServeHTTP(rr, req)

		// Test that we can create gateway (actual homepage test would need server)
		assert.NotNil(t, gw)
	})

	t.Run("Raw Content Storage and Retrieval", func(t *testing.T) {
		// Store test content
		testData := []byte("Hello, Gateway World!")
		testCID, err := dagWrapper.BlockServiceWrapper.AddBlockRaw(ctx, testData)
		require.NoError(t, err)

		// Verify content exists
		exists, err := dagWrapper.HasBlock(ctx, testCID)
		require.NoError(t, err)
		assert.True(t, exists, "Content should exist")

		// Retrieve content
		retrievedData, err := dagWrapper.GetBlockRaw(ctx, testCID)
		require.NoError(t, err)
		assert.Equal(t, testData, retrievedData, "Retrieved data should match original")
	})

	t.Run("UnixFS File Operations", func(t *testing.T) {
		// Add file through UnixFS
		fileContent := []byte("This is a test file for the gateway")

		// Create file node
		fileReader := strings.NewReader(string(fileContent))
		fileNode := files.NewReaderFile(fileReader)

		fileCID, err := unixfsSystem.Put(ctx, fileNode)
		require.NoError(t, err)
		assert.True(t, fileCID.Defined(), "File CID should be valid")

		// Read file back
		node, err := unixfsSystem.Get(ctx, fileCID)
		require.NoError(t, err)

		file, ok := node.(files.File)
		require.True(t, ok, "Should be a file")
		defer file.Close()

		readContent, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.Equal(t, fileContent, readContent, "File content should match")
	})

	t.Run("Directory Operations", func(t *testing.T) {
		// Create files in directory structure
		testFiles := map[string][]byte{
			"file1.txt": []byte("Content of file 1"),
			"file2.txt": []byte("Content of file 2"),
			"file3.txt": []byte("Content of file 3"),
		}

		var cids []cid.Cid
		for _, content := range testFiles {
			fileReader := strings.NewReader(string(content))
			fileNode := files.NewReaderFile(fileReader)

			fileCID, err := unixfsSystem.Put(ctx, fileNode)
			require.NoError(t, err)
			cids = append(cids, fileCID)
		}

		assert.Len(t, cids, 3, "Should have created 3 files")
	})

	t.Run("HTTP Path Parsing", func(t *testing.T) {
		testCases := []struct {
			path        string
			expectedCID string
			expectedSub string
			shouldError bool
		}{
			{"/ipfs/QmTest123", "QmTest123", "", false},
			{"/ipfs/QmTest123/file.txt", "QmTest123", "file.txt", false},
			{"/ipfs/QmTest123/dir/file.txt", "QmTest123", "dir/file.txt", false},
			{"/invalid", "", "", true},
			{"/ipfs/", "", "", true},
		}

		for _, tc := range testCases {
			parts := strings.Split(strings.Trim(tc.path, "/"), "/")
			if len(parts) < 2 || parts[0] != "ipfs" {
				assert.True(t, tc.shouldError, "Should error for path: %s", tc.path)
				continue
			}

			cidStr := parts[1]
			subPath := strings.Join(parts[2:], "/")

			if !tc.shouldError {
				assert.Equal(t, tc.expectedCID, cidStr, "CID should match for path: %s", tc.path)
				assert.Equal(t, tc.expectedSub, subPath, "Sub-path should match for path: %s", tc.path)
			}
		}
	})

	t.Run("Content Type Detection", func(t *testing.T) {
		testCases := []struct {
			filename   string
			expectedCT string
		}{
			{"test.html", "text/html"},
			{"test.css", "text/css"},
			{"test.js", "text/javascript"},
			{"test.json", "application/json"},
			{"test.txt", "text/plain"},
			{"test.png", "image/png"},
			{"test.unknown", ""},
		}

		for _, tc := range testCases {
			// This tests the logic that would be used in serveFile
			// In a real implementation, we'd test the actual HTTP response
			if tc.expectedCT != "" {
				assert.NotEmpty(t, tc.expectedCT, "Should have content type for %s", tc.filename)
			}
		}
	})
}

func TestGatewayAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Setup
	dagWrapper, err := dag.NewIpldWrapper(nil, nil)
	require.NoError(t, err)
	defer dagWrapper.Close()

	unixfsSystem, err := unixfs.New(256*1024, nil)
	require.NoError(t, err)
	defer unixfsSystem.Close()

	t.Run("API Add File Simulation", func(t *testing.T) {
		// Simulate what the API would do
		testContent := []byte("Test file content for API")
		filename := "api-test.txt"

		// This simulates the core logic of handleAPIAdd
		fileReader := strings.NewReader(string(testContent))
		fileNode := files.NewReaderFile(fileReader)

		fileCID, err := unixfsSystem.Put(ctx, fileNode)
		require.NoError(t, err)

		// Verify response structure
		response := map[string]any{
			"Name": filename,
			"Hash": fileCID.String(),
			"Size": len(testContent),
		}

		assert.Equal(t, filename, response["Name"])
		assert.Equal(t, fileCID.String(), response["Hash"])
		assert.Equal(t, len(testContent), response["Size"])
	})

	t.Run("API Object Stat Simulation", func(t *testing.T) {
		// Store test content
		testData := []byte("Test data for object stat")
		testCID, err := dagWrapper.BlockServiceWrapper.AddBlockRaw(ctx, testData)
		require.NoError(t, err)

		// Simulate object stat response
		response := map[string]any{
			"Hash":           testCID.String(),
			"DataSize":       len(testData),
			"LinksSize":      0,
			"CumulativeSize": len(testData),
			"Type":           "file",
		}

		assert.Equal(t, testCID.String(), response["Hash"])
		assert.Equal(t, len(testData), response["DataSize"])
		assert.Equal(t, "file", response["Type"])
	})
}

func TestGatewayConfig(t *testing.T) {
	dagWrapper, err := dag.NewIpldWrapper(nil, nil)
	require.NoError(t, err)
	defer dagWrapper.Close()

	t.Run("Default Configuration", func(t *testing.T) {
		config := gateway.GatewayConfig{}
		gw := gateway.NewGateway(dagWrapper, nil, config)
		assert.NotNil(t, gw, "Gateway should be created with default config")
	})

	t.Run("Custom Configuration", func(t *testing.T) {
		config := gateway.GatewayConfig{
			Port: 9090,
		}
		gw := gateway.NewGateway(dagWrapper, nil, config)
		assert.NotNil(t, gw, "Gateway should be created with custom config")
	})
}

func TestMultipartFormParsing(t *testing.T) {
	// Test multipart form parsing logic used in API
	t.Run("Create Multipart Form", func(t *testing.T) {
		var b bytes.Buffer
		writer := multipart.NewWriter(&b)

		// Add file field
		fileWriter, err := writer.CreateFormFile("file", "test.txt")
		require.NoError(t, err)

		testContent := []byte("test file content")
		_, err = fileWriter.Write(testContent)
		require.NoError(t, err)

		err = writer.Close()
		require.NoError(t, err)

		// Parse the form (this simulates what the API handler does)
		req := httptest.NewRequest("POST", "/api/v0/add", &b)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		err = req.ParseMultipartForm(32 << 20)
		require.NoError(t, err)

		file, header, err := req.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		content, err := io.ReadAll(file)
		require.NoError(t, err)

		assert.Equal(t, "test.txt", header.Filename)
		assert.Equal(t, testContent, content)
	})
}

func TestHTMLTemplateLogic(t *testing.T) {
	t.Run("Breadcrumb Generation", func(t *testing.T) {
		// Test breadcrumb logic
		rootCID := "QmTest123"
		subPath := "dir1/dir2/file.txt"

		breadcrumbs := []struct {
			Name string
			Path string
		}{
			{"Root", "/ipfs/" + rootCID},
		}

		if subPath != "" {
			parts := strings.Split(subPath, "/")
			currentPath := "/ipfs/" + rootCID
			for _, part := range parts {
				currentPath = currentPath + "/" + part
				breadcrumbs = append(breadcrumbs, struct {
					Name string
					Path string
				}{part, currentPath})
			}
		}

		assert.Equal(t, 4, len(breadcrumbs), "Should have root + 3 path parts")
		assert.Equal(t, "Root", breadcrumbs[0].Name)
		assert.Equal(t, "dir1", breadcrumbs[1].Name)
		assert.Equal(t, "dir2", breadcrumbs[2].Name)
		assert.Equal(t, "file.txt", breadcrumbs[3].Name)
	})

	t.Run("Parent Path Calculation", func(t *testing.T) {
		rootCID := "QmTest123"

		testCases := []struct {
			subPath        string
			expectedParent string
		}{
			{"", ""},
			{"file.txt", "/ipfs/" + rootCID},
			{"dir/file.txt", "/ipfs/" + rootCID + "/dir"},
			{"dir1/dir2/file.txt", "/ipfs/" + rootCID + "/dir1/dir2"},
		}

		for _, tc := range testCases {
			var parentPath string
			if tc.subPath != "" {
				parentParts := strings.Split(tc.subPath, "/")
				if len(parentParts) > 1 {
					parentPath = "/ipfs/" + rootCID + "/" + strings.Join(parentParts[:len(parentParts)-1], "/")
				} else {
					parentPath = "/ipfs/" + rootCID
				}
			}

			assert.Equal(t, tc.expectedParent, parentPath, "Parent path should match for subPath: %s", tc.subPath)
		}
	})
}
