# 05-pin-gc: 데이터 생명주기 관리와 가비지 컬렉션

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- **Pin(핀)**의 개념과 데이터 영속성 보장 방법
- **가비지 컬렉션(GC)**의 원리와 디스크 공간 관리
- **Pin 타입**의 종류와 각각의 사용 시나리오
- **데이터 생명주기** 관리 전략과 자동화
- **스토리지 최적화** 기법과 성능 튜닝
- **Pin 정책** 설계와 실무 적용 방안

## 📋 사전 요구사항

- **00-block-cid** 모듈 완료 (Block과 CID 이해)
- **01-persistent** 모듈 완료 (데이터 영속성 이해)
- **02-dag-ipld** 모듈 완료 (DAG와 연결된 데이터)
- **03-unixfs** 모듈 완료 (파일 시스템과 청킹)
- 메모리 관리와 가비지 컬렉션의 기본 개념

## 🔑 핵심 개념

### Pin(핀)이란?

**Pin**은 IPFS에서 특정 데이터가 가비지 컬렉션되지 않도록 보호하는 메커니즘입니다:

```
일반 데이터:   [Block] ← GC로 삭제 가능
핀된 데이터:   [Block] + 📌 ← GC에서 보호됨
```

### Pin 타입

| 타입 | 설명 | 사용 예 |
|------|------|---------|
| **Direct** | 특정 블록만 핀 | 단일 파일, 작은 데이터 |
| **Recursive** | 연결된 모든 블록 핀 | 디렉터리, 복잡한 구조 |
| **Indirect** | 다른 핀의 의존성 | 자동 관리, 내부 참조 |

### 가비지 컬렉션 과정

```
1. 스캔: 모든 블록 조사
2. 마크: 핀된 블록과 의존성 표시
3. 스윕: 표시되지 않은 블록 삭제
4. 통계: 회수된 공간 보고
```

### 데이터 생명주기

```
생성 → 사용 → 핀 설정 → 보존 → 핀 해제 → GC 후보 → 삭제
    ↑                           ↓
    └─────── 재핀 (선택) ←────────┘
```

## 💻 코드 분석

### 1. Pin Manager 설계

```go
// pkg/pin.go:25-40
type PinManager struct {
    dagWrapper *dag.DagWrapper
    pinner     pin.Pinner
    gcManager  *GCManager
    stats      struct {
        mutex        sync.RWMutex
        TotalPins    int64 `json:"total_pins"`
        DirectPins   int64 `json:"direct_pins"`
        RecursivePins int64 `json:"recursive_pins"`
        IndirectPins int64 `json:"indirect_pins"`
        LastGC       time.Time `json:"last_gc"`
        SpaceReclaimed int64 `json:"space_reclaimed"`
    }
}

func NewPinManager(dagWrapper *dag.DagWrapper) (*PinManager, error) {
    // Pinner 인터페이스 초기화
    pinner := pin.NewPinner(dagWrapper.GetDatastore(), dagWrapper.GetDAGService(), dagWrapper.GetDAGService())

    return &PinManager{
        dagWrapper: dagWrapper,
        pinner:     pinner,
        gcManager:  NewGCManager(dagWrapper),
    }, nil
}
```

**설계 특징**:
- **pin.Pinner** 인터페이스로 표준 Pin 관리
- **GCManager** 분리로 관심사 분리
- **통계 수집**으로 상태 모니터링
- **동시성 안전**한 Pin 작업

### 2. Pin 추가 (다양한 타입)

```go
// pkg/pin.go:65-95
func (pm *PinManager) PinAdd(ctx context.Context, c cid.Cid, pinType PinType) error {
    pm.stats.mutex.Lock()
    defer pm.stats.mutex.Unlock()

    switch pinType {
    case PinTypeDirect:
        // 1. Direct Pin: 특정 블록만 핀
        err := pm.pinner.Pin(ctx, c, false)
        if err != nil {
            return fmt.Errorf("failed to pin direct: %w", err)
        }
        pm.stats.DirectPins++
        fmt.Printf("   📌 Direct pin added: %s\n", c.String()[:20]+"...")

    case PinTypeRecursive:
        // 2. Recursive Pin: 모든 연결된 블록 핀
        err := pm.pinner.Pin(ctx, c, true)
        if err != nil {
            return fmt.Errorf("failed to pin recursive: %w", err)
        }
        pm.stats.RecursivePins++
        fmt.Printf("   🔗 Recursive pin added: %s\n", c.String()[:20]+"...")

        // 연결된 블록 수 계산
        linkedBlocks := pm.countLinkedBlocks(ctx, c)
        fmt.Printf("      Protecting %d linked blocks\n", linkedBlocks)

    default:
        return fmt.Errorf("unsupported pin type: %v", pinType)
    }

    pm.stats.TotalPins++
    return pm.pinner.Flush(ctx)
}
```

### 3. Pin 상태 조회

```go
// pkg/pin.go:130-165
func (pm *PinManager) ListPins(ctx context.Context, pinType PinType) ([]PinInfo, error) {
    var pins []PinInfo

    // 1. Pin 타입별 조회
    switch pinType {
    case PinTypeDirect:
        directPins, err := pm.pinner.DirectKeys(ctx)
        if err != nil {
            return nil, err
        }
        for c := range directPins {
            pins = append(pins, PinInfo{
                CID:  c,
                Type: PinTypeDirect,
                Size: pm.getBlockSize(ctx, c),
            })
        }

    case PinTypeRecursive:
        recursivePins, err := pm.pinner.RecursiveKeys(ctx)
        if err != nil {
            return nil, err
        }
        for c := range recursivePins {
            pins = append(pins, PinInfo{
                CID:  c,
                Type: PinTypeRecursive,
                Size: pm.calculateRecursiveSize(ctx, c),
            })
        }

    case PinTypeIndirect:
        indirectPins, err := pm.pinner.InternalPins(ctx)
        if err != nil {
            return nil, err
        }
        for c := range indirectPins {
            pins = append(pins, PinInfo{
                CID:  c,
                Type: PinTypeIndirect,
                Size: pm.getBlockSize(ctx, c),
            })
        }

    case PinTypeAll:
        // 2. 모든 Pin 타입 조회
        allTypes := []PinType{PinTypeDirect, PinTypeRecursive, PinTypeIndirect}
        for _, pt := range allTypes {
            typePins, err := pm.ListPins(ctx, pt)
            if err != nil {
                continue
            }
            pins = append(pins, typePins...)
        }
    }

    return pins, nil
}
```

### 4. 가비지 컬렉션 구현

```go
// pkg/gc.go:25-70
type GCManager struct {
    dagWrapper *dag.DagWrapper
    pinner     pin.Pinner
    stats      GCStats
}

func (gcm *GCManager) RunGC(ctx context.Context, options GCOptions) (*GCResult, error) {
    fmt.Printf("   🗑️  Starting garbage collection...\n")
    startTime := time.Now()

    // 1. GC 통계 초기화
    result := &GCResult{
        StartTime:    startTime,
        RemovedCIDs:  make([]cid.Cid, 0),
        TotalRemoved: 0,
        SpaceFreed:   0,
    }

    // 2. 모든 블록 스캔
    allBlocks, err := gcm.getAllBlocks(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get all blocks: %w", err)
    }

    fmt.Printf("      Scanning %d total blocks...\n", len(allBlocks))

    // 3. 핀된 블록과 의존성 마킹
    pinnedBlocks := make(map[cid.Cid]bool)
    err = gcm.markPinnedBlocks(ctx, pinnedBlocks)
    if err != nil {
        return nil, fmt.Errorf("failed to mark pinned blocks: %w", err)
    }

    fmt.Printf("      %d blocks are pinned (protected)\n", len(pinnedBlocks))

    // 4. 가비지 블록 식별 및 삭제
    for _, blockCID := range allBlocks {
        if !pinnedBlocks[blockCID] {
            blockSize := gcm.getBlockSize(ctx, blockCID)

            // 삭제 실행
            if options.DryRun {
                fmt.Printf("      [DRY RUN] Would remove: %s (%d bytes)\n",
                          blockCID.String()[:20]+"...", blockSize)
            } else {
                err := gcm.dagWrapper.Delete(ctx, blockCID)
                if err != nil {
                    fmt.Printf("      ⚠️  Failed to remove %s: %v\n", blockCID, err)
                    continue
                }
            }

            result.RemovedCIDs = append(result.RemovedCIDs, blockCID)
            result.SpaceFreed += blockSize
        }
    }

    // 5. 결과 정리
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.TotalRemoved = len(result.RemovedCIDs)

    fmt.Printf("   ✅ GC completed in %v\n", result.Duration)
    fmt.Printf("      Removed %d blocks, freed %d bytes\n",
              result.TotalRemoved, result.SpaceFreed)

    return result, nil
}
```

### 5. 자동 GC 스케줄링

```go
// pkg/scheduler.go:20-60
type GCScheduler struct {
    pinManager    *PinManager
    gcInterval    time.Duration
    threshold     GCThreshold
    isRunning     bool
    stopChannel   chan bool
    lastGC        time.Time
}

func NewGCScheduler(pm *PinManager, interval time.Duration) *GCScheduler {
    return &GCScheduler{
        pinManager:  pm,
        gcInterval:  interval,
        threshold:   DefaultGCThreshold,
        stopChannel: make(chan bool),
    }
}

func (gcs *GCScheduler) Start(ctx context.Context) {
    if gcs.isRunning {
        return
    }

    gcs.isRunning = true
    fmt.Printf("   ⏰ GC scheduler started (interval: %v)\n", gcs.gcInterval)

    go func() {
        ticker := time.NewTicker(gcs.gcInterval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                // 1. GC 실행 조건 확인
                if gcs.shouldRunGC(ctx) {
                    fmt.Printf("   🔄 Automatic GC triggered\n")

                    result, err := gcs.pinManager.RunGC(ctx, GCOptions{
                        DryRun: false,
                        Force:  false,
                    })

                    if err != nil {
                        fmt.Printf("   ❌ Automatic GC failed: %v\n", err)
                    } else {
                        fmt.Printf("   ✅ Automatic GC freed %d bytes\n", result.SpaceFreed)
                        gcs.lastGC = time.Now()
                    }
                }

            case <-gcs.stopChannel:
                fmt.Printf("   ⏹️  GC scheduler stopped\n")
                return
            }
        }
    }()
}

func (gcs *GCScheduler) shouldRunGC(ctx context.Context) bool {
    // 2. GC 실행 조건 검사
    stats := gcs.pinManager.GetStats()

    // 디스크 사용량 체크
    if stats.DiskUsage > gcs.threshold.MaxDiskUsage {
        return true
    }

    // 시간 기반 체크
    if time.Since(gcs.lastGC) > gcs.threshold.MaxAge {
        return true
    }

    // 블록 수 기반 체크
    if stats.TotalBlocks > gcs.threshold.MaxBlocks {
        return true
    }

    return false
}
```

### 6. Pin 정책 관리

```go
// pkg/policy.go:15-55
type PinPolicy struct {
    rules       []PinRule
    autoPin     bool
    retention   time.Duration
    maxPinSize  int64
}

type PinRule struct {
    Pattern    string        `json:"pattern"`     // CID prefix or content type
    PinType    PinType       `json:"pin_type"`
    TTL        time.Duration `json:"ttl"`
    Priority   int           `json:"priority"`
    Condition  string        `json:"condition"`   // size, age, access_count
}

func (pp *PinPolicy) ShouldPin(ctx context.Context, c cid.Cid, metadata *ContentMetadata) (bool, PinType) {
    // 1. 규칙별 평가
    for _, rule := range pp.rules {
        if pp.matchesRule(c, metadata, rule) {
            fmt.Printf("   📋 Pin rule matched: %s -> %v\n", rule.Pattern, rule.PinType)
            return true, rule.PinType
        }
    }

    // 2. 기본 정책
    if pp.autoPin {
        // 크기 기반 자동 Pin 결정
        if metadata.Size < pp.maxPinSize {
            return true, PinTypeDirect
        }
    }

    return false, PinTypeDirect
}

func (pp *PinPolicy) matchesRule(c cid.Cid, metadata *ContentMetadata, rule PinRule) bool {
    // 3. 패턴 매칭
    if rule.Pattern != "" {
        if strings.HasPrefix(c.String(), rule.Pattern) {
            return true
        }
    }

    // 4. 조건 평가
    switch rule.Condition {
    case "size_gt":
        return metadata.Size > rule.MaxSize
    case "age_lt":
        return time.Since(metadata.Created) < rule.TTL
    case "access_count_gt":
        return metadata.AccessCount > rule.MinAccess
    }

    return false
}
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 05-pin-gc
go run main.go
```

**예상 출력**:
```
=== Pin Management and Garbage Collection Demo ===

1. Setting up Pin Manager:
   ✅ Pin manager initialized
   ✅ GC manager ready

2. Adding various content types:
   💾 Adding text data...
   📌 Direct pin added: bafkreigh2akiscai...
   💾 Adding binary data...
   📌 Direct pin added: bafkreibc4uoyerf...
   💾 Adding large structured data...
   🔗 Recursive pin added: bafybeihdwdcwfw...
      Protecting 15 linked blocks

3. Pin status overview:
   📊 Pin Statistics:
      Total pins: 3
      Direct pins: 2
      Recursive pins: 1
      Indirect pins: 0

   📋 Detailed pin list:
   📌 bafkreigh2a... (direct, 245 bytes)
   📌 bafkreibc4u... (direct, 1024 bytes)
   🔗 bafybeihdwd... (recursive, 15360 bytes)

4. Garbage collection demonstration:
   💾 Adding temporary data (not pinned)...
   🗑️  Starting garbage collection...
      Scanning 25 total blocks...
      5 blocks are pinned (protected)
      [DRY RUN] Would remove: bafkreitemp1... (512 bytes)
      [DRY RUN] Would remove: bafkreitemp2... (1024 bytes)
   ✅ GC completed in 45ms
      Would remove 2 blocks, would free 1536 bytes

   🗑️  Running actual GC...
   ✅ GC completed in 32ms
      Removed 2 blocks, freed 1536 bytes

5. Automatic GC scheduling:
   ⏰ GC scheduler started (interval: 30s)
   📊 Monitoring thresholds:
      Max disk usage: 100MB
      Max age since last GC: 1h
      Max blocks: 10000

6. Pin policy demonstration:
   📋 Applying pin policies...
   📋 Pin rule matched: large_files -> recursive
   📌 Auto-pinned large file as recursive
   📋 Pin rule matched: temp_* -> none
   ⚠️  Temporary file not pinned (will be GC'd)
```

### 2. Pin 타입 비교 실험

```bash
# 다양한 Pin 타입의 동작 확인
PIN_DEMO_MODE=comparison go run main.go
```

**관찰 포인트**:
- **Direct Pin**: 특정 블록만 보호, 빠른 Pin/Unpin
- **Recursive Pin**: 연결된 모든 블록 보호, 완전한 보존
- **Indirect Pin**: 자동 관리, 참조 관계 유지

### 3. GC 성능 벤치마크

```bash
# 대용량 데이터로 GC 성능 측정
GC_BENCHMARK=true go run main.go
```

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ Pin 추가/제거 기능
- ✅ 다양한 Pin 타입 처리
- ✅ GC 정확성 검증
- ✅ 스케줄링 및 자동화
- ✅ Pin 정책 적용

## 🔍 고급 활용 사례

### 1. 분산 백업 시스템

```go
type DistributedBackupManager struct {
    pinManager *PinManager
    replicas   int
    backupPolicy *BackupPolicy
}

func (dbm *DistributedBackupManager) BackupWithPolicy(data []byte,
                                                     policy BackupPolicy) error {
    ctx := context.Background()

    // 1. 백업 정책에 따른 Pin 타입 결정
    pinType := policy.DeterminePinType(len(data))

    // 2. 원본 데이터 Pin
    originalCID, err := dbm.pinManager.PinData(ctx, data, pinType)
    if err != nil {
        return fmt.Errorf("failed to pin original: %w", err)
    }

    // 3. 복제본 생성 및 Pin
    for i := 0; i < dbm.replicas; i++ {
        replicaData := dbm.createReplica(data, i)
        replicaCID, err := dbm.pinManager.PinData(ctx, replicaData, PinTypeDirect)
        if err != nil {
            log.Printf("Failed to create replica %d: %v", i, err)
            continue
        }

        fmt.Printf("Created replica %d: %s\n", i, replicaCID.String()[:20]+"...")
    }

    // 4. 백업 메타데이터 저장
    metadata := BackupMetadata{
        OriginalCID: originalCID,
        CreatedAt:   time.Now(),
        Policy:      policy,
        Replicas:    dbm.replicas,
    }

    return dbm.storeBackupMetadata(metadata)
}
```

### 2. 콘텐츠 생명주기 관리

```go
type ContentLifecycleManager struct {
    pinManager *PinManager
    policies   map[string]*LifecyclePolicy
    scheduler  *time.Ticker
}

type LifecyclePolicy struct {
    HotPeriod    time.Duration // 자주 접근되는 기간
    WarmPeriod   time.Duration // 가끔 접근되는 기간
    ColdPeriod   time.Duration // 거의 접근되지 않는 기간
    ArchivePeriod time.Duration // 아카이브 기간
}

func (clm *ContentLifecycleManager) ManageContent(cid cid.Cid,
                                                 metadata ContentMetadata) error {
    policy := clm.policies[metadata.ContentType]
    age := time.Since(metadata.CreatedAt)

    switch {
    case age < policy.HotPeriod:
        // HOT: Recursive Pin으로 완전 보호
        return clm.pinManager.PinAdd(ctx, cid, PinTypeRecursive)

    case age < policy.WarmPeriod:
        // WARM: Direct Pin으로 기본 보호
        return clm.pinManager.PinAdd(ctx, cid, PinTypeDirect)

    case age < policy.ColdPeriod:
        // COLD: Pin 해제, 압축 저장
        clm.pinManager.PinRemove(ctx, cid)
        return clm.compressAndStore(cid, metadata)

    default:
        // ARCHIVE: 외부 스토리지로 이전
        return clm.archiveToExternalStorage(cid, metadata)
    }
}
```

### 3. 지능형 캐시 관리

```go
type IntelligentCache struct {
    pinManager   *PinManager
    accessStats  map[cid.Cid]*AccessStats
    cachePolicy  *CachePolicy
    maxCacheSize int64
    currentSize  int64
}

type AccessStats struct {
    Count      int64
    LastAccess time.Time
    Frequency  float64 // 접근 빈도 (per hour)
}

func (ic *IntelligentCache) Access(cid cid.Cid) ([]byte, error) {
    ctx := context.Background()

    // 1. 접근 통계 업데이트
    ic.updateAccessStats(cid)

    // 2. 캐시에서 데이터 조회
    data, err := ic.pinManager.GetData(ctx, cid)
    if err != nil {
        return nil, err
    }

    // 3. 캐시 최적화 트리거
    go ic.optimizeCache()

    return data, nil
}

func (ic *IntelligentCache) optimizeCache() {
    if ic.currentSize <= ic.maxCacheSize {
        return
    }

    // LFU (Least Frequently Used) 알고리즘으로 제거 대상 선정
    candidates := ic.getLFUCandidates()

    for _, cid := range candidates {
        if ic.currentSize <= ic.maxCacheSize*0.8 { // 80%까지 줄이기
            break
        }

        size := ic.getDataSize(cid)
        ic.pinManager.PinRemove(context.Background(), cid)
        ic.currentSize -= size

        fmt.Printf("Cache evicted: %s (freed %d bytes)\n",
                  cid.String()[:20]+"...", size)
    }
}

func (ic *IntelligentCache) getLFUCandidates() []cid.Cid {
    type cidFreq struct {
        cid  cid.Cid
        freq float64
    }

    var candidates []cidFreq
    for cid, stats := range ic.accessStats {
        candidates = append(candidates, cidFreq{cid: cid, freq: stats.Frequency})
    }

    // 빈도순 정렬 (낮은 빈도부터)
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].freq < candidates[j].freq
    })

    var result []cid.Cid
    for _, candidate := range candidates {
        result = append(result, candidate.cid)
    }

    return result
}
```

## ⚠️ 주의사항 및 모범 사례

### 1. Pin 전략 설계

```go
// ✅ 콘텐츠 타입별 Pin 전략
func selectPinStrategy(contentType string, size int64, importance Priority) PinType {
    switch {
    case importance == Critical:
        return PinTypeRecursive // 중요한 데이터는 완전 보호

    case contentType == "application/json" && size < 1024*1024:
        return PinTypeRecursive // 작은 구조화된 데이터

    case contentType == "image/*" || contentType == "video/*":
        return PinTypeDirect // 미디어 파일은 Direct Pin

    case size > 100*1024*1024:
        return PinTypeDirect // 대용량 파일은 Direct Pin으로 성능 확보

    default:
        return PinTypeDirect
    }
}
```

### 2. GC 타이밍 최적화

```go
// ✅ 시스템 부하를 고려한 GC 스케줄링
type AdaptiveGCScheduler struct {
    *GCScheduler
    cpuThreshold  float64
    memThreshold  float64
    ioThreshold   float64
}

func (agcs *AdaptiveGCScheduler) shouldRunGC(ctx context.Context) bool {
    // 1. 시스템 리소스 상태 확인
    systemStats := agcs.getSystemStats()

    if systemStats.CPUUsage > agcs.cpuThreshold {
        fmt.Printf("Delaying GC: high CPU usage (%.1f%%)\n", systemStats.CPUUsage)
        return false
    }

    if systemStats.MemoryUsage > agcs.memThreshold {
        fmt.Printf("Delaying GC: high memory usage (%.1f%%)\n", systemStats.MemoryUsage)
        return false
    }

    if systemStats.IOWait > agcs.ioThreshold {
        fmt.Printf("Delaying GC: high IO wait (%.1f%%)\n", systemStats.IOWait)
        return false
    }

    // 2. 기본 GC 조건 확인
    return agcs.GCScheduler.shouldRunGC(ctx)
}
```

### 3. Pin 충돌 해결

```go
// ✅ Pin 타입 변경 시 안전한 처리
func (pm *PinManager) ChangePinType(ctx context.Context, c cid.Cid,
                                   fromType, toType PinType) error {
    // 1. 현재 Pin 상태 확인
    isPinned, currentType, err := pm.GetPinStatus(ctx, c)
    if err != nil {
        return err
    }

    if !isPinned {
        return fmt.Errorf("CID %s is not pinned", c)
    }

    if currentType != fromType {
        return fmt.Errorf("expected pin type %v, got %v", fromType, currentType)
    }

    // 2. 원자적 Pin 타입 변경
    tx := pm.beginTransaction()
    defer tx.rollback() // 에러 시 롤백

    // 새로운 타입으로 Pin 추가
    err = pm.PinAdd(ctx, c, toType)
    if err != nil {
        return fmt.Errorf("failed to add new pin type: %w", err)
    }

    // 기존 Pin 제거
    err = pm.PinRemove(ctx, c, fromType)
    if err != nil {
        return fmt.Errorf("failed to remove old pin type: %w", err)
    }

    return tx.commit()
}
```

### 4. 대용량 데이터 GC

```go
// ✅ 메모리 효율적인 대용량 GC
func (gcm *GCManager) RunLargeScaleGC(ctx context.Context) error {
    const batchSize = 1000

    // 1. 스트리밍 방식으로 블록 처리
    blockChan := make(chan cid.Cid, batchSize)
    resultChan := make(chan gcResult, batchSize)

    // 워커 풀 시작
    workers := runtime.NumCPU()
    for i := 0; i < workers; i++ {
        go gcm.gcWorker(ctx, blockChan, resultChan)
    }

    // 2. 배치 단위로 블록 스캔 및 처리
    go func() {
        defer close(blockChan)

        err := gcm.streamAllBlocks(ctx, func(c cid.Cid) {
            blockChan <- c
        })

        if err != nil {
            log.Printf("Error streaming blocks: %v", err)
        }
    }()

    // 3. 결과 수집
    var totalRemoved int64
    var spaceFreed int64

    for result := range resultChan {
        if result.removed {
            totalRemoved++
            spaceFreed += result.size
        }

        // 주기적으로 진행 상황 보고
        if totalRemoved%1000 == 0 {
            fmt.Printf("GC progress: %d blocks removed, %d bytes freed\n",
                      totalRemoved, spaceFreed)
        }
    }

    fmt.Printf("Large-scale GC completed: %d blocks removed, %d bytes freed\n",
              totalRemoved, spaceFreed)

    return nil
}
```

## 🔧 트러블슈팅

### 문제 1: "pin not found" 에러

**원인**: Pin이 이미 제거되었거나 존재하지 않음
```go
// 해결: Pin 상태 확인 후 작업
func safePinRemove(pm *PinManager, ctx context.Context, c cid.Cid) error {
    isPinned, pinType, err := pm.GetPinStatus(ctx, c)
    if err != nil {
        return err
    }

    if !isPinned {
        fmt.Printf("CID %s is not pinned, skipping removal\n", c)
        return nil
    }

    return pm.PinRemove(ctx, c, pinType)
}
```

### 문제 2: GC가 너무 오래 걸림

**원인**: 대용량 데이터베이스 또는 비효율적인 스캔
```go
// 해결: 증분 GC 및 배치 처리
func (gcm *GCManager) RunIncrementalGC(ctx context.Context,
                                      maxDuration time.Duration) error {
    startTime := time.Now()
    processed := 0

    return gcm.streamAllBlocks(ctx, func(c cid.Cid) {
        if time.Since(startTime) > maxDuration {
            fmt.Printf("GC time limit reached, processed %d blocks\n", processed)
            return
        }

        gcm.processBlock(ctx, c)
        processed++
    })
}
```

### 문제 3: 디스크 공간 부족

**원인**: GC가 효과적으로 작동하지 않음
```go
// 해결: 강제 GC 및 임계값 조정
func emergencyGC(pm *PinManager) error {
    ctx := context.Background()

    // 1. 모든 임시 Pin 제거
    tempPins := pm.findTemporaryPins(ctx)
    for _, c := range tempPins {
        pm.PinRemove(ctx, c, PinTypeDirect)
    }

    // 2. 강제 GC 실행
    result, err := pm.RunGC(ctx, GCOptions{
        Force:    true,
        Aggressive: true,
    })

    if err != nil {
        return err
    }

    fmt.Printf("Emergency GC freed %d bytes\n", result.SpaceFreed)
    return nil
}
```

## 📚 추가 학습 자료

### 관련 문서
- [IPFS Pinning](https://docs.ipfs.io/concepts/persistence/)
- [Garbage Collection in IPFS](https://docs.ipfs.io/concepts/lifecycle/)
- [Pin API Reference](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-pin)

### 다음 단계
1. **06-gateway**: HTTP 인터페이스와 웹 통합
2. **07-ipns**: 네이밍 시스템과 동적 콘텐츠
3. **99-kubo-api-demo**: 실제 IPFS 네트워크 연동

## 🍳 쿡북 (Cookbook) - 바로 사용할 수 있는 코드

### 📌 자동 Pin 관리 시스템

```go
package main

import (
    "context"
    "time"

    pin "github.com/gosuda/boxo-starter-kit/05-pin-gc/pkg"
    dag "github.com/gosuda/boxo-starter-kit/02-dag-ipld/pkg"
)

// 자동으로 콘텐츠를 Pin하고 관리하는 시스템
type AutoPinSystem struct {
    pinManager *pin.PinManager
    policies   map[string]*pin.PinPolicy
}

func NewAutoPinSystem(dagWrapper *dag.DagWrapper) *AutoPinSystem {
    pinManager, _ := pin.NewPinManager(dagWrapper)

    return &AutoPinSystem{
        pinManager: pinManager,
        policies:   make(map[string]*pin.PinPolicy),
    }
}

// 콘텐츠 타입별 자동 Pin 정책 설정
func (aps *AutoPinSystem) SetupPolicies() {
    // 이미지 파일: Direct Pin, 30일 보관
    aps.policies["image"] = &pin.PinPolicy{
        PinType:   pin.PinTypeDirect,
        TTL:       30 * 24 * time.Hour,
        MaxSize:   10 * 1024 * 1024, // 10MB 이하만
        AutoPin:   true,
    }

    // 문서 파일: Recursive Pin, 1년 보관
    aps.policies["document"] = &pin.PinPolicy{
        PinType:   pin.PinTypeRecursive,
        TTL:       365 * 24 * time.Hour,
        MaxSize:   100 * 1024 * 1024, // 100MB 이하만
        AutoPin:   true,
    }

    // 임시 파일: Pin하지 않음
    aps.policies["temp"] = &pin.PinPolicy{
        AutoPin: false,
    }
}

// 파일을 추가하고 정책에 따라 자동 Pin
func (aps *AutoPinSystem) AddFile(filename string, data []byte,
                                  contentType string) (cid.Cid, error) {
    ctx := context.Background()

    // 1. 데이터 저장
    c, err := aps.pinManager.AddData(ctx, data)
    if err != nil {
        return cid.Undef, err
    }

    // 2. 정책 확인 및 적용
    if policy, exists := aps.policies[contentType]; exists && policy.AutoPin {
        if int64(len(data)) <= policy.MaxSize {
            err = aps.pinManager.PinAdd(ctx, c, policy.PinType)
            if err != nil {
                return c, err
            }

            fmt.Printf("🔧 Auto-pinned %s as %v (TTL: %v)\n",
                      filename, policy.PinType, policy.TTL)

            // 3. TTL 기반 자동 해제 스케줄링
            aps.scheduleUnpin(c, policy.TTL)
        }
    }

    return c, nil
}

// TTL 후 자동 Pin 해제
func (aps *AutoPinSystem) scheduleUnpin(c cid.Cid, ttl time.Duration) {
    go func() {
        time.Sleep(ttl)

        err := aps.pinManager.PinRemove(context.Background(), c)
        if err != nil {
            fmt.Printf("⚠️ Failed to auto-unpin %s: %v\n", c, err)
        } else {
            fmt.Printf("🗂️ Auto-unpinned %s after TTL\n", c.String()[:20]+"...")
        }
    }()
}
```

### 🧹 스마트 가비지 컬렉터

```go
type SmartGC struct {
    pinManager   *pin.PinManager
    thresholds   *GCThresholds
    isRunning    bool
    stats        *GCStats
}

type GCThresholds struct {
    DiskUsagePercent float64       // 디스크 사용률 임계값
    MaxAge          time.Duration  // 최대 데이터 보관 기간
    MaxBlocks       int64         // 최대 블록 수
    MinFreeSpace    int64         // 최소 여유 공간
}

func NewSmartGC(pinManager *pin.PinManager) *SmartGC {
    return &SmartGC{
        pinManager: pinManager,
        thresholds: &GCThresholds{
            DiskUsagePercent: 80.0,  // 80% 사용 시 GC 트리거
            MaxAge:          7 * 24 * time.Hour, // 7일 후 GC 대상
            MaxBlocks:       100000, // 10만 블록 초과 시 GC
            MinFreeSpace:    1024 * 1024 * 1024, // 1GB 여유 공간 유지
        },
        stats: &GCStats{},
    }
}

// 스마트 GC 시작 (시스템 상태 기반 자동 조정)
func (sgc *SmartGC) Start(ctx context.Context) {
    if sgc.isRunning {
        return
    }

    sgc.isRunning = true
    fmt.Printf("🧠 Smart GC started with adaptive thresholds\n")

    go func() {
        ticker := time.NewTicker(5 * time.Minute) // 5분마다 상태 체크
        defer ticker.Stop()

        for sgc.isRunning {
            select {
            case <-ticker.C:
                sgc.checkAndRunGC(ctx)

            case <-ctx.Done():
                sgc.isRunning = false
                return
            }
        }
    }()
}

// 시스템 상태를 분석하여 GC 필요성 판단
func (sgc *SmartGC) checkAndRunGC(ctx context.Context) {
    systemStats := sgc.getSystemStats()
    urgency := sgc.calculateUrgency(systemStats)

    switch urgency {
    case UrgencyHigh:
        fmt.Printf("🚨 High urgency GC triggered\n")
        sgc.runAggressiveGC(ctx)

    case UrgencyMedium:
        fmt.Printf("⚠️ Medium urgency GC triggered\n")
        sgc.runNormalGC(ctx)

    case UrgencyLow:
        fmt.Printf("💡 Low urgency GC triggered\n")
        sgc.runGentleGC(ctx)

    case UrgencyNone:
        // GC 필요 없음
        return
    }

    sgc.updateStats()
}

// 시급도 계산 (여러 지표 종합)
func (sgc *SmartGC) calculateUrgency(stats SystemStats) Urgency {
    score := 0.0

    // 디스크 사용률 기반 점수
    if stats.DiskUsagePercent > 90 {
        score += 50
    } else if stats.DiskUsagePercent > sgc.thresholds.DiskUsagePercent {
        score += 30
    }

    // 블록 수 기반 점수
    if stats.TotalBlocks > sgc.thresholds.MaxBlocks*2 {
        score += 30
    } else if stats.TotalBlocks > sgc.thresholds.MaxBlocks {
        score += 15
    }

    // 메모리 사용률 기반 점수
    if stats.MemoryUsagePercent > 85 {
        score += 20
    }

    // 마지막 GC 이후 시간 기반 점수
    if time.Since(sgc.stats.LastGC) > 24*time.Hour {
        score += 10
    }

    switch {
    case score >= 70:
        return UrgencyHigh
    case score >= 40:
        return UrgencyMedium
    case score >= 20:
        return UrgencyLow
    default:
        return UrgencyNone
    }
}

// 단계별 GC 실행 (시급도에 따른 다른 전략)
func (sgc *SmartGC) runAggressiveGC(ctx context.Context) {
    // 1. 모든 임시 데이터 제거
    sgc.removeTemporaryData(ctx)

    // 2. 오래된 캐시 데이터 제거
    sgc.removeOldCache(ctx, 1*time.Hour)

    // 3. 강제 GC 실행
    result, err := sgc.pinManager.RunGC(ctx, pin.GCOptions{
        Force:      true,
        Aggressive: true,
        MaxDuration: 10 * time.Minute,
    })

    if err != nil {
        fmt.Printf("❌ Aggressive GC failed: %v\n", err)
    } else {
        fmt.Printf("✅ Aggressive GC freed %d bytes\n", result.SpaceFreed)
    }
}

func (sgc *SmartGC) runNormalGC(ctx context.Context) {
    // 표준 GC + 선택적 정리
    sgc.removeOldCache(ctx, 6*time.Hour)

    result, err := sgc.pinManager.RunGC(ctx, pin.GCOptions{
        MaxDuration: 5 * time.Minute,
    })

    if err == nil {
        fmt.Printf("✅ Normal GC freed %d bytes\n", result.SpaceFreed)
    }
}

func (sgc *SmartGC) runGentleGC(ctx context.Context) {
    // 부드러운 GC (시스템 부하 최소화)
    result, err := sgc.pinManager.RunGC(ctx, pin.GCOptions{
        Gentle:      true,
        MaxDuration: 2 * time.Minute,
    })

    if err == nil {
        fmt.Printf("✅ Gentle GC freed %d bytes\n", result.SpaceFreed)
    }
}
```

### 📊 Pin 상태 대시보드

```go
type PinDashboard struct {
    pinManager *pin.PinManager
    httpServer *http.Server
    updateChan chan bool
}

func NewPinDashboard(pinManager *pin.PinManager, port int) *PinDashboard {
    dashboard := &PinDashboard{
        pinManager: pinManager,
        updateChan: make(chan bool, 10),
    }

    // HTTP 서버 설정
    mux := http.NewServeMux()
    mux.HandleFunc("/", dashboard.handleHome)
    mux.HandleFunc("/api/stats", dashboard.handleStats)
    mux.HandleFunc("/api/pins", dashboard.handlePins)
    mux.HandleFunc("/api/gc", dashboard.handleGC)

    dashboard.httpServer = &http.Server{
        Addr:    fmt.Sprintf(":%d", port),
        Handler: mux,
    }

    return dashboard
}

// 웹 대시보드 시작
func (pd *PinDashboard) Start() error {
    fmt.Printf("📊 Pin Dashboard starting on http://localhost%s\n",
              pd.httpServer.Addr)

    go pd.updateLoop() // 백그라운드 업데이트
    return pd.httpServer.ListenAndServe()
}

// 실시간 통계 업데이트
func (pd *PinDashboard) updateLoop() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            pd.updateChan <- true
        }
    }
}

// 홈페이지 (대시보드 UI)
func (pd *PinDashboard) handleHome(w http.ResponseWriter, r *http.Request) {
    html := `
<!DOCTYPE html>
<html>
<head>
    <title>IPFS Pin Dashboard</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, sans-serif; margin: 20px; }
        .stat-box { background: #f5f5f5; padding: 15px; margin: 10px 0; border-radius: 8px; }
        .pin-list { max-height: 400px; overflow-y: auto; }
        .pin-item { padding: 8px; border-bottom: 1px solid #eee; }
        .direct { color: #007bff; }
        .recursive { color: #28a745; }
        .indirect { color: #6c757d; }
        button { background: #007bff; color: white; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer; }
        button:hover { background: #0056b3; }
        .refresh { float: right; }
    </style>
</head>
<body>
    <h1>📌 IPFS Pin Management Dashboard</h1>

    <button class="refresh" onclick="location.reload()">🔄 Refresh</button>

    <div id="stats"></div>
    <div id="pins"></div>

    <div style="margin-top: 20px;">
        <button onclick="runGC()">🗑️ Run Garbage Collection</button>
        <span id="gc-status"></span>
    </div>

    <script>
        async function loadStats() {
            const response = await fetch('/api/stats');
            const stats = await response.json();

            document.getElementById('stats').innerHTML = \`
                <div class="stat-box">
                    <h3>📊 Pin Statistics</h3>
                    <p><strong>Total Pins:</strong> \${stats.total_pins}</p>
                    <p><strong>Direct Pins:</strong> <span class="direct">\${stats.direct_pins}</span></p>
                    <p><strong>Recursive Pins:</strong> <span class="recursive">\${stats.recursive_pins}</span></p>
                    <p><strong>Indirect Pins:</strong> <span class="indirect">\${stats.indirect_pins}</span></p>
                    <p><strong>Last GC:</strong> \${stats.last_gc || 'Never'}</p>
                    <p><strong>Space Reclaimed:</strong> \${formatBytes(stats.space_reclaimed)}</p>
                </div>
            \`;
        }

        async function loadPins() {
            const response = await fetch('/api/pins');
            const pins = await response.json();

            let html = '<div class="stat-box"><h3>📋 Pin List</h3><div class="pin-list">';

            pins.forEach(pin => {
                html += \`
                    <div class="pin-item">
                        <span class="\${pin.type}">\${pin.type}</span>
                        <code>\${pin.cid.substring(0, 20)}...</code>
                        <small>(\${formatBytes(pin.size)})</small>
                    </div>
                \`;
            });

            html += '</div></div>';
            document.getElementById('pins').innerHTML = html;
        }

        async function runGC() {
            document.getElementById('gc-status').innerHTML = '⏳ Running GC...';

            try {
                const response = await fetch('/api/gc', { method: 'POST' });
                const result = await response.json();

                document.getElementById('gc-status').innerHTML =
                    \`✅ GC completed: \${result.removed_count} blocks removed, \${formatBytes(result.space_freed)} freed\`;

                // 통계 새로고침
                setTimeout(() => {
                    loadStats();
                    loadPins();
                }, 1000);

            } catch (error) {
                document.getElementById('gc-status').innerHTML = '❌ GC failed: ' + error.message;
            }
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        // 초기 로드
        loadStats();
        loadPins();

        // 자동 새로고침 (30초마다)
        setInterval(() => {
            loadStats();
            loadPins();
        }, 30000);
    </script>
</body>
</html>`

    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(html))
}

// 통계 API
func (pd *PinDashboard) handleStats(w http.ResponseWriter, r *http.Request) {
    stats := pd.pinManager.GetStats()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

// Pin 목록 API
func (pd *PinDashboard) handlePins(w http.ResponseWriter, r *http.Request) {
    pins, err := pd.pinManager.ListPins(r.Context(), pin.PinTypeAll)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(pins)
}

// GC 실행 API
func (pd *PinDashboard) handleGC(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    result, err := pd.pinManager.RunGC(r.Context(), pin.GCOptions{})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    response := map[string]interface{}{
        "removed_count": result.TotalRemoved,
        "space_freed":   result.SpaceFreed,
        "duration":      result.Duration.Seconds(),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

### 🔄 백업 및 복원 시스템

```go
type BackupSystem struct {
    pinManager   *pin.PinManager
    backupPath   string
    compression  bool
    encryption   bool
}

func NewBackupSystem(pinManager *pin.PinManager, backupPath string) *BackupSystem {
    return &BackupSystem{
        pinManager:  pinManager,
        backupPath:  backupPath,
        compression: true,
        encryption:  false, // 필요시 활성화
    }
}

// 전체 Pin 데이터를 백업
func (bs *BackupSystem) BackupAll(ctx context.Context) (*BackupManifest, error) {
    fmt.Printf("💾 Starting full backup to %s\n", bs.backupPath)

    manifest := &BackupManifest{
        Timestamp: time.Now(),
        Version:   "1.0",
        Pins:      make([]BackupPin, 0),
    }

    // 1. 모든 Pin 조회
    allPins, err := bs.pinManager.ListPins(ctx, pin.PinTypeAll)
    if err != nil {
        return nil, fmt.Errorf("failed to list pins: %w", err)
    }

    totalPins := len(allPins)
    fmt.Printf("📋 Found %d pins to backup\n", totalPins)

    // 2. 각 Pin된 데이터 백업
    for i, pinInfo := range allPins {
        data, err := bs.pinManager.GetData(ctx, pinInfo.CID)
        if err != nil {
            fmt.Printf("⚠️ Failed to get data for %s: %v\n", pinInfo.CID, err)
            continue
        }

        // 백업 파일 생성
        backupPin, err := bs.backupSinglePin(pinInfo, data)
        if err != nil {
            fmt.Printf("⚠️ Failed to backup %s: %v\n", pinInfo.CID, err)
            continue
        }

        manifest.Pins = append(manifest.Pins, *backupPin)

        // 진행률 표시
        if (i+1)%100 == 0 || i+1 == totalPins {
            fmt.Printf("📦 Backup progress: %d/%d pins (%.1f%%)\n",
                      i+1, totalPins, float64(i+1)/float64(totalPins)*100)
        }
    }

    // 3. 매니페스트 저장
    manifestPath := filepath.Join(bs.backupPath, "manifest.json")
    manifestData, _ := json.MarshalIndent(manifest, "", "  ")

    err = os.WriteFile(manifestPath, manifestData, 0644)
    if err != nil {
        return nil, fmt.Errorf("failed to save manifest: %w", err)
    }

    fmt.Printf("✅ Backup completed: %d pins backed up\n", len(manifest.Pins))
    return manifest, nil
}

// 백업에서 복원
func (bs *BackupSystem) RestoreAll(ctx context.Context, manifestPath string) error {
    fmt.Printf("📥 Starting restore from %s\n", manifestPath)

    // 1. 매니페스트 로드
    manifestData, err := os.ReadFile(manifestPath)
    if err != nil {
        return fmt.Errorf("failed to read manifest: %w", err)
    }

    var manifest BackupManifest
    err = json.Unmarshal(manifestData, &manifest)
    if err != nil {
        return fmt.Errorf("failed to parse manifest: %w", err)
    }

    fmt.Printf("📋 Found %d pins in backup (created: %s)\n",
              len(manifest.Pins), manifest.Timestamp.Format("2006-01-02 15:04:05"))

    // 2. 각 Pin 복원
    restored := 0
    for i, backupPin := range manifest.Pins {
        err := bs.restoreSinglePin(ctx, backupPin)
        if err != nil {
            fmt.Printf("⚠️ Failed to restore %s: %v\n", backupPin.CID, err)
            continue
        }

        restored++

        // 진행률 표시
        if (i+1)%100 == 0 || i+1 == len(manifest.Pins) {
            fmt.Printf("📦 Restore progress: %d/%d pins (%.1f%%)\n",
                      i+1, len(manifest.Pins), float64(i+1)/float64(len(manifest.Pins))*100)
        }
    }

    fmt.Printf("✅ Restore completed: %d/%d pins restored\n",
              restored, len(manifest.Pins))

    return nil
}

// 개별 Pin 백업
func (bs *BackupSystem) backupSinglePin(pinInfo pin.PinInfo, data []byte) (*BackupPin, error) {
    // 압축 처리
    if bs.compression {
        compressed, err := bs.compressData(data)
        if err == nil && len(compressed) < len(data) {
            data = compressed
        }
    }

    // 백업 파일 저장
    filename := fmt.Sprintf("%s.dat", pinInfo.CID.String())
    filePath := filepath.Join(bs.backupPath, filename)

    err := os.WriteFile(filePath, data, 0644)
    if err != nil {
        return nil, err
    }

    return &BackupPin{
        CID:        pinInfo.CID.String(),
        Type:       pinInfo.Type.String(),
        Size:       pinInfo.Size,
        BackupPath: filename,
        Compressed: bs.compression,
    }, nil
}

// 개별 Pin 복원
func (bs *BackupSystem) restoreSinglePin(ctx context.Context, backupPin BackupPin) error {
    // 백업 파일 읽기
    filePath := filepath.Join(bs.backupPath, backupPin.BackupPath)
    data, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("failed to read backup file: %w", err)
    }

    // 압축 해제
    if backupPin.Compressed {
        decompressed, err := bs.decompressData(data)
        if err != nil {
            return fmt.Errorf("failed to decompress: %w", err)
        }
        data = decompressed
    }

    // 데이터 복원 및 Pin
    c, err := bs.pinManager.AddData(ctx, data)
    if err != nil {
        return fmt.Errorf("failed to add data: %w", err)
    }

    // Pin 타입에 따라 Pin 설정
    pinType, _ := pin.ParsePinType(backupPin.Type)
    err = bs.pinManager.PinAdd(ctx, c, pinType)
    if err != nil {
        return fmt.Errorf("failed to pin: %w", err)
    }

    return nil
}

// 사용 예제
func ExampleBackupRestore() {
    // 백업 시스템 초기화
    dagWrapper, _ := dag.New(nil, "")
    pinManager, _ := pin.NewPinManager(dagWrapper)
    backupSystem := NewBackupSystem(pinManager, "/backup/ipfs")

    ctx := context.Background()

    // 전체 백업 실행
    manifest, err := backupSystem.BackupAll(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("백업 완료: %d개 Pin이 백업됨\n", len(manifest.Pins))

    // 복원 (필요시)
    // err = backupSystem.RestoreAll(ctx, "/backup/ipfs/manifest.json")
}
```

이제 Pin 관리와 가비지 컬렉션의 모든 측면을 완벽하게 다룰 수 있습니다! 🚀