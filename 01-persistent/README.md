# 01-persistent: 데이터 영속성과 저장소 전략

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- IPFS에서 **데이터 영속성**의 중요성과 구현 방법
- **4가지 저장소 백엔드**의 특성과 적합한 사용 시나리오
- **Datastore 인터페이스**를 통한 추상화의 장점
- **성능 vs 영속성** 트레이드오프 분석
- 프로덕션 환경에서의 **저장소 선택 기준**

## 📋 사전 요구사항

- **00-block-cid** 모듈 완료 (Block과 CID 이해)
- 데이터베이스의 기본 개념 (키-값 저장소)
- 파일시스템과 메모리의 차이점 이해
- Go의 인터페이스와 의존성 주입 개념

## 🔑 핵심 개념

### 데이터 영속성이란?

**영속성(Persistence)**은 프로그램이 종료되어도 데이터가 보존되는 특성입니다:

```
메모리 저장소: 빠르지만 프로그램 종료 시 데이터 손실
영구 저장소: 느리지만 데이터가 디스크에 안전하게 보관
```

### Datastore 추상화

IPFS는 **Datastore 인터페이스**를 통해 다양한 저장 백엔드를 지원합니다:

```go
type Datastore interface {
    Put(ctx context.Context, key datastore.Key, value []byte) error
    Get(ctx context.Context, key datastore.Key) (value []byte, err error)
    Has(ctx context.Context, key datastore.Key) (exists bool, err error)
    Delete(ctx context.Context, key datastore.Key) error
}
```

### 4가지 저장소 백엔드

| 백엔드 | 영속성 | 성능 | 사용 사례 |
|--------|--------|------|-----------|
| **Memory** | ❌ | 🚀 최고 | 테스트, 임시 데이터 |
| **Flatfs** | ✅ | 🏃 빠름 | 단순한 파일 저장 |
| **Badger** | ✅ | 🚀 매우 빠름 | 고성능 애플리케이션 |
| **Pebble** | ✅ | 🏃 빠름 | 대용량 데이터 처리 |

## 💻 코드 분석

### 1. Persistent Wrapper 설계

```go
// pkg/persistent.go:21-31
type PersistentWrapper struct {
    blockWrapper *block.BlockWrapper
    datastore    datastore.Datastore
    closer       io.Closer
}

func New(persistentType PersistentType, path string) (*PersistentWrapper, error) {
    // 백엔드별 초기화 로직
    return &PersistentWrapper{...}, nil
}
```

**설계 특징**:
- **block.BlockWrapper**를 재사용하여 코드 중복 방지
- **datastore.Datastore** 인터페이스로 추상화
- **io.Closer**로 리소스 정리 보장

### 2. 백엔드별 초기화 전략

#### Memory 백엔드
```go
// pkg/persistent.go:42-48
case PersistentTypeMemory:
    ds := datastore.NewMapDatastore()
    bs := blockstore.NewBlockstore(ds)
    return &PersistentWrapper{
        blockWrapper: block.New(bs),
        datastore:    ds,
        closer:       nil, // 메모리는 정리 불필요
    }, nil
```

**특징**: 즉시 사용 가능, 리소스 정리 불필요

#### Flatfs 백엔드
```go
// pkg/persistent.go:50-67
case PersistentTypeFlatfs:
    if path == "" {
        path = filepath.Join(os.TempDir(), "ipfs-flatfs")
    }

    flatfsDS, err := flatfs.CreateOrOpen(path, flatfs.IPFS_DEF_SHARD, false)
    if err != nil {
        return nil, fmt.Errorf("failed to create flatfs datastore: %w", err)
    }
```

**특징**:
- 파일시스템 기반 (디렉터리 구조)
- **샤딩(Sharding)** 지원으로 성능 최적화
- 설정 가능한 저장 경로

#### Badger 백엔드
```go
// pkg/persistent.go:72-89
case PersistentTypeBadger:
    if path == "" {
        path = filepath.Join(os.TempDir(), "ipfs-badger")
    }

    opts := badger.DefaultOptions(path)
    opts.Logger = nil // 로그 비활성화

    badgerDS, err := badger.NewDatastore(path, &opts)
```

**특징**:
- **LSM-Tree** 기반 고성능 키-값 저장소
- **압축 및 가비지 컬렉션** 자동 관리
- **트랜잭션** 지원

#### Pebble 백엔드
```go
// pkg/persistent.go:94-105
case PersistentTypePebble:
    if path == "" {
        path = filepath.Join(os.TempDir(), "ipfs-pebble")
    }

    pebbleDS, err := pebble.NewDatastore(path)
```

**특징**:
- **RocksDB** 호환 인터페이스
- **CockroachDB**에서 개발된 고성능 저장소
- **대용량 데이터** 처리에 최적화

### 3. 성능 측정 및 비교

```go
// main.go:169-189
func benchmarkBackend(ctx context.Context, pw *persistent.PersistentWrapper, backendName string, operations int) {
    start := time.Now()

    for i := 0; i < operations; i++ {
        data := []byte(fmt.Sprintf("benchmark data %d for %s", i, backendName))
        cid, err := pw.Put(ctx, data)
        if err != nil {
            log.Printf("   ❌ Failed to put data: %v", err)
            continue
        }

        _, err = pw.Get(ctx, cid)
        if err != nil {
            log.Printf("   ❌ Failed to get data: %v", err)
        }
    }

    duration := time.Since(start)
    opsPerSecond := float64(operations) / duration.Seconds()

    fmt.Printf("   📊 %s: %d ops in %v (%.0f ops/sec)\n",
        backendName, operations, duration.Round(time.Millisecond), opsPerSecond)
}
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 01-persistent
go run main.go
```

**예상 출력**:
```
=== Persistent Storage Demo ===

1. Testing Memory backend:
   ✅ Memory backend initialized
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Retrieved data matches

2. Testing Flatfs backend:
   ✅ Flatfs backend initialized at /tmp/ipfs-flatfs-demo
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Data persists after restart: true

3. Testing Badger backend:
   ✅ Badger backend initialized at /tmp/ipfs-badger-demo
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Data persists after restart: true

4. Testing Pebble backend:
   ✅ Pebble backend initialized at /tmp/ipfs-pebble-demo
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Data persists after restart: true

5. Performance comparison (1000 operations):
   📊 Memory: 1000 ops in 45ms (22222 ops/sec)
   📊 Flatfs: 1000 ops in 234ms (4274 ops/sec)
   📊 Badger: 1000 ops in 156ms (6410 ops/sec)
   📊 Pebble: 1000 ops in 189ms (5291 ops/sec)
```

### 2. 영속성 테스트

프로그램을 두 번 실행하여 데이터가 유지되는지 확인:

```bash
# 첫 번째 실행 - 데이터 저장
go run main.go

# 두 번째 실행 - 저장된 데이터 확인
go run main.go
```

**관찰 포인트**: Flatfs, Badger, Pebble은 데이터가 유지되지만 Memory는 유지되지 않음

### 3. 저장소 경로 확인

```bash
# Flatfs 저장소 내용 확인
ls -la /tmp/ipfs-flatfs-demo/

# Badger 저장소 내용 확인
ls -la /tmp/ipfs-badger-demo/

# Pebble 저장소 내용 확인
ls -la /tmp/ipfs-pebble-demo/
```

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ 4가지 백엔드 초기화
- ✅ 데이터 저장/검색 기능
- ✅ 영속성 보장 (재시작 후 데이터 유지)
- ✅ 에러 처리 및 리소스 정리

## 🔍 성능 분석

### 벤치마크 결과 해석

일반적인 성능 순서 (환경에 따라 다를 수 있음):

1. **Memory** (20,000+ ops/sec)
   - 가장 빠름, 하지만 영속성 없음
   - 테스트 및 캐시 용도

2. **Badger** (6,000+ ops/sec)
   - 영구 저장소 중 가장 빠름
   - 압축 및 최적화 기능

3. **Pebble** (5,000+ ops/sec)
   - 대용량 처리에 강함
   - CockroachDB 검증된 안정성

4. **Flatfs** (4,000+ ops/sec)
   - 단순하고 안정적
   - 디버깅 및 검사 용이

### 메모리 사용량 패턴

```go
// pkg/persistent.go:213-230
func (pw *PersistentWrapper) GetStats() (*DatastoreStats, error) {
    // 메모리 사용량 통계 수집
    var m runtime.MemStats
    runtime.ReadMemStats(&m)

    return &DatastoreStats{
        TotalBlocks:   pw.blockCount,
        TotalSize:     pw.totalSize,
        MemoryUsage:   m.Alloc,
        LastAccessed:  pw.lastAccessed,
    }, nil
}
```

## 🔗 실제 활용 사례

### 1. 개발 환경별 백엔드 선택

```go
func selectBackend(env string) persistent.PersistentType {
    switch env {
    case "test":
        return persistent.PersistentTypeMemory    // 빠른 테스트
    case "development":
        return persistent.PersistentTypeFlatfs    // 디버깅 용이
    case "production":
        return persistent.PersistentTypeBadger    // 고성능
    case "large-scale":
        return persistent.PersistentTypePebble    // 대용량 처리
    default:
        return persistent.PersistentTypeMemory
    }
}
```

### 2. 설정 기반 초기화

```go
type Config struct {
    Backend     string `json:"backend"`
    DataPath    string `json:"data_path"`
    Performance string `json:"performance"`
}

func initStorage(config Config) (*persistent.PersistentWrapper, error) {
    backendType := persistent.ParsePersistentType(config.Backend)
    return persistent.New(backendType, config.DataPath)
}
```

### 3. 자동 마이그레이션

```go
func migrateStorage(oldPath, newPath string,
                   oldType, newType persistent.PersistentType) error {
    // 기존 저장소에서 데이터 읽기
    oldStore, err := persistent.New(oldType, oldPath)
    if err != nil {
        return err
    }
    defer oldStore.Close()

    // 새 저장소로 데이터 복사
    newStore, err := persistent.New(newType, newPath)
    if err != nil {
        return err
    }
    defer newStore.Close()

    // 데이터 마이그레이션 로직...
}
```

## ⚠️ 주의사항 및 모범 사례

### 1. 백엔드별 적합한 사용 시나리오

```go
// ✅ 권장 사용법
switch useCase {
case "unit-testing":
    backend = persistent.PersistentTypeMemory
case "integration-testing":
    backend = persistent.PersistentTypeFlatfs
case "high-performance-app":
    backend = persistent.PersistentTypeBadger
case "large-dataset":
    backend = persistent.PersistentTypePebble
}
```

### 2. 리소스 정리

```go
// ✅ 항상 리소스 정리
defer func() {
    if err := persistentWrapper.Close(); err != nil {
        log.Printf("Failed to close persistent storage: %v", err)
    }
}()
```

### 3. 에러 처리 전략

```go
// ✅ 백엔드별 특화된 에러 처리
cid, err := pw.Put(ctx, data)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "disk space"):
        return handleDiskSpaceError(err)
    case strings.Contains(err.Error(), "permission"):
        return handlePermissionError(err)
    default:
        return handleGenericError(err)
    }
}
```

### 4. 성능 모니터링

```go
// ✅ 정기적인 성능 측정
ticker := time.NewTicker(5 * time.Minute)
go func() {
    for range ticker.C {
        stats, err := pw.GetStats()
        if err == nil {
            log.Printf("Storage stats: %d blocks, %d bytes",
                stats.TotalBlocks, stats.TotalSize)
        }
    }
}()
```

## 🔧 트러블슈팅

### 문제 1: "permission denied" 에러

**원인**: 저장소 디렉터리 접근 권한 부족
```bash
# 해결: 권한 확인 및 수정
ls -la /path/to/storage/
chmod 755 /path/to/storage/
```

### 문제 2: "disk space" 에러

**원인**: 디스크 공간 부족
```bash
# 해결: 디스크 공간 확인
df -h /path/to/storage/

# 불필요한 데이터 정리
du -sh /path/to/storage/*
```

### 문제 3: Badger "database locked" 에러

**원인**: 다중 프로세스가 같은 Badger DB 접근
```go
// 해결: 프로세스당 고유 경로 사용
path := fmt.Sprintf("/tmp/badger-%d", os.Getpid())
```

### 문제 4: 성능 저하

**원인**: 부적절한 백엔드 선택 또는 설정
```go
// 해결: 벤치마크 기반 백엔드 선택
results := benchmarkAllBackends()
optimalBackend := selectOptimalBackend(results)
```

## 📊 백엔드 선택 가이드

### 결정 트리

```
데이터 영속성 필요?
├─ 아니오 → Memory
└─ 예
   ├─ 고성능 필요?
   │  ├─ 예 → Badger
   │  └─ 아니오 → Flatfs
   └─ 대용량 데이터?
      ├─ 예 → Pebble
      └─ 아니오 → Badger
```

### 상세 비교표

| 기준 | Memory | Flatfs | Badger | Pebble |
|------|--------|--------|--------|--------|
| **영속성** | ❌ | ✅ | ✅ | ✅ |
| **성능** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **메모리 사용** | 높음 | 낮음 | 보통 | 보통 |
| **설정 복잡도** | 낮음 | 낮음 | 보통 | 보통 |
| **디버깅 용이성** | 높음 | 높음 | 보통 | 낮음 |
| **압축 지원** | ❌ | ❌ | ✅ | ✅ |
| **트랜잭션** | ❌ | ❌ | ✅ | ✅ |
| **적합한 데이터 크기** | 소규모 | 중간 | 대규모 | 초대규모 |

## 📚 추가 학습 자료

### 관련 문서
- [go-datastore Documentation](https://github.com/ipfs/go-datastore)
- [Badger Documentation](https://dgraph.io/docs/badger/)
- [Pebble Documentation](https://github.com/cockroachdb/pebble)
- [IPFS Datastore Interface](https://docs.ipfs.io/concepts/glossary/#datastore)

### 다음 단계
1. **02-dag-ipld**: 복잡한 데이터 구조와 연결된 데이터 학습
2. **03-unixfs**: 파일시스템 기능과 대용량 파일 처리
3. **05-pin-gc**: 데이터 생명주기 관리와 가비지 컬렉션

## 🎓 연습 문제

### 기초 연습
1. 각 백엔드로 같은 데이터를 저장하고 성능을 비교해보세요
2. 프로그램을 재시작한 후 영구 저장소에서 데이터가 유지되는지 확인하세요
3. 잘못된 경로로 저장소를 초기화할 때의 에러를 처리해보세요

### 심화 연습
1. 설정 파일을 읽어서 동적으로 백엔드를 선택하는 시스템을 만들어보세요
2. 저장소 통계를 주기적으로 수집하고 모니터링하는 시스템을 구현해보세요
3. 한 백엔드에서 다른 백엔드로 데이터를 마이그레이션하는 도구를 만들어보세요

### 실전 과제
1. 웹 API를 통해 저장소 상태를 확인할 수 있는 관리 도구를 만들어보세요
2. 여러 백엔드를 동시에 사용하는 하이브리드 저장소를 설계해보세요
3. 자동으로 최적 백엔드를 선택하는 지능형 저장소 매니저를 구현해보세요

이제 다양한 저장소 백엔드의 특성과 선택 기준을 이해하셨을 것입니다. 다음 모듈에서는 더 복잡한 데이터 구조를 다루는 방법을 학습하겠습니다! 🚀