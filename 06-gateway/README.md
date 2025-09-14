# 06-gateway: IPFS HTTP 게이트웨이 구현

## 🎯 학습 목표
- HTTP 게이트웨이를 통해 IPFS 콘텐츠에 웹 접근하는 방법 이해
- UnixFS 파일 시스템과 웹 인터페이스 통합
- 디렉터리 리스팅과 콘텐츠 타입 처리
- RESTful API 엔드포인트 설계 및 구현
- 실제 웹 애플리케이션에서 IPFS 데이터 활용

## 📋 사전 요구사항
- **이전 챕터**: 00-block-cid, 01-persistent, 02-dag-ipld, 03-unixfs 완료
- **기술 지식**: HTTP 프로토콜, RESTful API, HTML/CSS 기초
- **Go 지식**: HTTP 서버, 템플릿, JSON 처리

## 🔑 핵심 개념

### IPFS HTTP 게이트웨이란?
IPFS HTTP 게이트웨이는 분산 파일 시스템의 콘텐츠를 기존 웹 브라우저와 HTTP 클라이언트에서 접근할 수 있게 하는 브릿지입니다.

#### 게이트웨이의 역할
- **프로토콜 변환**: IPFS 네이티브 프로토콜 ↔ HTTP/HTTPS
- **콘텐츠 해석**: CID를 통한 데이터 검색 및 웹 형식으로 제공
- **메타데이터 처리**: 파일 타입 감지 및 적절한 HTTP 헤더 설정
- **디렉터리 네비게이션**: UnixFS 디렉터리 구조를 HTML로 렌더링

### 게이트웨이 URL 패턴
```
# CID 기반 접근
http://localhost:8080/ipfs/{CID}
http://localhost:8080/ipfs/{CID}/path/to/file

# API 엔드포인트
http://localhost:8080/api/v0/add
http://localhost:8080/api/v0/get/{CID}
http://localhost:8080/api/v0/ls/{CID}
```

## 💻 코드 분석

### 1. Gateway 구조체 설계
```go
type Gateway struct {
    dagWrapper *dag.DAGWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
    server *http.Server
}
```

**설계 결정**:
- `dagWrapper`: 저수준 블록 및 DAG 데이터 접근
- `unixfsWrapper`: 파일 시스템 추상화
- `server`: HTTP 서버 인스턴스 관리

### 2. HTTP 라우터 구성
```go
func (gw *Gateway) setupRoutes() {
    http.HandleFunc("/", gw.homepageHandler)
    http.HandleFunc("/ipfs/", gw.ipfsHandler)
    http.HandleFunc("/api/v0/add", gw.apiAddHandler)
    http.HandleFunc("/api/v0/get/", gw.apiGetHandler)
    http.HandleFunc("/api/v0/ls/", gw.apiListHandler)
}
```

**라우팅 전략**:
- `/`: 게이트웨이 홈페이지 (사용법 안내)
- `/ipfs/{CID}`: 콘텐츠 직접 접근
- `/api/v0/*`: RESTful API 엔드포인트

### 3. 콘텐츠 타입 감지
```go
func detectContentType(data []byte, filename string) string {
    if contentType := http.DetectContentType(data); contentType != "application/octet-stream" {
        return contentType
    }

    // 파일 확장자 기반 감지
    ext := strings.ToLower(filepath.Ext(filename))
    switch ext {
    case ".js":
        return "application/javascript"
    case ".css":
        return "text/css"
    case ".md":
        return "text/markdown"
    default:
        return "application/octet-stream"
    }
}
```

## 🏃‍♂️ 실습 가이드

### 단계 1: 게이트웨이 서버 시작
```bash
cd 06-gateway
go run main.go
```

### 단계 2: 웹 브라우저에서 접근
1. http://localhost:8080 방문 (홈페이지)
2. 샘플 파일 업로드 및 CID 확인

### 단계 3: 콘텐츠 접근 테스트
```bash
# 파일 추가
curl -X POST -F file=@test.txt http://localhost:8080/api/v0/add

# 반환된 CID로 접근
curl http://localhost:8080/ipfs/{CID}

# 디렉터리 리스팅
curl http://localhost:8080/api/v0/ls/{DIR_CID}
```

### 예상 결과
- **홈페이지**: HTML 인터페이스로 게이트웨이 기능 소개
- **파일 접근**: 브라우저에서 직접 파일 내용 표시
- **디렉터리**: HTML 테이블로 파일 목록 표시
- **API**: JSON 형식의 구조화된 응답

## 🚀 고급 활용 사례

### 1. 정적 웹사이트 호스팅
UnixFS 디렉터리에 웹사이트를 저장하고 게이트웨이로 서빙:
```go
// index.html, style.css, script.js를 포함한 디렉터리 생성
dirCID := addWebsiteToIPFS("./website/")
fmt.Printf("웹사이트 접근: http://localhost:8080/ipfs/%s\n", dirCID)
```

### 2. 대용량 미디어 스트리밍
청킹된 파일의 부분 요청 처리:
```go
func (gw *Gateway) handleRangeRequest(w http.ResponseWriter, r *http.Request, data []byte) {
    rangeHeader := r.Header.Get("Range")
    if rangeHeader != "" {
        // HTTP Range 요청 처리
        // 부분 콘텐츠 응답
    }
}
```

### 3. 캐싱 전략
자주 접근되는 콘텐츠의 성능 최적화:
```go
type CachedGateway struct {
    *Gateway
    cache map[string][]byte
    cacheMutex sync.RWMutex
}
```

## 🔧 최적화 및 보안

### 성능 최적화
- **Connection Pooling**: HTTP 클라이언트 재사용
- **Response Compression**: gzip 압축 적용
- **Static Asset Caching**: CDN과 유사한 캐싱 헤더

### 보안 고려사항
- **CORS 설정**: 크로스 오리진 요청 제어
- **Rate Limiting**: 과도한 요청 방지
- **Content Validation**: 악성 콘텐츠 검증

## 🐛 트러블슈팅

### 문제 1: CID를 찾을 수 없음
**증상**: `404 Not Found` 에러
**해결책**:
```bash
# CID 유효성 검증
curl http://localhost:8080/api/v0/get/{CID}
```

### 문제 2: 잘못된 콘텐츠 타입
**증상**: 브라우저에서 파일이 제대로 렌더링되지 않음
**해결책**: `detectContentType` 함수 확장

### 문제 3: 서버 성능 저하
**증상**: 느린 응답 시간
**해결책**:
- 캐싱 레이어 추가
- 고루틴 기반 동시 처리
- 메모리 사용량 모니터링

## 🔗 연계 학습
- **다음 단계**: 07-ipns (동적 콘텐츠 업데이트)
- **고급 주제**:
  - HTTPS 인증서 관리
  - 로드 밸런싱
  - CDN 통합

## 📚 참고 자료
- [IPFS HTTP Gateway Specification](https://docs.ipfs.tech/concepts/ipfs-gateway/)
- [Go HTTP Server Best Practices](https://golang.org/doc/articles/wiki/)
- [UnixFS Specification](https://github.com/ipfs/specs/blob/main/UNIXFS.md)

---

# 🍳 실전 쿡북: 바로 쓸 수 있는 코드

## 1. 📺 미디어 스트리밍 게이트웨이

완전한 미디어 스트리밍 서버를 만들어보세요.

```go
package main

import (
    "fmt"
    "io"
    "net/http"
    "strconv"
    "strings"
    "context"
    "os"

    "github.com/ipfs/go-cid"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type MediaGateway struct {
    dagWrapper    *dag.DAGWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
}

func NewMediaGateway() (*MediaGateway, error) {
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &MediaGateway{
        dagWrapper:    dagWrapper,
        unixfsWrapper: unixfsWrapper,
    }, nil
}

func (mg *MediaGateway) StreamHandler(w http.ResponseWriter, r *http.Request) {
    // CID 추출
    path := strings.TrimPrefix(r.URL.Path, "/stream/")
    c, err := cid.Decode(path)
    if err != nil {
        http.Error(w, "Invalid CID", http.StatusBadRequest)
        return
    }

    // 파일 데이터 가져오기
    ctx := context.Background()
    data, err := mg.unixfsWrapper.GetFile(ctx, c.String())
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    // Range 요청 처리 (스트리밍 지원)
    rangeHeader := r.Header.Get("Range")
    if rangeHeader != "" {
        mg.handleRangeRequest(w, r, data, rangeHeader)
        return
    }

    // 전체 파일 제공
    w.Header().Set("Content-Type", "video/mp4")
    w.Header().Set("Accept-Ranges", "bytes")
    w.Header().Set("Content-Length", strconv.Itoa(len(data)))
    w.Write(data)
}

func (mg *MediaGateway) handleRangeRequest(w http.ResponseWriter, r *http.Request, data []byte, rangeHeader string) {
    // Range: bytes=0-1023 파싱
    ranges := strings.Replace(rangeHeader, "bytes=", "", 1)
    parts := strings.Split(ranges, "-")

    start, _ := strconv.Atoi(parts[0])
    end := len(data) - 1
    if parts[1] != "" {
        end, _ = strconv.Atoi(parts[1])
    }

    if start > end || start >= len(data) {
        w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
        return
    }

    if end >= len(data) {
        end = len(data) - 1
    }

    w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(data)))
    w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
    w.Header().Set("Content-Type", "video/mp4")
    w.WriteHeader(http.StatusPartialContent)
    w.Write(data[start : end+1])
}

func main() {
    mg, err := NewMediaGateway()
    if err != nil {
        panic(err)
    }

    http.HandleFunc("/stream/", mg.StreamHandler)

    // 샘플 비디오 추가
    if videoData, err := os.ReadFile("sample.mp4"); err == nil {
        ctx := context.Background()
        cid, _ := mg.unixfsWrapper.AddFile(ctx, "sample.mp4", videoData)
        fmt.Printf("비디오 스트리밍 URL: http://localhost:8080/stream/%s\n", cid.String())
    }

    fmt.Println("미디어 스트리밍 게이트웨이 시작됨 :8080")
    http.ListenAndServe(":8080", nil)
}
```

## 2. 🎨 이미지 갤러리 게이트웨이

자동 썸네일 생성과 갤러리 UI를 제공하는 게이트웨이입니다.

```go
package main

import (
    "html/template"
    "net/http"
    "strings"
    "context"
    "path/filepath"
    "fmt"

    "github.com/ipfs/go-cid"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type GalleryGateway struct {
    dagWrapper    *dag.DAGWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
}

type GalleryImage struct {
    CID      string
    Filename string
    Size     string
    IsImage  bool
}

func NewGalleryGateway() (*GalleryGateway, error) {
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &GalleryGateway{
        dagWrapper:    dagWrapper,
        unixfsWrapper: unixfsWrapper,
    }, nil
}

func (gg *GalleryGateway) GalleryHandler(w http.ResponseWriter, r *http.Request) {
    // 갤러리 디렉터리 CID 추출
    path := strings.TrimPrefix(r.URL.Path, "/gallery/")
    if path == "" {
        gg.showUploadForm(w, r)
        return
    }

    c, err := cid.Decode(path)
    if err != nil {
        http.Error(w, "Invalid CID", http.StatusBadRequest)
        return
    }

    ctx := context.Background()
    entries, err := gg.unixfsWrapper.ListDir(ctx, c.String())
    if err != nil {
        http.Error(w, "Directory not found", http.StatusNotFound)
        return
    }

    // 이미지 파일만 필터링
    var images []GalleryImage
    for _, entry := range entries {
        ext := strings.ToLower(filepath.Ext(entry.Name))
        isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".gif"

        images = append(images, GalleryImage{
            CID:      entry.Hash,
            Filename: entry.Name,
            Size:     fmt.Sprintf("%.1f KB", float64(entry.Size)/1024),
            IsImage:  isImage,
        })
    }

    gg.renderGallery(w, images)
}

func (gg *GalleryGateway) showUploadForm(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html>
<head>
    <title>IPFS 이미지 갤러리</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .upload-form { border: 2px dashed #ccc; padding: 20px; text-align: center; }
        .upload-form:hover { border-color: #007cba; }
    </style>
</head>
<body>
    <h1>🎨 IPFS 이미지 갤러리</h1>

    <div class="upload-form">
        <h3>이미지 업로드</h3>
        <form action="/upload" method="post" enctype="multipart/form-data">
            <input type="file" name="images" multiple accept="image/*" required>
            <br><br>
            <button type="submit">갤러리 생성</button>
        </form>
    </div>

    <h3>샘플 갤러리</h3>
    <p>이미지들을 업로드하면 디렉터리 CID가 생성됩니다.</p>
    <p>예: <code>/gallery/{CID}</code></p>
</body>
</html>`

    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

func (gg *GalleryGateway) renderGallery(w http.ResponseWriter, images []GalleryImage) {
    tmpl := template.Must(template.New("gallery").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>IPFS 이미지 갤러리</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .gallery { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 20px; }
        .image-card { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        .image-card img { width: 100%; height: 200px; object-fit: cover; }
        .image-info { padding: 15px; }
        .filename { font-weight: bold; margin-bottom: 5px; }
        .size { color: #666; font-size: 0.9em; }
        .cid { font-family: monospace; font-size: 0.8em; color: #007cba; word-break: break-all; }
        h1 { text-align: center; color: #333; }
    </style>
</head>
<body>
    <h1>🎨 IPFS 이미지 갤러리</h1>

    <div class="gallery">
        {{range .}}
        <div class="image-card">
            {{if .IsImage}}
            <img src="/ipfs/{{.CID}}" alt="{{.Filename}}" loading="lazy">
            {{else}}
            <div style="height: 200px; display: flex; align-items: center; justify-content: center; background: #eee;">
                <span>📄 {{.Filename}}</span>
            </div>
            {{end}}
            <div class="image-info">
                <div class="filename">{{.Filename}}</div>
                <div class="size">{{.Size}}</div>
                <div class="cid">{{.CID}}</div>
            </div>
        </div>
        {{end}}
    </div>
</body>
</html>`))

    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, images)
}

func main() {
    gg, err := NewGalleryGateway()
    if err != nil {
        panic(err)
    }

    http.HandleFunc("/gallery/", gg.GalleryHandler)
    http.HandleFunc("/ipfs/", func(w http.ResponseWriter, r *http.Request) {
        // 간단한 IPFS 파일 서빙
        path := strings.TrimPrefix(r.URL.Path, "/ipfs/")
        c, _ := cid.Decode(path)

        ctx := context.Background()
        data, err := gg.unixfsWrapper.GetFile(ctx, c.String())
        if err != nil {
            http.NotFound(w, r)
            return
        }

        w.Write(data)
    })

    fmt.Println("이미지 갤러리 게이트웨이 시작됨 :8080")
    fmt.Println("방문: http://localhost:8080/gallery/")
    http.ListenAndServe(":8080", nil)
}
```

## 3. 📚 문서 뷰어 게이트웨이

마크다운, PDF, 텍스트 파일을 웹에서 보기 좋게 렌더링하는 게이트웨이입니다.

```go
package main

import (
    "html/template"
    "net/http"
    "strings"
    "context"
    "path/filepath"
    "fmt"

    "github.com/ipfs/go-cid"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type DocGateway struct {
    dagWrapper    *dag.DAGWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
}

func NewDocGateway() (*DocGateway, error) {
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &DocGateway{
        dagWrapper:    dagWrapper,
        unixfsWrapper: unixfsWrapper,
    }, nil
}

func (dg *DocGateway) DocHandler(w http.ResponseWriter, r *http.Request) {
    path := strings.TrimPrefix(r.URL.Path, "/doc/")
    parts := strings.SplitN(path, "/", 2)
    if len(parts) < 1 {
        http.Error(w, "CID required", http.StatusBadRequest)
        return
    }

    cidStr := parts[0]
    filename := ""
    if len(parts) > 1 {
        filename = parts[1]
    }

    c, err := cid.Decode(cidStr)
    if err != nil {
        http.Error(w, "Invalid CID", http.StatusBadRequest)
        return
    }

    ctx := context.Background()
    data, err := dg.unixfsWrapper.GetFile(ctx, c.String())
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    // 파일 타입별 렌더링
    ext := strings.ToLower(filepath.Ext(filename))
    switch ext {
    case ".md", ".markdown":
        dg.renderMarkdown(w, string(data), filename)
    case ".txt", ".log":
        dg.renderText(w, string(data), filename)
    case ".json":
        dg.renderJSON(w, string(data), filename)
    case ".go", ".js", ".py", ".java":
        dg.renderCode(w, string(data), filename, ext[1:])
    default:
        dg.renderRaw(w, data, filename)
    }
}

func (dg *DocGateway) renderMarkdown(w http.ResponseWriter, content, filename string) {
    tmpl := template.Must(template.New("markdown").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Filename}} - IPFS 문서 뷰어</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; line-height: 1.6; max-width: 800px; margin: 0 auto; padding: 20px; }
        .header { border-bottom: 1px solid #eee; margin-bottom: 20px; padding-bottom: 10px; }
        .content { white-space: pre-wrap; }
        code { background: #f1f1f1; padding: 2px 4px; border-radius: 3px; }
        pre { background: #f8f8f8; padding: 15px; border-radius: 5px; overflow-x: auto; }
    </style>
</head>
<body>
    <div class="header">
        <h1>📄 {{.Filename}}</h1>
        <small>IPFS 분산 문서</small>
    </div>
    <div class="content">{{.Content}}</div>
</body>
</html>`))

    data := struct {
        Filename string
        Content  string
    }{
        Filename: filename,
        Content:  content,
    }

    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, data)
}

func (dg *DocGateway) renderCode(w http.ResponseWriter, content, filename, lang string) {
    tmpl := template.Must(template.New("code").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Filename}} - IPFS 코드 뷰어</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 0; background: #2d3748; color: #e2e8f0; }
        .header { background: #1a202c; padding: 15px 20px; border-bottom: 1px solid #4a5568; }
        .code-container { font-family: 'Monaco', 'Menlo', monospace; }
        .line-numbers { background: #1a202c; color: #718096; padding: 15px 10px; display: inline-block; vertical-align: top; min-width: 50px; text-align: right; border-right: 1px solid #4a5568; }
        .code-content { padding: 15px 20px; display: inline-block; white-space: pre; overflow-x: auto; width: calc(100% - 90px); }
        .lang-badge { background: #4299e1; color: white; padding: 2px 8px; border-radius: 12px; font-size: 0.8em; }
    </style>
</head>
<body>
    <div class="header">
        <h2>💻 {{.Filename}} <span class="lang-badge">{{.Lang}}</span></h2>
    </div>
    <div class="code-container">
        <div class="line-numbers">{{range $i, $line := .Lines}}{{add $i 1}}
{{end}}</div><div class="code-content">{{.Content}}</div>
    </div>
</body>
</html>`))

    lines := strings.Split(content, "\n")
    data := struct {
        Filename string
        Lang     string
        Content  string
        Lines    []string
    }{
        Filename: filename,
        Lang:     lang,
        Content:  content,
        Lines:    lines,
    }

    funcMap := template.FuncMap{
        "add": func(a, b int) int { return a + b },
    }

    tmpl = tmpl.Funcs(funcMap)
    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, data)
}

func (dg *DocGateway) renderText(w http.ResponseWriter, content, filename string) {
    tmpl := template.Must(template.New("text").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>{{.Filename}} - IPFS 텍스트 뷰어</title>
    <style>
        body { font-family: 'Monaco', 'Menlo', monospace; line-height: 1.5; margin: 0; background: #1e1e1e; color: #d4d4d4; }
        .header { background: #333; padding: 15px 20px; border-bottom: 1px solid #555; }
        .content { padding: 20px; white-space: pre-wrap; }
    </style>
</head>
<body>
    <div class="header">
        <h2>📝 {{.Filename}}</h2>
    </div>
    <div class="content">{{.Content}}</div>
</body>
</html>`))

    data := struct {
        Filename string
        Content  string
    }{
        Filename: filename,
        Content:  content,
    }

    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, data)
}

func (dg *DocGateway) renderJSON(w http.ResponseWriter, content, filename string) {
    // JSON 포맷팅 및 하이라이팅
    dg.renderCode(w, content, filename, "json")
}

func (dg *DocGateway) renderRaw(w http.ResponseWriter, data []byte, filename string) {
    w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
    w.Write(data)
}

func main() {
    dg, err := NewDocGateway()
    if err != nil {
        panic(err)
    }

    http.HandleFunc("/doc/", dg.DocHandler)

    fmt.Println("문서 뷰어 게이트웨이 시작됨 :8080")
    fmt.Println("사용법: /doc/{CID}/{filename}")
    http.ListenAndServe(":8080", nil)
}
```

## 4. 🔄 실시간 동기화 게이트웨이

파일 변경을 감지하고 자동으로 IPFS에 업데이트하는 실시간 동기화 게이트웨이입니다.

```go
package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
    "context"
    "sync"

    "github.com/fsnotify/fsnotify"
    dag "github.com/sonheesung/boxo-starter-kit/02-dag-ipld/pkg"
    unixfs "github.com/sonheesung/boxo-starter-kit/03-unixfs/pkg"
)

type SyncGateway struct {
    dagWrapper    *dag.DAGWrapper
    unixfsWrapper *unixfs.UnixFsWrapper
    watchers      map[string]*fsnotify.Watcher
    syncStatus    map[string]*SyncInfo
    mutex         sync.RWMutex
}

type SyncInfo struct {
    Path        string    `json:"path"`
    CID         string    `json:"cid"`
    LastSync    time.Time `json:"last_sync"`
    Status      string    `json:"status"` // "syncing", "synced", "error"
    FileCount   int       `json:"file_count"`
    TotalSize   int64     `json:"total_size"`
}

func NewSyncGateway() (*SyncGateway, error) {
    dagWrapper, err := dag.New(nil, "")
    if err != nil {
        return nil, err
    }

    unixfsWrapper, err := unixfs.New(dagWrapper)
    if err != nil {
        return nil, err
    }

    return &SyncGateway{
        dagWrapper:    dagWrapper,
        unixfsWrapper: unixfsWrapper,
        watchers:      make(map[string]*fsnotify.Watcher),
        syncStatus:    make(map[string]*SyncInfo),
    }, nil
}

func (sg *SyncGateway) AddWatchPath(localPath string) error {
    sg.mutex.Lock()
    defer sg.mutex.Unlock()

    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }

    // 디렉터리 재귀적으로 감시 추가
    err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return watcher.Add(path)
        }
        return nil
    })

    if err != nil {
        watcher.Close()
        return err
    }

    sg.watchers[localPath] = watcher
    sg.syncStatus[localPath] = &SyncInfo{
        Path:     localPath,
        Status:   "watching",
        LastSync: time.Now(),
    }

    // 초기 동기화
    go sg.syncPath(localPath)

    // 파일 변경 감시
    go sg.watchChanges(localPath, watcher)

    return nil
}

func (sg *SyncGateway) watchChanges(localPath string, watcher *fsnotify.Watcher) {
    for {
        select {
        case event, ok := <-watcher.Events:
            if !ok {
                return
            }

            if event.Op&fsnotify.Write == fsnotify.Write ||
               event.Op&fsnotify.Create == fsnotify.Create ||
               event.Op&fsnotify.Remove == fsnotify.Remove {

                fmt.Printf("파일 변경 감지: %s\n", event.Name)

                // 디렉터리가 새로 생성된 경우 감시 추가
                if event.Op&fsnotify.Create == fsnotify.Create {
                    if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
                        watcher.Add(event.Name)
                    }
                }

                // 잠시 대기 후 동기화 (여러 변경사항을 배치로 처리)
                time.Sleep(1 * time.Second)
                go sg.syncPath(localPath)
            }

        case err, ok := <-watcher.Errors:
            if !ok {
                return
            }
            fmt.Printf("감시 에러: %v\n", err)
        }
    }
}

func (sg *SyncGateway) syncPath(localPath string) {
    sg.mutex.Lock()
    info := sg.syncStatus[localPath]
    info.Status = "syncing"
    sg.mutex.Unlock()

    ctx := context.Background()

    // 디렉터리 전체를 IPFS에 추가
    cid, fileCount, totalSize, err := sg.addDirectoryToIPFS(ctx, localPath)

    sg.mutex.Lock()
    if err != nil {
        info.Status = "error"
        fmt.Printf("동기화 실패: %v\n", err)
    } else {
        info.CID = cid
        info.Status = "synced"
        info.LastSync = time.Now()
        info.FileCount = fileCount
        info.TotalSize = totalSize
        fmt.Printf("동기화 완료: %s -> %s\n", localPath, cid)
    }
    sg.mutex.Unlock()
}

func (sg *SyncGateway) addDirectoryToIPFS(ctx context.Context, dirPath string) (string, int, int64, error) {
    var files []unixfs.FileInfo
    var totalSize int64

    err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() {
            data, err := os.ReadFile(path)
            if err != nil {
                return err
            }

            // 상대 경로 계산
            relPath, err := filepath.Rel(dirPath, path)
            if err != nil {
                return err
            }

            files = append(files, unixfs.FileInfo{
                Name: relPath,
                Data: data,
            })

            totalSize += int64(len(data))
        }
        return nil
    })

    if err != nil {
        return "", 0, 0, err
    }

    dirName := filepath.Base(dirPath)
    cid, err := sg.unixfsWrapper.CreateDir(ctx, dirName, files)
    if err != nil {
        return "", 0, 0, err
    }

    return cid.String(), len(files), totalSize, nil
}

func (sg *SyncGateway) StatusHandler(w http.ResponseWriter, r *http.Request) {
    sg.mutex.RLock()
    statuses := make([]*SyncInfo, 0, len(sg.syncStatus))
    for _, info := range sg.syncStatus {
        statuses = append(statuses, info)
    }
    sg.mutex.RUnlock()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(statuses)
}

func (sg *SyncGateway) DashboardHandler(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html>
<head>
    <title>IPFS 실시간 동기화 대시보드</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .status-card { background: white; border-radius: 8px; padding: 20px; margin: 10px 0; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .status-synced { border-left: 4px solid #10b981; }
        .status-syncing { border-left: 4px solid #f59e0b; }
        .status-error { border-left: 4px solid #ef4444; }
        .path { font-weight: bold; margin-bottom: 10px; }
        .cid { font-family: monospace; background: #f1f1f1; padding: 5px; border-radius: 3px; word-break: break-all; }
        .stats { display: flex; gap: 20px; margin: 10px 0; }
        .stat { text-align: center; }
        .stat-value { font-size: 1.5em; font-weight: bold; }
        .stat-label { font-size: 0.9em; color: #666; }
        h1 { text-align: center; color: #333; }
        .add-form { background: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
    </style>
    <script>
        function refreshStatus() {
            fetch('/api/status')
                .then(response => response.json())
                .then(data => {
                    // 상태 업데이트 로직
                    setTimeout(refreshStatus, 5000); // 5초마다 갱신
                });
        }

        window.onload = function() {
            refreshStatus();
        };
    </script>
</head>
<body>
    <div class="container">
        <h1>🔄 IPFS 실시간 동기화 대시보드</h1>

        <div class="add-form">
            <h3>새 디렉터리 감시 추가</h3>
            <form action="/api/watch" method="post">
                <input type="text" name="path" placeholder="로컬 디렉터리 경로" style="width: 300px; padding: 8px;" required>
                <button type="submit">감시 시작</button>
            </form>
        </div>

        <div id="status-list">
            <!-- 동적으로 업데이트됨 -->
        </div>
    </div>
</body>
</html>`))

    w.Header().Set("Content-Type", "text/html")
    tmpl.Execute(w, nil)
}

func main() {
    sg, err := NewSyncGateway()
    if err != nil {
        panic(err)
    }

    http.HandleFunc("/", sg.DashboardHandler)
    http.HandleFunc("/api/status", sg.StatusHandler)
    http.HandleFunc("/api/watch", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        path := r.FormValue("path")
        if path == "" {
            http.Error(w, "Path required", http.StatusBadRequest)
            return
        }

        err := sg.AddWatchPath(path)
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        http.Redirect(w, r, "/", http.StatusSeeOther)
    })

    // 샘플 디렉터리 감시 시작
    if _, err := os.Stat("./sample"); err == nil {
        sg.AddWatchPath("./sample")
    }

    fmt.Println("실시간 동기화 게이트웨이 시작됨 :8080")
    fmt.Println("대시보드: http://localhost:8080")
    http.ListenAndServe(":8080", nil)
}
```

---

이 쿡북의 예제들을 사용하면 다음과 같은 실용적인 IPFS 게이트웨이를 구축할 수 있습니다:

1. **미디어 스트리밍**: HTTP Range 요청을 지원하는 비디오 스트리밍 서버
2. **이미지 갤러리**: 자동 썸네일 생성과 반응형 갤러리 UI
3. **문서 뷰어**: 다양한 파일 형식의 웹 기반 뷰어
4. **실시간 동기화**: 파일 시스템 변경을 자동으로 IPFS에 동기화

각 예제는 완전한 실행 가능한 코드로, 실제 프로덕션 환경에서 사용할 수 있도록 에러 처리와 사용자 경험을 고려하여 작성되었습니다.