# 05-pin-gc: Data Lifecycle Management and Garbage Collection

## 🎯 Learning Objectives

Through this module, you will learn:
- The concept of **Pin** and methods to ensure data persistence
- Principles of **Garbage Collection (GC)** and disk space management
- Types of **Pin types** and their usage scenarios
- **Data lifecycle** management strategies and automation
- **Storage optimization** techniques and performance tuning
- **Pin policy** design and practical implementation approaches

## 📋 Prerequisites

- **00-block-cid** module completed (understanding Blocks and CIDs)
- **01-persistent** module completed (understanding data persistence)
- **05-dag-ipld** module completed (understanding DAG and connected data)
- **06-unixfs-car** module completed (understanding file systems and chunking)
- Basic concepts of memory management and garbage collection

## 🔑 Core Concepts

### What is a Pin?

A **Pin** is a mechanism in IPFS to protect specific data from being garbage collected:

```
Regular data:   [Block] ← Can be deleted by GC
Pinned data:    [Block] + 📌 ← Protected from GC
```

### Pin Types

| Type | Description | Use Cases |
|------|-------------|-----------|
| **Direct** | Pin specific block only | Single files, small data |
| **Recursive** | Pin all connected blocks | Directories, complex structures |
| **Indirect** | Dependencies of other pins | Automatic management, internal references |

### Garbage Collection Process

```
1. Scan: Examine all blocks
2. Mark: Mark pinned blocks and dependencies
3. Sweep: Delete unmarked blocks
4. Stats: Report reclaimed space
```

### Data Lifecycle

```
Create → Use → Pin → Preserve → Unpin → GC candidate → Delete
    ↑                              ↓
    └─────── Re-pin (optional) ←────┘
```

## 💻 Code Analysis

### 1. Pin Manager Design

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
    // Initialize Pinner interface
    pinner := pin.NewPinner(dagWrapper.GetDatastore(), dagWrapper.GetDAGService(), dagWrapper.GetDAGService())

    return &PinManager{
        dagWrapper: dagWrapper,
        pinner:     pinner,
        gcManager:  NewGCManager(dagWrapper),
    }, nil
}
```

**Design Features**:
- Standard Pin management using **pin.Pinner** interface
- Separation of concerns with **GCManager**
- State monitoring with **statistics collection**
- **Concurrency-safe** Pin operations

### 2. Adding Pins (Various Types)

```go
// pkg/pin.go:65-95
func (pm *PinManager) PinAdd(ctx context.Context, c cid.Cid, pinType PinType) error {
    pm.stats.mutex.Lock()
    defer pm.stats.mutex.Unlock()

    switch pinType {
    case PinTypeDirect:
        // 1. Direct Pin: Pin only specific block
        err := pm.pinner.Pin(ctx, c, false)
        if err != nil {
            return fmt.Errorf("failed to pin direct: %w", err)
        }
        pm.stats.DirectPins++
        fmt.Printf("   📌 Direct pin added: %s\n", c.String()[:20]+"...")

    case PinTypeRecursive:
        // 2. Recursive Pin: Pin all connected blocks
        err := pm.pinner.Pin(ctx, c, true)
        if err != nil {
            return fmt.Errorf("failed to pin recursive: %w", err)
        }
        pm.stats.RecursivePins++
        fmt.Printf("   🔗 Recursive pin added: %s\n", c.String()[:20]+"...")

        // Calculate number of linked blocks
        linkedBlocks := pm.countLinkedBlocks(ctx, c)
        fmt.Printf("      Protecting %d linked blocks\n", linkedBlocks)

    default:
        return fmt.Errorf("unsupported pin type: %v", pinType)
    }

    pm.stats.TotalPins++
    return pm.pinner.Flush(ctx)
}
```

### 3. Querying Pin Status

```go
// pkg/pin.go:130-165
func (pm *PinManager) ListPins(ctx context.Context, pinType PinType) ([]PinInfo, error) {
    var pins []PinInfo

    // 1. Query by pin type
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
        // 2. Query all pin types
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

### 4. Garbage Collection Implementation

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

    // 1. Initialize GC statistics
    result := &GCResult{
        StartTime:    startTime,
        RemovedCIDs:  make([]cid.Cid, 0),
        TotalRemoved: 0,
        SpaceFreed:   0,
    }

    // 2. Scan all blocks
    allBlocks, err := gcm.getAllBlocks(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get all blocks: %w", err)
    }

    fmt.Printf("      Scanning %d total blocks...\n", len(allBlocks))

    // 3. Mark pinned blocks and dependencies
    pinnedBlocks := make(map[cid.Cid]bool)
    err = gcm.markPinnedBlocks(ctx, pinnedBlocks)
    if err != nil {
        return nil, fmt.Errorf("failed to mark pinned blocks: %w", err)
    }

    fmt.Printf("      %d blocks are pinned (protected)\n", len(pinnedBlocks))

    // 4. Identify and delete garbage blocks
    for _, blockCID := range allBlocks {
        if !pinnedBlocks[blockCID] {
            blockSize := gcm.getBlockSize(ctx, blockCID)

            // Execute deletion
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

    // 5. Finalize results
    result.EndTime = time.Now()
    result.Duration = result.EndTime.Sub(result.StartTime)
    result.TotalRemoved = len(result.RemovedCIDs)

    fmt.Printf("   ✅ GC completed in %v\n", result.Duration)
    fmt.Printf("      Removed %d blocks, freed %d bytes\n",
              result.TotalRemoved, result.SpaceFreed)

    return result, nil
}
```

### 5. Automatic GC Scheduling

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
                // 1. Check GC execution conditions
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
    // 2. Check GC execution conditions
    stats := gcs.pinManager.GetStats()

    // Check disk usage
    if stats.DiskUsage > gcs.threshold.MaxDiskUsage {
        return true
    }

    // Check time-based criteria
    if time.Since(gcs.lastGC) > gcs.threshold.MaxAge {
        return true
    }

    // Check block count-based criteria
    if stats.TotalBlocks > gcs.threshold.MaxBlocks {
        return true
    }

    return false
}
```

### 6. Pin Policy Management

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
    // 1. Evaluate rules
    for _, rule := range pp.rules {
        if pp.matchesRule(c, metadata, rule) {
            fmt.Printf("   📋 Pin rule matched: %s -> %v\n", rule.Pattern, rule.PinType)
            return true, rule.PinType
        }
    }

    // 2. Default policy
    if pp.autoPin {
        // Automatic pin decision based on size
        if metadata.Size < pp.maxPinSize {
            return true, PinTypeDirect
        }
    }

    return false, PinTypeDirect
}

func (pp *PinPolicy) matchesRule(c cid.Cid, metadata *ContentMetadata, rule PinRule) bool {
    // 3. Pattern matching
    if rule.Pattern != "" {
        if strings.HasPrefix(c.String(), rule.Pattern) {
            return true
        }
    }

    // 4. Condition evaluation
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

## 🏃‍♂️ Hands-on Guide

### 1. Basic Execution

```bash
cd 05-pin-gc
go run main.go
```

**Expected Output**:
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

### 2. Pin Type Comparison Experiment

```bash
# Verify behavior of different pin types
PIN_DEMO_MODE=comparison go run main.go
```

**Observation Points**:
- **Direct Pin**: Protects specific blocks only, fast pin/unpin
- **Recursive Pin**: Protects all connected blocks, complete preservation
- **Indirect Pin**: Automatic management, maintains reference relationships

### 3. GC Performance Benchmark

```bash
# Measure GC performance with large data
GC_BENCHMARK=true go run main.go
```

### 4. Running Tests

```bash
go test -v ./...
```

**Key Test Cases**:
- ✅ Pin add/remove functionality
- ✅ Various pin type handling
- ✅ GC accuracy verification
- ✅ Scheduling and automation
- ✅ Pin policy application

## 🔍 Advanced Use Cases

### 1. Distributed Backup System

```go
type DistributedBackupManager struct {
    pinManager *PinManager
    replicas   int
    backupPolicy *BackupPolicy
}

func (dbm *DistributedBackupManager) BackupWithPolicy(data []byte,
                                                     policy BackupPolicy) error {
    ctx := context.Background()

    // 1. Determine pin type based on backup policy
    pinType := policy.DeterminePinType(len(data))

    // 2. Pin original data
    originalCID, err := dbm.pinManager.PinData(ctx, data, pinType)
    if err != nil {
        return fmt.Errorf("failed to pin original: %w", err)
    }

    // 3. Create and pin replicas
    for i := 0; i < dbm.replicas; i++ {
        replicaData := dbm.createReplica(data, i)
        replicaCID, err := dbm.pinManager.PinData(ctx, replicaData, PinTypeDirect)
        if err != nil {
            log.Printf("Failed to create replica %d: %v", i, err)
            continue
        }

        fmt.Printf("Created replica %d: %s\n", i, replicaCID.String()[:20]+"...")
    }

    // 4. Store backup metadata
    metadata := BackupMetadata{
        OriginalCID: originalCID,
        CreatedAt:   time.Now(),
        Policy:      policy,
        Replicas:    dbm.replicas,
    }

    return dbm.storeBackupMetadata(metadata)
}
```

### 2. Content Lifecycle Management

```go
type ContentLifecycleManager struct {
    pinManager *PinManager
    policies   map[string]*LifecyclePolicy
    scheduler  *time.Ticker
}

type LifecyclePolicy struct {
    HotPeriod    time.Duration // Frequently accessed period
    WarmPeriod   time.Duration // Occasionally accessed period
    ColdPeriod   time.Duration // Rarely accessed period
    ArchivePeriod time.Duration // Archive period
}

func (clm *ContentLifecycleManager) ManageContent(cid cid.Cid,
                                                 metadata ContentMetadata) error {
    policy := clm.policies[metadata.ContentType]
    age := time.Since(metadata.CreatedAt)

    switch {
    case age < policy.HotPeriod:
        // HOT: Complete protection with Recursive Pin
        return clm.pinManager.PinAdd(ctx, cid, PinTypeRecursive)

    case age < policy.WarmPeriod:
        // WARM: Basic protection with Direct Pin
        return clm.pinManager.PinAdd(ctx, cid, PinTypeDirect)

    case age < policy.ColdPeriod:
        // COLD: Unpin, compressed storage
        clm.pinManager.PinRemove(ctx, cid)
        return clm.compressAndStore(cid, metadata)

    default:
        // ARCHIVE: Move to external storage
        return clm.archiveToExternalStorage(cid, metadata)
    }
}
```

### 3. Intelligent Cache Management

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
    Frequency  float64 // Access frequency (per hour)
}

func (ic *IntelligentCache) Access(cid cid.Cid) ([]byte, error) {
    ctx := context.Background()

    // 1. Update access statistics
    ic.updateAccessStats(cid)

    // 2. Retrieve data from cache
    data, err := ic.pinManager.GetData(ctx, cid)
    if err != nil {
        return nil, err
    }

    // 3. Trigger cache optimization
    go ic.optimizeCache()

    return data, nil
}

func (ic *IntelligentCache) optimizeCache() {
    if ic.currentSize <= ic.maxCacheSize {
        return
    }

    // Select removal candidates using LFU (Least Frequently Used) algorithm
    candidates := ic.getLFUCandidates()

    for _, cid := range candidates {
        if ic.currentSize <= ic.maxCacheSize*0.8 { // Reduce to 80%
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

    // Sort by frequency (lowest frequency first)
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

## ⚠️ Best Practices and Considerations

### 1. Pin Strategy Design

```go
// ✅ Pin strategies by content type
func selectPinStrategy(contentType string, size int64, importance Priority) PinType {
    switch {
    case importance == Critical:
        return PinTypeRecursive // Complete protection for critical data

    case contentType == "application/json" && size < 1024*1024:
        return PinTypeRecursive // Small structured data

    case contentType == "image/*" || contentType == "video/*":
        return PinTypeDirect // Media files with Direct Pin

    case size > 100*1024*1024:
        return PinTypeDirect // Direct Pin for large files for performance

    default:
        return PinTypeDirect
    }
}
```

### 2. GC Timing Optimization

```go
// ✅ GC scheduling considering system load
type AdaptiveGCScheduler struct {
    *GCScheduler
    cpuThreshold  float64
    memThreshold  float64
    ioThreshold   float64
}

func (agcs *AdaptiveGCScheduler) shouldRunGC(ctx context.Context) bool {
    // 1. Check system resource status
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

    // 2. Check basic GC conditions
    return agcs.GCScheduler.shouldRunGC(ctx)
}
```

### 3. Pin Conflict Resolution

```go
// ✅ Safe handling when changing pin types
func (pm *PinManager) ChangePinType(ctx context.Context, c cid.Cid,
                                   fromType, toType PinType) error {
    // 1. Check current pin status
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

    // 2. Atomic pin type change
    tx := pm.beginTransaction()
    defer tx.rollback() // Rollback on error

    // Add new pin type
    err = pm.PinAdd(ctx, c, toType)
    if err != nil {
        return fmt.Errorf("failed to add new pin type: %w", err)
    }

    // Remove old pin
    err = pm.PinRemove(ctx, c, fromType)
    if err != nil {
        return fmt.Errorf("failed to remove old pin type: %w", err)
    }

    return tx.commit()
}
```

### 4. Large-scale Data GC

```go
// ✅ Memory-efficient large-scale GC
func (gcm *GCManager) RunLargeScaleGC(ctx context.Context) error {
    const batchSize = 1000

    // 1. Process blocks in streaming fashion
    blockChan := make(chan cid.Cid, batchSize)
    resultChan := make(chan gcResult, batchSize)

    // Start worker pool
    workers := runtime.NumCPU()
    for i := 0; i < workers; i++ {
        go gcm.gcWorker(ctx, blockChan, resultChan)
    }

    // 2. Scan and process blocks in batches
    go func() {
        defer close(blockChan)

        err := gcm.streamAllBlocks(ctx, func(c cid.Cid) {
            blockChan <- c
        })

        if err != nil {
            log.Printf("Error streaming blocks: %v", err)
        }
    }()

    // 3. Collect results
    var totalRemoved int64
    var spaceFreed int64

    for result := range resultChan {
        if result.removed {
            totalRemoved++
            spaceFreed += result.size
        }

        // Periodic progress reporting
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

## 🔧 Troubleshooting

### Issue 1: "pin not found" error

**Cause**: Pin has already been removed or doesn't exist
```go
// Solution: Check pin status before operation
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

### Issue 2: GC takes too long

**Cause**: Large database or inefficient scanning
```go
// Solution: Incremental GC and batch processing
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

### Issue 3: Insufficient disk space

**Cause**: GC not working effectively
```go
// Solution: Force GC and adjust thresholds
func emergencyGC(pm *PinManager) error {
    ctx := context.Background()

    // 1. Remove all temporary pins
    tempPins := pm.findTemporaryPins(ctx)
    for _, c := range tempPins {
        pm.PinRemove(ctx, c, PinTypeDirect)
    }

    // 2. Execute forced GC
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

## 📚 Additional Learning Resources

### Related Documentation
- [IPFS Pinning](https://docs.ipfs.io/concepts/persistence/)
- [Garbage Collection in IPFS](https://docs.ipfs.io/concepts/lifecycle/)
- [Pin API Reference](https://docs.ipfs.io/reference/kubo/rpc/#api-v0-pin)

## 📚 Next Steps

### Immediate Next Steps
1. **[09-ipns](../09-ipns)**: Learn mutable naming system for persistent content
   - **Connection**: Use IPNS to create stable names for pinned content
   - **Why Next**: Provide persistent access points to content managed by pin/GC
   - **Learning Focus**: Mutable pointers and content resolution

2. **[10-gateway](../10-gateway)**: Serve pinned content via HTTP gateway
   - **Connection**: Provide web access to carefully managed content
   - **Why Important**: Bridge between content management and web accessibility
   - **Learning Focus**: HTTP interfaces for content-addressed storage

### Related Modules
3. **[11-kubo-api-demo](../11-kubo-api-demo)**: Complete IPFS network integration
   - **Connection**: Apply pin/GC strategies in real IPFS network context
   - **When to Learn**: For production-ready content management systems

4. **[07-mfs](../07-mfs)**: Pin management for mutable file systems
   - **Connection**: Understand how pin/GC applies to dynamic file system content
   - **Relevance**: Content lifecycle in mutable environments

5. **[17-ipni](../17-ipni)**: Content indexing and discovery for pinned content
   - **Connection**: Enhanced discovery of pinned content through network indexing
   - **Advanced Use**: Large-scale content management and discovery

### Alternative Learning Paths
- **For Web Integration**: Jump to **[10-gateway](../10-gateway)** for immediate HTTP content serving
- **For Naming Systems**: Go to **[09-ipns](../09-ipns)** to learn persistent content addressing
- **For Production Use**: Skip to **[11-kubo-api-demo](../11-kubo-api-demo)** for complete IPFS integration
- **For Advanced Storage**: Explore **[17-ipni](../17-ipni)** for enhanced content indexing

## 🍳 Cookbook - Ready-to-use Code

### 📌 Auto Pin Management System

```go
package main

import (
    "context"
    "time"

    pin "github.com/gosuda/boxo-starter-kit/05-pin-gc/pkg"
    dag "github.com/gosuda/boxo-starter-kit/05-dag-ipld/pkg"
)

// System that automatically pins and manages content
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

// Set auto pin policies by content type
func (aps *AutoPinSystem) SetupPolicies() {
    // Image files: Direct Pin, 30-day retention
    aps.policies["image"] = &pin.PinPolicy{
        PinType:   pin.PinTypeDirect,
        TTL:       30 * 24 * time.Hour,
        MaxSize:   10 * 1024 * 1024, // Only under 10MB
        AutoPin:   true,
    }

    // Document files: Recursive Pin, 1-year retention
    aps.policies["document"] = &pin.PinPolicy{
        PinType:   pin.PinTypeRecursive,
        TTL:       365 * 24 * time.Hour,
        MaxSize:   100 * 1024 * 1024, // Only under 100MB
        AutoPin:   true,
    }

    // Temporary files: Don't pin
    aps.policies["temp"] = &pin.PinPolicy{
        AutoPin: false,
    }
}

// Add file and auto-pin according to policy
func (aps *AutoPinSystem) AddFile(filename string, data []byte,
                                  contentType string) (cid.Cid, error) {
    ctx := context.Background()

    // 1. Store data
    c, err := aps.pinManager.AddData(ctx, data)
    if err != nil {
        return cid.Undef, err
    }

    // 2. Check and apply policy
    if policy, exists := aps.policies[contentType]; exists && policy.AutoPin {
        if int64(len(data)) <= policy.MaxSize {
            err = aps.pinManager.PinAdd(ctx, c, policy.PinType)
            if err != nil {
                return c, err
            }

            fmt.Printf("🔧 Auto-pinned %s as %v (TTL: %v)\n",
                      filename, policy.PinType, policy.TTL)

            // 3. Schedule TTL-based auto-unpin
            aps.scheduleUnpin(c, policy.TTL)
        }
    }

    return c, nil
}

// Auto unpin after TTL
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

### 🧹 Smart Garbage Collector

```go
type SmartGC struct {
    pinManager   *pin.PinManager
    thresholds   *GCThresholds
    isRunning    bool
    stats        *GCStats
}

type GCThresholds struct {
    DiskUsagePercent float64       // Disk usage threshold
    MaxAge          time.Duration  // Maximum data retention period
    MaxBlocks       int64         // Maximum number of blocks
    MinFreeSpace    int64         // Minimum free space
}

func NewSmartGC(pinManager *pin.PinManager) *SmartGC {
    return &SmartGC{
        pinManager: pinManager,
        thresholds: &GCThresholds{
            DiskUsagePercent: 80.0,  // Trigger GC at 80% usage
            MaxAge:          7 * 24 * time.Hour, // GC candidate after 7 days
            MaxBlocks:       100000, // GC when exceeding 100k blocks
            MinFreeSpace:    1024 * 1024 * 1024, // Maintain 1GB free space
        },
        stats: &GCStats{},
    }
}

// Start smart GC (automatic adjustment based on system state)
func (sgc *SmartGC) Start(ctx context.Context) {
    if sgc.isRunning {
        return
    }

    sgc.isRunning = true
    fmt.Printf("🧠 Smart GC started with adaptive thresholds\n")

    go func() {
        ticker := time.NewTicker(5 * time.Minute) // Check status every 5 minutes
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

// Analyze system state to determine GC necessity
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
        // No GC needed
        return
    }

    sgc.updateStats()
}

// Calculate urgency (comprehensive assessment of multiple metrics)
func (sgc *SmartGC) calculateUrgency(stats SystemStats) Urgency {
    score := 0.0

    // Score based on disk usage
    if stats.DiskUsagePercent > 90 {
        score += 50
    } else if stats.DiskUsagePercent > sgc.thresholds.DiskUsagePercent {
        score += 30
    }

    // Score based on block count
    if stats.TotalBlocks > sgc.thresholds.MaxBlocks*2 {
        score += 30
    } else if stats.TotalBlocks > sgc.thresholds.MaxBlocks {
        score += 15
    }

    // Score based on memory usage
    if stats.MemoryUsagePercent > 85 {
        score += 20
    }

    // Score based on time since last GC
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

// Tiered GC execution (different strategies based on urgency)
func (sgc *SmartGC) runAggressiveGC(ctx context.Context) {
    // 1. Remove all temporary data
    sgc.removeTemporaryData(ctx)

    // 2. Remove old cache data
    sgc.removeOldCache(ctx, 1*time.Hour)

    // 3. Execute forced GC
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
    // Standard GC + selective cleanup
    sgc.removeOldCache(ctx, 6*time.Hour)

    result, err := sgc.pinManager.RunGC(ctx, pin.GCOptions{
        MaxDuration: 5 * time.Minute,
    })

    if err == nil {
        fmt.Printf("✅ Normal GC freed %d bytes\n", result.SpaceFreed)
    }
}

func (sgc *SmartGC) runGentleGC(ctx context.Context) {
    // Gentle GC (minimize system load)
    result, err := sgc.pinManager.RunGC(ctx, pin.GCOptions{
        Gentle:      true,
        MaxDuration: 2 * time.Minute,
    })

    if err == nil {
        fmt.Printf("✅ Gentle GC freed %d bytes\n", result.SpaceFreed)
    }
}
```

Now you have a complete understanding of Pin management and garbage collection in all aspects! 🚀