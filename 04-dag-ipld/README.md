# 02-dag-ipld: 분산 데이터 구조와 IPLD

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- **DAG(Directed Acyclic Graph)**의 개념과 IPFS에서의 활용
- **IPLD(InterPlanetary Linked Data)** 데이터 모델의 이해
- **복잡한 데이터 구조**를 IPFS에 저장하고 탐색하는 방법
- **Path Resolution**을 통한 연결된 데이터 접근
- **DAGService** 인터페이스의 구현과 활용
- **코덱(Codec)**을 통한 다양한 데이터 형식 지원

## 📋 사전 요구사항

- **00-block-cid** 모듈 완료 (Block과 CID 이해)
- **01-persistent** 모듈 완료 (데이터 영속성 이해)
- JSON 데이터 구조에 대한 이해
- 그래프 이론의 기본 개념 (노드, 엣지, 사이클)

## 🔑 핵심 개념

### DAG(Directed Acyclic Graph)란?

**DAG**는 방향성이 있고 사이클이 없는 그래프입니다:

```
     A
   ↙   ↘
  B  →  C
  ↓     ↓
  D  →  E
```

**특징**:
- **방향성**: 각 연결에 방향이 있음
- **비순환**: 출발점으로 돌아오는 경로가 없음
- **불변성**: 한번 생성된 노드는 변경되지 않음

### IPLD(InterPlanetary Linked Data)란?

**IPLD**는 다양한 분산 시스템에서 데이터를 연결하고 탐색할 수 있는 데이터 모델입니다:

```json
{
  "name": "Alice",
  "age": 30,
  "friends": [
    {"/": "bafkreiabcd..."}, // CID 링크
    {"/": "bafkreiefgh..."}  // CID 링크
  ],
  "profile": {
    "bio": "Software Engineer",
    "avatar": {"/": "bafkreixyz..."}
  }
}
```

### Path Resolution

IPLD는 **경로 기반 접근**을 지원합니다:

```
/profile/bio           → "Software Engineer"
/friends/0             → {다른 사용자 객체}
/profile/avatar        → {이미지 데이터}
```

## 💻 코드 분석

### 1. DAG Wrapper 설계

```go
// pkg/dag.go:24-32
type DagWrapper struct {
    dagService     format.DAGService
    nodeGetter     format.NodeGetter
    persistentWrapper *persistent.PersistentWrapper
}

func New(pw *persistent.PersistentWrapper, datastorePath string) (*DagWrapper, error) {
    // DAGService 초기화 및 IPLD 지원 설정
}
```

**설계 특징**:
- **persistent.PersistentWrapper** 재사용으로 저장소 추상화
- **format.DAGService** 인터페이스로 IPLD 표준 준수
- **다중 코덱** 지원을 위한 유연한 아키텍처

### 2. 복잡한 데이터 구조 저장

```go
// pkg/dag.go:88-110
func (dw *DagWrapper) PutAny(ctx context.Context, data any) (cid.Cid, error) {
    // 1. JSON 직렬화
    jsonData, err := json.Marshal(data)
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to marshal data: %w", err)
    }

    // 2. DAG-JSON 코덱으로 노드 생성
    node, err := dagJSON.Decode(dagJSON.DecodeOptions{}, bytes.NewReader(jsonData))
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to decode as DAG-JSON: %w", err)
    }

    // 3. DAG에 추가
    err = dw.dagService.Add(ctx, node)
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to add node to DAG: %w", err)
    }

    return node.Cid(), nil
}
```

**핵심 과정**:
1. **직렬화**: Go 구조체 → JSON
2. **IPLD 변환**: JSON → IPLD 노드
3. **DAG 저장**: 노드를 DAG에 추가

### 3. Path Resolution 구현

```go
// pkg/dag.go:113-142
func (dw *DagWrapper) GetPath(ctx context.Context, rootCID cid.Cid, path string) (any, error) {
    if path == "" || path == "/" {
        return dw.GetAny(ctx, rootCID)
    }

    // 1. 루트 노드 조회
    rootNode, err := dw.dagService.Get(ctx, rootCID)
    if err != nil {
        return nil, fmt.Errorf("failed to get root node: %w", err)
    }

    // 2. 경로 파싱 및 탐색
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")
    currentNode := rootNode

    for _, segment := range pathSegments {
        // 3. 각 세그먼트별 노드 탐색
        nextNode, _, err := currentNode.Resolve([]string{segment})
        if err != nil {
            return nil, fmt.Errorf("failed to resolve path segment '%s': %w", segment, err)
        }

        // 4. CID 링크인 경우 실제 노드 로드
        if nextCID, ok := nextNode.(cid.Cid); ok {
            currentNode, err = dw.dagService.Get(ctx, nextCID)
            if err != nil {
                return nil, fmt.Errorf("failed to get linked node: %w", err)
            }
        }
    }

    return dw.convertIPLDtoGo(currentNode)
}
```

### 4. 연결된 데이터 구조 생성

```go
// main.go:95-115
func createLinkedData(ctx context.Context, dw *dag.DagWrapper) map[string]cid.Cid {
    cids := make(map[string]cid.Cid)

    // 1. 개별 객체들 생성
    profileCID, _ := dw.PutAny(ctx, map[string]any{
        "bio":      "IPFS Developer",
        "location": "Distributed Web",
        "skills":   []string{"Go", "IPFS", "Blockchain"},
    })

    addressCID, _ := dw.PutAny(ctx, map[string]any{
        "street": "123 Blockchain Ave",
        "city":   "Decentralized City",
        "country": "Internet",
    })

    // 2. 연결된 사용자 객체 생성
    userCID, _ := dw.PutAny(ctx, map[string]any{
        "name":    "Alice",
        "age":     30,
        "profile": map[string]string{"/": profileCID.String()}, // CID 링크
        "address": map[string]string{"/": addressCID.String()}, // CID 링크
        "metadata": map[string]any{
            "created": time.Now().Format(time.RFC3339),
            "version": "1.0",
        },
    })

    return map[string]cid.Cid{
        "user":    userCID,
        "profile": profileCID,
        "address": addressCID,
    }
}
```

**데이터 연결 구조**:
```
User
├─ name: "Alice"
├─ age: 30
├─ profile → (CID링크) → Profile
│   ├─ bio: "IPFS Developer"
│   ├─ location: "Distributed Web"
│   └─ skills: ["Go", "IPFS", "Blockchain"]
└─ address → (CID링크) → Address
    ├─ street: "123 Blockchain Ave"
    ├─ city: "Decentralized City"
    └─ country: "Internet"
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 02-dag-ipld
go run main.go
```

**예상 출력**:
```
=== DAG and IPLD Demo ===

1. Setting up DAG service with Badger backend:
   ✅ DAG service ready with persistent storage

2. Storing simple data structures:
   ✅ Stored person → bafyreigq4zsipbdvx...
   ✅ Stored company → bafyreih7x4jzrm2q...

3. Creating linked data structures:
   📦 Creating interconnected objects:
   ✅ profile → bafyreick25vk37ls...
   ✅ address → bafyreifp2eq7nbzd...
   ✅ user → bafyreigxm4b8xqrs...

4. Path resolution examples:
   🔍 Resolving paths in linked data:
   ✅ /name → Alice
   ✅ /age → 30
   ✅ /profile/bio → IPFS Developer
   ✅ /profile/skills/0 → Go
   ✅ /address/city → Decentralized City

5. Complex data operations:
   🔗 Following links across multiple objects:
   ✅ User's skills count: 3
   ✅ Profile creation time: 2024-01-15T10:30:00Z
```

### 2. Path Resolution 실험

코드를 수정하여 다양한 경로를 테스트해보세요:

```go
// 테스트할 경로들
paths := []string{
    "/name",                    // 직접 필드
    "/profile/bio",             // 링크된 객체의 필드
    "/profile/skills/1",        // 배열 인덱스
    "/address/street",          // 중첩된 링크
    "/metadata/created",        // 메타데이터
}

for _, path := range paths {
    result, err := dw.GetPath(ctx, userCID, path)
    if err != nil {
        fmt.Printf("   ❌ Failed to resolve %s: %v\n", path, err)
    } else {
        fmt.Printf("   ✅ %s → %v\n", path, result)
    }
}
```

### 3. 데이터 구조 시각화

DAG 구조를 이해하기 위해 연결 관계를 출력:

```go
func printDAGStructure(ctx context.Context, dw *dag.DagWrapper, rootCID cid.Cid, depth int) {
    indent := strings.Repeat("  ", depth)

    data, err := dw.GetAny(ctx, rootCID)
    if err != nil {
        fmt.Printf("%s❌ Error: %v\n", indent, err)
        return
    }

    fmt.Printf("%s📦 %s\n", indent, rootCID.String()[:12]+"...")

    if dataMap, ok := data.(map[string]any); ok {
        for key, value := range dataMap {
            fmt.Printf("%s├─ %s: %v\n", indent, key, value)

            // CID 링크인 경우 재귀적으로 탐색
            if linkMap, ok := value.(map[string]any); ok {
                if cidStr, exists := linkMap["/"]; exists {
                    if linkedCID, err := cid.Parse(cidStr.(string)); err == nil {
                        printDAGStructure(ctx, dw, linkedCID, depth+1)
                    }
                }
            }
        }
    }
}
```

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ 단순 객체 저장/검색
- ✅ 복잡한 중첩 구조
- ✅ Path resolution 기능
- ✅ CID 링크 해결
- ✅ 에러 처리 (잘못된 경로)

## 🔍 고급 활용 사례

### 1. 버전 관리 시스템

```go
type Document struct {
    Title     string            `json:"title"`
    Content   string            `json:"content"`
    Author    string            `json:"author"`
    Version   int               `json:"version"`
    Parent    map[string]string `json:"parent,omitempty"` // 이전 버전 링크
    Created   string            `json:"created"`
}

func createVersionedDocument(ctx context.Context, dw *dag.DagWrapper,
                           title, content, author string, parentCID *cid.Cid) (cid.Cid, error) {
    doc := Document{
        Title:   title,
        Content: content,
        Author:  author,
        Version: 1,
        Created: time.Now().Format(time.RFC3339),
    }

    if parentCID != nil {
        // 이전 버전에 링크
        doc.Parent = map[string]string{"/": parentCID.String()}

        // 버전 번호 증가
        if parentDoc, err := dw.GetAny(ctx, *parentCID); err == nil {
            if parent, ok := parentDoc.(map[string]any); ok {
                if v, exists := parent["version"]; exists {
                    doc.Version = int(v.(float64)) + 1
                }
            }
        }
    }

    return dw.PutAny(ctx, doc)
}
```

### 2. 소셜 그래프 구현

```go
type User struct {
    Name      string              `json:"name"`
    Bio       string              `json:"bio"`
    Following []map[string]string `json:"following"` // CID 링크 배열
    Followers []map[string]string `json:"followers"` // CID 링크 배열
    Posts     []map[string]string `json:"posts"`     // 게시글 CID 링크
}

func followUser(ctx context.Context, dw *dag.DagWrapper,
               followerCID, targetCID cid.Cid) error {
    // 1. 팔로워 사용자 정보 조회
    followerData, err := dw.GetAny(ctx, followerCID)
    if err != nil {
        return err
    }

    // 2. following 목록에 추가
    follower := followerData.(map[string]any)
    following := follower["following"].([]any)
    following = append(following, map[string]string{"/": targetCID.String()})
    follower["following"] = following

    // 3. 업데이트된 사용자 정보 저장
    _, err = dw.PutAny(ctx, follower)
    return err
}
```

### 3. 파일 시스템 트리

```go
type FileNode struct {
    Name     string              `json:"name"`
    Type     string              `json:"type"`     // "file" or "directory"
    Size     int64               `json:"size,omitempty"`
    Children []map[string]string `json:"children,omitempty"` // 디렉터리인 경우
    Content  map[string]string   `json:"content,omitempty"`  // 파일인 경우
}

func createFileTree(ctx context.Context, dw *dag.DagWrapper) (cid.Cid, error) {
    // 파일들 생성
    file1CID, _ := dw.PutAny(ctx, FileNode{
        Name: "README.md",
        Type: "file",
        Size: 1024,
        Content: map[string]string{"/": "bafkreifile1content..."},
    })

    file2CID, _ := dw.PutAny(ctx, FileNode{
        Name: "main.go",
        Type: "file",
        Size: 2048,
        Content: map[string]string{"/": "bafkreifile2content..."},
    })

    // 디렉터리 생성
    return dw.PutAny(ctx, FileNode{
        Name: "project",
        Type: "directory",
        Children: []map[string]string{
            {"/": file1CID.String()},
            {"/": file2CID.String()},
        },
    })
}
```

## ⚠️ 주의사항 및 모범 사례

### 1. CID 링크 생성

```go
// ✅ 올바른 CID 링크 형식
link := map[string]string{"/": targetCID.String()}

// ❌ 잘못된 형식
link := map[string]string{"cid": targetCID.String()}
link := targetCID.String() // 문자열로만 저장
```

### 2. Path Resolution 에러 처리

```go
// ✅ 경로별 세밀한 에러 처리
result, err := dw.GetPath(ctx, rootCID, "/profile/skills/10")
if err != nil {
    if strings.Contains(err.Error(), "index out of range") {
        return handleArrayIndexError(err)
    }
    if strings.Contains(err.Error(), "path not found") {
        return handlePathNotFoundError(err)
    }
    return handleGenericError(err)
}
```

### 3. 메모리 효율적인 대용량 데이터 처리

```go
// ✅ 스트리밍 방식으로 대용량 데이터 처리
func processLargeDataset(ctx context.Context, dw *dag.DagWrapper,
                        dataStream <-chan map[string]any) error {
    for data := range dataStream {
        cid, err := dw.PutAny(ctx, data)
        if err != nil {
            return err
        }

        // 처리 완료된 데이터는 메모리에서 해제
        log.Printf("Processed: %s", cid)
    }
    return nil
}
```

### 4. 순환 참조 방지

```go
// ✅ 깊이 제한으로 순환 참조 방지
func traverseDAG(ctx context.Context, dw *dag.DagWrapper,
                rootCID cid.Cid, maxDepth int) error {
    visited := make(map[string]bool)
    return traverseDAGRecursive(ctx, dw, rootCID, visited, 0, maxDepth)
}

func traverseDAGRecursive(ctx context.Context, dw *dag.DagWrapper,
                         currentCID cid.Cid, visited map[string]bool,
                         depth, maxDepth int) error {
    if depth > maxDepth {
        return fmt.Errorf("maximum depth exceeded")
    }

    cidStr := currentCID.String()
    if visited[cidStr] {
        return nil // 이미 방문한 노드
    }
    visited[cidStr] = true

    // 노드 처리 로직...
    return nil
}
```

## 🔧 트러블슈팅

### 문제 1: "path not found" 에러

**원인**: 잘못된 경로 또는 존재하지 않는 필드
```go
// 해결: 경로 유효성 검사
func validatePath(ctx context.Context, dw *dag.DagWrapper,
                  rootCID cid.Cid, path string) error {
    pathSegments := strings.Split(strings.Trim(path, "/"), "/")

    for i, segment := range pathSegments {
        partialPath := "/" + strings.Join(pathSegments[:i+1], "/")
        _, err := dw.GetPath(ctx, rootCID, partialPath)
        if err != nil {
            return fmt.Errorf("invalid path at segment '%s': %w", segment, err)
        }
    }
    return nil
}
```

### 문제 2: "node not found" 에러

**원인**: CID 링크가 가리키는 노드가 존재하지 않음
```go
// 해결: 링크 무결성 검사
func validateLinks(ctx context.Context, dw *dag.DagWrapper,
                  data map[string]any) error {
    for key, value := range data {
        if linkMap, ok := value.(map[string]any); ok {
            if cidStr, exists := linkMap["/"]; exists {
                targetCID, err := cid.Parse(cidStr.(string))
                if err != nil {
                    return fmt.Errorf("invalid CID in field '%s': %w", key, err)
                }

                exists, err := dw.Has(ctx, targetCID)
                if err != nil || !exists {
                    return fmt.Errorf("linked node not found for field '%s': %s",
                                     key, targetCID)
                }
            }
        }
    }
    return nil
}
```

### 문제 3: JSON 직렬화 에러

**원인**: 직렬화할 수 없는 데이터 타입 포함
```go
// 해결: 직렬화 가능한 데이터로 변환
func sanitizeForJSON(data any) any {
    switch v := data.(type) {
    case time.Time:
        return v.Format(time.RFC3339)
    case func():
        return nil // 함수는 제거
    case chan interface{}:
        return nil // 채널은 제거
    case map[string]any:
        result := make(map[string]any)
        for k, val := range v {
            if sanitized := sanitizeForJSON(val); sanitized != nil {
                result[k] = sanitized
            }
        }
        return result
    default:
        return v
    }
}
```

## 📊 성능 최적화

### 1. 배치 처리

```go
// ✅ 여러 노드를 배치로 처리
func putBatch(ctx context.Context, dw *dag.DagWrapper,
             items []any) ([]cid.Cid, error) {
    var cids []cid.Cid

    // 병렬 처리를 위한 워커 풀
    const workers = 4
    jobs := make(chan any, len(items))
    results := make(chan struct{cid cid.Cid; err error}, len(items))

    // 워커 시작
    for i := 0; i < workers; i++ {
        go func() {
            for item := range jobs {
                cid, err := dw.PutAny(ctx, item)
                results <- struct{cid cid.Cid; err error}{cid, err}
            }
        }()
    }

    // 작업 전송
    for _, item := range items {
        jobs <- item
    }
    close(jobs)

    // 결과 수집
    for i := 0; i < len(items); i++ {
        result := <-results
        if result.err != nil {
            return nil, result.err
        }
        cids = append(cids, result.cid)
    }

    return cids, nil
}
```

### 2. 캐싱 전략

```go
// ✅ LRU 캐시로 성능 향상
type CachedDagWrapper struct {
    *DagWrapper
    cache *lru.Cache
}

func (cdw *CachedDagWrapper) GetAny(ctx context.Context, c cid.Cid) (any, error) {
    // 캐시 확인
    if cached, ok := cdw.cache.Get(c.String()); ok {
        return cached, nil
    }

    // 캐시 미스 시 실제 조회
    result, err := cdw.DagWrapper.GetAny(ctx, c)
    if err != nil {
        return nil, err
    }

    // 캐시에 저장
    cdw.cache.Add(c.String(), result)
    return result, nil
}
```

## 📚 추가 학습 자료

### 관련 문서
- [IPLD Specification](https://ipld.io/docs/)
- [DAG-JSON Codec](https://ipld.io/docs/codecs/dag-json/)
- [IPFS DAG API](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-dag)
- [Graph Theory Basics](https://en.wikipedia.org/wiki/Directed_acyclic_graph)

### 다음 단계
1. **03-unixfs**: 파일 및 디렉터리 구조의 IPLD 표현
2. **04-network-bitswap**: 분산 네트워크에서 DAG 노드 교환
3. **07-ipns**: 변경 가능한 포인터로 DAG 루트 업데이트

## 🎓 연습 문제

### 기초 연습
1. 간단한 사용자 프로필을 IPLD로 저장하고 경로로 접근해보세요
2. 두 객체 간 상호 참조를 만들고 링크를 따라가 보세요
3. 배열 인덱스를 사용한 path resolution을 테스트해보세요

### 심화 연습
1. 블로그 시스템을 설계하여 게시글 간 링크 구조를 만들어보세요
2. 파일 시스템 트리를 구현하고 디렉터리 탐색을 구현해보세요
3. Git과 유사한 커밋 히스토리를 DAG로 표현해보세요

### 실전 과제
1. 소셜 네트워크의 팔로우 관계를 DAG로 모델링하고 추천 알고리즘을 구현해보세요
2. 문서 버전 관리 시스템을 만들어 변경 이력을 추적해보세요
3. 분산 데이터베이스의 스키마를 IPLD로 설계하고 쿼리 시스템을 구현해보세요

이제 복잡한 데이터 구조를 IPFS에서 어떻게 다루는지 이해하셨을 것입니다. 다음 모듈에서는 실제 파일과 디렉터리를 다루는 방법을 학습하겠습니다! 🚀