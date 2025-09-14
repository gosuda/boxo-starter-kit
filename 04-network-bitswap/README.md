# 04-network-bitswap: P2P 네트워킹과 블록 교환

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- **P2P 네트워킹**의 기본 원리와 libp2p 활용
- **Bitswap 프로토콜**의 작동 방식과 블록 교환 메커니즘
- **피어 발견(Peer Discovery)**과 연결 관리
- **Want-list** 기반 데이터 요청 시스템
- **Provider/Consumer** 패턴의 구현과 활용
- **네트워크 통계** 수집과 성능 모니터링

## 📋 사전 요구사항

- **00-block-cid** 모듈 완료 (Block과 CID 이해)
- **01-persistent** 모듈 완료 (데이터 영속성 이해)
- **02-dag-ipld** 모듈 완료 (DAG와 연결된 데이터)
- P2P 네트워킹의 기본 개념
- 암호화와 디지털 서명의 기본 이해

## 🔑 핵심 개념

### P2P 네트워킹이란?

**P2P(Peer-to-Peer)**는 중앙 서버 없이 피어들이 직접 연결되어 데이터를 교환하는 방식입니다:

```
중앙집중형:     Client ←→ Server ←→ Client
P2P:          Peer ←→ Peer ←→ Peer
                ↖       ↗
                  Peer
```

### Bitswap 프로토콜

**Bitswap**은 IPFS에서 블록을 효율적으로 교환하기 위한 프로토콜입니다:

```
1. Want-list: 필요한 블록들의 CID 목록
2. Have-list: 보유한 블록들의 CID 목록
3. Exchange: 상호 이익을 위한 블록 교환
4. Strategy: 공정한 교환을 위한 전략
```

### libp2p 네트워크 스택

```
Application Layer    │ IPFS, Bitswap
Protocol Layer       │ /ipfs/bitswap/1.2.0
Stream Layer         │ yamux, mplex
Security Layer       │ TLS, Noise
Transport Layer      │ TCP, QUIC, WebSocket
Network Layer        │ IPv4, IPv6
```

### 피어 발견 메커니즘

1. **Bootstrap Nodes**: 알려진 노드들로부터 시작
2. **DHT (Distributed Hash Table)**: 분산 라우팅 테이블
3. **mDNS**: 로컬 네트워크 자동 발견
4. **Relay**: NAT/방화벽 우회

## 💻 코드 분석

### 1. Bitswap Node 설계

```go
// pkg/bitswap.go:25-42
type BitswapNode struct {
    host       host.Host           // libp2p 호스트
    dagWrapper *dag.DagWrapper     // DAG 데이터 접근
    id         peer.ID             // 피어 식별자
    addresses  []multiaddr.Multiaddr // 네트워크 주소
    stats      struct {             // 통계 정보
        mutex         sync.RWMutex
        BlocksSent    int64 `json:"blocks_sent"`
        BlocksReceived int64 `json:"blocks_received"`
        PeersConnected int   `json:"peers_connected"`
        WantListSize   int   `json:"want_list_size"`
    }
}
```

**설계 특징**:
- **libp2p host**로 P2P 네트워킹 추상화
- **dag.DagWrapper** 재사용으로 데이터 레이어 연동
- **통계 수집**으로 성능 모니터링 지원
- **스레드 안전**한 동시성 처리

### 2. 네트워크 초기화

```go
// pkg/bitswap.go:44-85
func NewBitswapNode(dagWrapper *dag.DagWrapper) (*BitswapNode, error) {
    // 1. libp2p 호스트 생성
    h, err := libp2p.New(
        libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), // 동적 포트
        libp2p.RandomIdentity,                           // 랜덤 키 생성
        libp2p.DefaultSecurity,                          // 기본 보안
        libp2p.DefaultMuxers,                           // 스트림 멀티플렉싱
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create libp2p host: %w", err)
    }

    // 2. 피어 정보 수집
    peerID := h.ID()
    addrs := h.Addrs()

    fmt.Printf("   🆔 Peer ID: %s\n", peerID.String()[:12]+"...")
    fmt.Printf("   📍 Listening on %d addresses:\n", len(addrs))
    for i, addr := range addrs {
        if i < 3 { // 처음 3개만 표시
            fmt.Printf("      %s\n", addr.String())
        }
    }
    if len(addrs) > 3 {
        fmt.Printf("      ... and %d more\n", len(addrs)-3)
    }

    // 3. Bitswap 노드 생성
    node := &BitswapNode{
        host:       h,
        dagWrapper: dagWrapper,
        id:         peerID,
        addresses:  addrs,
    }

    return node, nil
}
```

### 3. 피어 연결 관리

```go
// pkg/bitswap.go:87-120
func (bn *BitswapNode) ConnectToPeer(ctx context.Context, peerAddr string) error {
    // 1. 멀티주소 파싱
    addr, err := multiaddr.NewMultiaddr(peerAddr)
    if err != nil {
        return fmt.Errorf("invalid multiaddr: %w", err)
    }

    // 2. 피어 정보 추출
    info, err := peer.AddrInfoFromP2pAddr(addr)
    if err != nil {
        return fmt.Errorf("failed to get peer info: %w", err)
    }

    // 3. 연결 시도
    fmt.Printf("   🔗 Connecting to peer %s...\n", info.ID.String()[:12]+"...")

    err = bn.host.Connect(ctx, *info)
    if err != nil {
        return fmt.Errorf("failed to connect to peer: %w", err)
    }

    // 4. 연결 성공 통계 업데이트
    bn.stats.mutex.Lock()
    bn.stats.PeersConnected++
    bn.stats.mutex.Unlock()

    fmt.Printf("   ✅ Successfully connected to peer\n")
    return nil
}
```

### 4. 블록 요청 시스템 (Want-list)

```go
// pkg/bitswap.go:159-190
func (bn *BitswapNode) RequestBlock(ctx context.Context, c cid.Cid) ([]byte, error) {
    // 1. 로컬에서 먼저 확인
    if exists, _ := bn.dagWrapper.Has(ctx, c); exists {
        return bn.dagWrapper.GetRaw(ctx, c)
    }

    // 2. Want-list에 추가
    bn.stats.mutex.Lock()
    bn.stats.WantListSize++
    bn.stats.mutex.Unlock()

    fmt.Printf("   📋 Added to want-list: %s\n", c.String()[:20]+"...")

    // 3. 연결된 피어들에게 요청
    connectedPeers := bn.host.Network().Peers()
    if len(connectedPeers) == 0 {
        return nil, fmt.Errorf("no connected peers to request block from")
    }

    fmt.Printf("   🔍 Requesting from %d connected peers\n", len(connectedPeers))

    // 4. 교육용 단순 구현: 첫 번째 피어에게만 요청
    // 실제 구현에서는 모든 피어에게 병렬 요청
    for _, peerID := range connectedPeers {
        fmt.Printf("   📤 Requesting block from peer %s\n", peerID.String()[:12]+"...")

        // 실제 Bitswap 프로토콜 메시지 전송
        // 여기서는 시뮬레이션
        if data := bn.simulateBlockRequest(ctx, peerID, c); data != nil {
            // 5. 받은 블록을 로컬 저장소에 캐시
            _, err := bn.dagWrapper.Put(ctx, data)
            if err != nil {
                return nil, fmt.Errorf("failed to store received block: %w", err)
            }

            bn.stats.mutex.Lock()
            bn.stats.BlocksReceived++
            bn.stats.WantListSize--
            bn.stats.mutex.Unlock()

            fmt.Printf("   ✅ Received and cached block\n")
            return data, nil
        }
    }

    return nil, fmt.Errorf("block not found on any connected peer")
}
```

### 5. 블록 제공 시스템 (Provide)

```go
// pkg/bitswap.go:122-157
func (bn *BitswapNode) ProvideBlock(ctx context.Context, data []byte) (cid.Cid, error) {
    // 1. 블록을 로컬 저장소에 저장
    c, err := bn.dagWrapper.Put(ctx, data)
    if err != nil {
        return cid.Undef, fmt.Errorf("failed to store block: %w", err)
    }

    fmt.Printf("   💾 Stored block locally: %s\n", c.String()[:20]+"...")

    // 2. 네트워크에 블록 제공 알림
    // 실제 구현에서는 DHT에 Provider 레코드 발행
    connectedPeers := bn.host.Network().Peers()
    if len(connectedPeers) > 0 {
        fmt.Printf("   📢 Announcing block to %d peers\n", len(connectedPeers))

        // 연결된 모든 피어에게 Have 메시지 전송
        for _, peerID := range connectedPeers {
            bn.announceBlockToPeer(ctx, peerID, c)
        }
    }

    // 3. 통계 업데이트
    bn.stats.mutex.Lock()
    bn.stats.BlocksSent++ // 제공 가능한 블록 증가
    bn.stats.mutex.Unlock()

    fmt.printf("   ✅ Block announced to network\n")
    return c, nil
}

// 교육용 시뮬레이션 함수
func (bn *BitswapNode) announceBlockToPeer(ctx context.Context, peerID peer.ID, c cid.Cid) {
    // 실제 구현에서는 Bitswap 프로토콜 메시지 전송
    fmt.Printf("   📡 Announced block %s to peer %s\n",
        c.String()[:12]+"...", peerID.String()[:12]+"...")
}
```

### 6. 네트워크 통계 및 모니터링

```go
// pkg/bitswap.go:243-273
func (bn *BitswapNode) GetStats() *BitswapStats {
    bn.stats.mutex.RLock()
    defer bn.stats.mutex.RUnlock()

    connectedPeers := bn.host.Network().Peers()

    return &BitswapStats{
        PeerID:         bn.id.String(),
        ConnectedPeers: len(connectedPeers),
        BlocksSent:     bn.stats.BlocksSent,
        BlocksReceived: bn.stats.BlocksReceived,
        WantListSize:   bn.stats.WantListSize,
        Addresses:      bn.getFormattedAddresses(),
    }
}

func (bn *BitswapNode) GetNetworkInfo() *NetworkInfo {
    connectedPeers := bn.host.Network().Peers()
    var peerInfo []PeerInfo

    for _, peerID := range connectedPeers {
        conns := bn.host.Network().ConnsToPeer(peerID)
        if len(conns) > 0 {
            peerInfo = append(peerInfo, PeerInfo{
                ID:        peerID.String(),
                Connected: true,
                Address:   conns[0].RemoteMultiaddr().String(),
            })
        }
    }

    return &NetworkInfo{
        LocalPeerID:    bn.id.String(),
        ConnectedPeers: peerInfo,
        ListenAddrs:    bn.getFormattedAddresses(),
    }
}
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 04-network-bitswap
go run main.go
```

**예상 출력**:
```
=== P2P Networking and Bitswap Demo ===

1. Creating first Bitswap node (Alice):
   🆔 Peer ID: 12D3KooWGRU...
   📍 Listening on 3 addresses:
      /ip4/127.0.0.1/tcp/64832
      /ip4/192.168.1.100/tcp/64832
      /ip6/::1/tcp/64832
   ✅ Alice node ready

2. Creating second Bitswap node (Bob):
   🆔 Peer ID: 12D3KooWHfv...
   📍 Listening on 3 addresses:
      /ip4/127.0.0.1/tcp/64833
      /ip4/192.168.1.100/tcp/64833
      /ip6/::1/tcp/64833
   ✅ Bob node ready

3. Connecting nodes:
   🔗 Connecting Bob to Alice...
   ✅ Successfully connected to peer

4. Block sharing demonstration:
   💾 Alice storing data: "Hello from Alice!"
   📢 Announcing block to 1 peers
   ✅ Block announced to network

   📋 Bob requesting Alice's block...
   🔍 Requesting from 1 connected peers
   📤 Requesting block from peer 12D3KooWGRU...
   ✅ Received and cached block
   ✅ Bob retrieved: "Hello from Alice!"

5. Network statistics:
   📊 Alice's stats:
      Peer ID: 12D3KooWGRU...
      Connected peers: 1
      Blocks sent: 1
      Blocks received: 0
      Want-list size: 0

   📊 Bob's stats:
      Peer ID: 12D3KooWHfv...
      Connected peers: 1
      Blocks sent: 0
      Blocks received: 1
      Want-list size: 0
```

### 2. 다중 노드 네트워크 실험

코드를 수정하여 더 많은 노드 생성:

```go
// 3개 이상의 노드로 네트워크 구성
nodes := []*bitswap.BitswapNode{}
for i := 0; i < 5; i++ {
    node, err := bitswap.NewBitswapNode(dagWrapper)
    if err != nil {
        log.Fatalf("Failed to create node %d: %v", i, err)
    }
    nodes = append(nodes, node)
}

// 메시 네트워크 구성 (모든 노드가 서로 연결)
for i, nodeA := range nodes {
    for j, nodeB := range nodes {
        if i != j {
            // nodeA가 nodeB에 연결
            addr := nodeB.GetListenAddr()
            nodeA.ConnectToPeer(ctx, addr)
        }
    }
}
```

### 3. 성능 벤치마크

```bash
# 다양한 블록 크기로 성능 측정
BLOCK_SIZE=1024 go run main.go      # 1KB 블록
BLOCK_SIZE=65536 go run main.go     # 64KB 블록
BLOCK_SIZE=1048576 go run main.go   # 1MB 블록
```

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ 노드 생성 및 초기화
- ✅ 피어 간 연결 설정
- ✅ 블록 요청/제공 기능
- ✅ 네트워크 통계 수집
- ✅ 에러 처리 및 재연결

## 🔍 고급 활용 사례

### 1. 컨텐츠 배포 네트워크 (CDN)

```go
type ContentDistributionNetwork struct {
    nodes map[string]*bitswap.BitswapNode
    cache map[string]cid.Cid // 인기 컨텐츠 캐시
}

func (cdn *ContentDistributionNetwork) DistributeContent(content []byte,
                                                        replicas int) error {
    // 1. 컨텐츠를 여러 노드에 복제
    var distributedNodes []*bitswap.BitswapNode

    for i := 0; i < replicas && i < len(cdn.nodes); i++ {
        node := cdn.selectOptimalNode() // 로드밸런싱
        cid, err := node.ProvideBlock(ctx, content)
        if err != nil {
            continue
        }

        distributedNodes = append(distributedNodes, node)
        fmt.Printf("Replicated content %s to node %s\n",
                  cid.String()[:12], node.GetPeerID()[:12])
    }

    return nil
}

func (cdn *ContentDistributionNetwork) GetContent(contentID cid.Cid) ([]byte, error) {
    // 2. 가장 가까운 노드에서 컨텐츠 요청
    fastestNode := cdn.findClosestNode()
    return fastestNode.RequestBlock(ctx, contentID)
}
```

### 2. 분산 파일 백업 시스템

```go
type DistributedBackup struct {
    nodes    []*bitswap.BitswapNode
    replicas int
}

func (db *DistributedBackup) BackupFile(filePath string) (*BackupManifest, error) {
    // 1. 파일을 청크로 분할
    chunks, err := db.chunkFile(filePath)
    if err != nil {
        return nil, err
    }

    manifest := &BackupManifest{
        OriginalPath: filePath,
        Chunks:       make([]ChunkInfo, 0),
        Timestamp:    time.Now(),
    }

    // 2. 각 청크를 여러 노드에 백업
    for i, chunk := range chunks {
        var chunkCIDs []cid.Cid

        // 복제본 생성
        for r := 0; r < db.replicas; r++ {
            node := db.selectBackupNode(i, r) // 다른 노드 선택
            cid, err := node.ProvideBlock(ctx, chunk)
            if err != nil {
                continue
            }
            chunkCIDs = append(chunkCIDs, cid)
        }

        manifest.Chunks = append(manifest.Chunks, ChunkInfo{
            Index:    i,
            Size:     len(chunk),
            Replicas: chunkCIDs,
        })
    }

    return manifest, nil
}

func (db *DistributedBackup) RestoreFile(manifest *BackupManifest,
                                        outputPath string) error {
    // 3. 분산된 청크들을 수집하여 파일 복원
    file, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer file.Close()

    for _, chunkInfo := range manifest.Chunks {
        var chunkData []byte

        // 복제본 중 하나라도 성공하면 OK
        for _, cid := range chunkInfo.Replicas {
            for _, node := range db.nodes {
                if data, err := node.RequestBlock(ctx, cid); err == nil {
                    chunkData = data
                    break
                }
            }
            if chunkData != nil {
                break
            }
        }

        if chunkData == nil {
            return fmt.Errorf("failed to retrieve chunk %d", chunkInfo.Index)
        }

        file.Write(chunkData)
    }

    return nil
}
```

### 3. 실시간 데이터 동기화

```go
type DataSyncNetwork struct {
    nodes      map[peer.ID]*bitswap.BitswapNode
    subscribers map[string][]peer.ID // 토픽별 구독자
}

func (dsn *DataSyncNetwork) PublishData(topic string, data []byte) error {
    // 1. 데이터를 네트워크에 저장
    publisher := dsn.selectPublisher()
    cid, err := publisher.ProvideBlock(ctx, data)
    if err != nil {
        return err
    }

    // 2. 구독자들에게 새 데이터 알림
    subscribers := dsn.subscribers[topic]
    for _, subscriberID := range subscribers {
        if node, exists := dsn.nodes[subscriberID]; exists {
            go func(n *bitswap.BitswapNode, c cid.Cid) {
                // 비동기로 데이터 동기화
                data, err := n.RequestBlock(ctx, c)
                if err == nil {
                    fmt.Printf("Synced %d bytes to subscriber %s\n",
                              len(data), n.GetPeerID()[:12])
                }
            }(node, cid)
        }
    }

    return nil
}

func (dsn *DataSyncNetwork) SubscribeToTopic(nodeID peer.ID, topic string) {
    if dsn.subscribers[topic] == nil {
        dsn.subscribers[topic] = make([]peer.ID, 0)
    }
    dsn.subscribers[topic] = append(dsn.subscribers[topic], nodeID)
}
```

## ⚠️ 주의사항 및 모범 사례

### 1. 네트워크 보안

```go
// ✅ 피어 인증 및 블랙리스트
type SecureBitswapNode struct {
    *BitswapNode
    trustedPeers map[peer.ID]bool
    blacklist    map[peer.ID]bool
}

func (sbn *SecureBitswapNode) ConnectToPeer(ctx context.Context,
                                           peerAddr string) error {
    info, err := peer.AddrInfoFromP2pAddr(multiaddr.NewMultiaddr(peerAddr))
    if err != nil {
        return err
    }

    // 블랙리스트 확인
    if sbn.blacklist[info.ID] {
        return fmt.Errorf("peer %s is blacklisted", info.ID)
    }

    // 신뢰할 수 있는 피어만 연결 (옵션)
    if len(sbn.trustedPeers) > 0 && !sbn.trustedPeers[info.ID] {
        return fmt.Errorf("peer %s is not trusted", info.ID)
    }

    return sbn.BitswapNode.ConnectToPeer(ctx, peerAddr)
}
```

### 2. 리소스 관리

```go
// ✅ 연결 수 제한 및 대역폭 관리
type ResourceManagedNode struct {
    *BitswapNode
    maxConnections int
    bandwidthLimit int64 // bytes per second
    lastTransfer   time.Time
    transferredBytes int64
}

func (rmn *ResourceManagedNode) RequestBlock(ctx context.Context,
                                           c cid.Cid) ([]byte, error) {
    // 대역폭 제한 확인
    if err := rmn.checkBandwidthLimit(); err != nil {
        return nil, err
    }

    // 연결 수 제한 확인
    if len(rmn.host.Network().Peers()) >= rmn.maxConnections {
        return nil, fmt.Errorf("maximum connections reached")
    }

    data, err := rmn.BitswapNode.RequestBlock(ctx, c)
    if err == nil {
        rmn.updateBandwidthUsage(int64(len(data)))
    }

    return data, err
}

func (rmn *ResourceManagedNode) checkBandwidthLimit() error {
    now := time.Now()
    if now.Sub(rmn.lastTransfer) > time.Second {
        rmn.transferredBytes = 0
        rmn.lastTransfer = now
    }

    if rmn.transferredBytes >= rmn.bandwidthLimit {
        return fmt.Errorf("bandwidth limit exceeded")
    }

    return nil
}
```

### 3. 에러 처리 및 재연결

```go
// ✅ 자동 재연결 및 회복 메커니즘
type ResilientBitswapNode struct {
    *BitswapNode
    reconnectAttempts int
    reconnectDelay    time.Duration
}

func (rbn *ResilientBitswapNode) ConnectWithRetry(ctx context.Context,
                                                 peerAddr string) error {
    var lastErr error

    for attempt := 0; attempt < rbn.reconnectAttempts; attempt++ {
        err := rbn.BitswapNode.ConnectToPeer(ctx, peerAddr)
        if err == nil {
            return nil
        }

        lastErr = err
        fmt.Printf("Connection attempt %d failed: %v\n", attempt+1, err)

        if attempt < rbn.reconnectAttempts-1 {
            time.Sleep(rbn.reconnectDelay * time.Duration(attempt+1))
        }
    }

    return fmt.Errorf("failed to connect after %d attempts: %w",
                     rbn.reconnectAttempts, lastErr)
}

func (rbn *ResilientBitswapNode) MonitorConnections() {
    ticker := time.NewTicker(30 * time.Second)
    go func() {
        for range ticker.C {
            connectedPeers := rbn.host.Network().Peers()
            fmt.Printf("Health check: %d peers connected\n", len(connectedPeers))

            // 연결이 끊긴 피어들 재연결 시도
            for _, peerID := range connectedPeers {
                conns := rbn.host.Network().ConnsToPeer(peerID)
                if len(conns) == 0 {
                    fmt.Printf("Reconnecting to peer %s\n", peerID.String()[:12])
                    // 재연결 로직...
                }
            }
        }
    }()
}
```

## 🔧 트러블슈팅

### 문제 1: "connection refused" 에러

**원인**: 방화벽 또는 NAT 문제
```go
// 해결: 다양한 전송 프로토콜 시도
func createRobustHost() (host.Host, error) {
    return libp2p.New(
        libp2p.ListenAddrStrings(
            "/ip4/0.0.0.0/tcp/0",        // TCP
            "/ip4/0.0.0.0/udp/0/quic",   // QUIC
            "/ip6/::/tcp/0",             // IPv6 TCP
        ),
        libp2p.EnableRelay(),            // 릴레이 활성화
        libp2p.EnableAutoRelay(),        // 자동 릴레이
        libp2p.NATPortMap(),            // UPnP 포트 매핑
    )
}
```

### 문제 2: "peer not found" 에러

**원인**: 피어 발견 실패
```go
// 해결: 부트스트랩 노드 사용
func connectToBootstrap(h host.Host) error {
    bootstrapPeers := []string{
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
    }

    for _, peerAddr := range bootstrapPeers {
        addr, _ := multiaddr.NewMultiaddr(peerAddr)
        info, _ := peer.AddrInfoFromP2pAddr(addr)
        h.Connect(context.Background(), *info)
    }
    return nil
}
```

### 문제 3: 메모리 부족

**원인**: 대용량 블록 처리
```go
// 해결: 스트리밍 블록 전송
func requestLargeBlock(bn *BitswapNode, c cid.Cid) error {
    // 블록을 청크 단위로 요청
    blockInfo, err := bn.getBlockInfo(c)
    if err != nil {
        return err
    }

    if blockInfo.Size > MaxBlockSize {
        return bn.requestBlockInChunks(c, blockInfo.Size)
    }

    _, err = bn.RequestBlock(context.Background(), c)
    return err
}
```

## 📚 추가 학습 자료

### 관련 문서
- [libp2p Documentation](https://docs.libp2p.io/)
- [Bitswap Specification](https://github.com/ipfs/specs/blob/master/BITSWAP.md)
- [IPFS Networking](https://docs.ipfs.io/concepts/networking/)
- [DHT in IPFS](https://docs.ipfs.io/concepts/dht/)

### 다음 단계
1. **05-pin-gc**: 데이터 생명주기 관리와 Pin/GC
2. **06-gateway**: HTTP 인터페이스로 웹 통합
3. **07-ipns**: 네이밍 시스템과 동적 콘텐츠

## 🍳 쿡북 (Cookbook) - 바로 사용할 수 있는 코드

### 📡 간단한 P2P 파일 공유

```go
package main

import (
    "context"
    "fmt"
    "os"

    bitswap "github.com/gosunuts/boxo-starter-kit/04-network-bitswap/pkg"
    dag "github.com/gosunuts/boxo-starter-kit/02-dag-ipld/pkg"
)

// 파일을 P2P 네트워크에 공유
func shareFile(filename string) {
    ctx := context.Background()

    // DAG와 Bitswap 노드 초기화
    dagWrapper, _ := dag.New(nil, "")
    node, _ := bitswap.NewBitswapNode(dagWrapper)
    defer node.Close()

    // 파일 읽기 및 공유
    data, _ := os.ReadFile(filename)
    cid, _ := node.ProvideBlock(ctx, data)

    fmt.Printf("파일 공유됨: %s\n", cid.String())
    fmt.Printf("다른 노드에서 요청: %s\n", node.GetListenAddr())
}
```

### 🔄 자동 재연결 P2P 노드

```go
type AutoReconnectNode struct {
    *bitswap.BitswapNode
    knownPeers []string
    isRunning  bool
}

func NewAutoReconnectNode(dagWrapper *dag.DagWrapper) *AutoReconnectNode {
    node, _ := bitswap.NewBitswapNode(dagWrapper)

    arn := &AutoReconnectNode{
        BitswapNode: node,
        knownPeers:  make([]string, 0),
        isRunning:   true,
    }

    // 자동 재연결 고루틴 시작
    go arn.autoReconnectLoop()

    return arn
}

func (arn *AutoReconnectNode) autoReconnectLoop() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for arn.isRunning {
        select {
        case <-ticker.C:
            for _, peerAddr := range arn.knownPeers {
                arn.ConnectToPeer(context.Background(), peerAddr)
            }
        }
    }
}

func (arn *AutoReconnectNode) AddKnownPeer(peerAddr string) {
    arn.knownPeers = append(arn.knownPeers, peerAddr)
}
```

### 📊 네트워크 상태 모니터

```go
type NetworkMonitor struct {
    node   *bitswap.BitswapNode
    stats  chan *bitswap.BitswapStats
    stopCh chan bool
}

func NewNetworkMonitor(node *bitswap.BitswapNode) *NetworkMonitor {
    nm := &NetworkMonitor{
        node:   node,
        stats:  make(chan *bitswap.BitswapStats, 100),
        stopCh: make(chan bool),
    }

    go nm.monitorLoop()
    return nm
}

func (nm *NetworkMonitor) monitorLoop() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            stats := nm.node.GetStats()
            nm.stats <- stats

            // 콘솔에 실시간 출력
            fmt.Printf("\r연결된 피어: %d | 송신: %d | 수신: %d | 대기열: %d",
                stats.ConnectedPeers, stats.BlocksSent,
                stats.BlocksReceived, stats.WantListSize)

        case <-nm.stopCh:
            return
        }
    }
}

func (nm *NetworkMonitor) GetLatestStats() *bitswap.BitswapStats {
    select {
    case stats := <-nm.stats:
        return stats
    default:
        return nm.node.GetStats()
    }
}
```

### 🏃‍♂️ 고성능 블록 다운로더

```go
type ParallelDownloader struct {
    nodes      []*bitswap.BitswapNode
    workerPool chan bool
    resultChan chan downloadResult
}

type downloadResult struct {
    cid  cid.Cid
    data []byte
    err  error
}

func NewParallelDownloader(nodes []*bitswap.BitswapNode, workers int) *ParallelDownloader {
    return &ParallelDownloader{
        nodes:      nodes,
        workerPool: make(chan bool, workers),
        resultChan: make(chan downloadResult, workers*2),
    }
}

func (pd *ParallelDownloader) DownloadBlocks(cids []cid.Cid) map[string][]byte {
    results := make(map[string][]byte)

    // 모든 CID에 대해 다운로드 작업 시작
    for _, c := range cids {
        go pd.downloadBlock(c)
    }

    // 결과 수집
    for i := 0; i < len(cids); i++ {
        result := <-pd.resultChan
        if result.err == nil {
            results[result.cid.String()] = result.data
        }
    }

    return results
}

func (pd *ParallelDownloader) downloadBlock(c cid.Cid) {
    pd.workerPool <- true // 워커 슬롯 확보
    defer func() { <-pd.workerPool }() // 슬롯 해제

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // 여러 노드에서 병렬로 시도
    for _, node := range pd.nodes {
        if data, err := node.RequestBlock(ctx, c); err == nil {
            pd.resultChan <- downloadResult{cid: c, data: data, err: nil}
            return
        }
    }

    pd.resultChan <- downloadResult{
        cid: c,
        err: fmt.Errorf("failed to download from all nodes"),
    }
}
```

### 🌐 분산 웹사이트 호스팅

```go
type DistributedWebsite struct {
    nodes    []*bitswap.BitswapNode
    content  map[string]cid.Cid // path -> CID
    manifest cid.Cid
}

func NewDistributedWebsite(nodes []*bitswap.BitswapNode) *DistributedWebsite {
    return &DistributedWebsite{
        nodes:   nodes,
        content: make(map[string]cid.Cid),
    }
}

func (dw *DistributedWebsite) UploadSite(siteDir string) error {
    ctx := context.Background()

    err := filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }

        // 파일 읽기
        data, err := os.ReadFile(path)
        if err != nil {
            return err
        }

        // 여러 노드에 복제 업로드
        relativePath, _ := filepath.Rel(siteDir, path)
        node := dw.selectOptimalNode() // 로드밸런싱

        cid, err := node.ProvideBlock(ctx, data)
        if err != nil {
            return err
        }

        dw.content[relativePath] = cid
        fmt.Printf("업로드됨: %s -> %s\n", relativePath, cid.String()[:20]+"...")

        return nil
    })

    if err != nil {
        return err
    }

    // 매니페스트 생성 및 업로드
    return dw.createManifest()
}

func (dw *DistributedWebsite) createManifest() error {
    manifestData, _ := json.Marshal(dw.content)
    node := dw.selectOptimalNode()

    manifestCID, err := node.ProvideBlock(context.Background(), manifestData)
    if err != nil {
        return err
    }

    dw.manifest = manifestCID
    fmt.Printf("웹사이트 매니페스트: %s\n", manifestCID.String())
    return nil
}

func (dw *DistributedWebsite) selectOptimalNode() *bitswap.BitswapNode {
    // 간단한 라운드로빈 선택
    return dw.nodes[rand.Intn(len(dw.nodes))]
}
```

### 📱 P2P 채팅 앱

```go
type P2PChat struct {
    node     *bitswap.BitswapNode
    username string
    messages chan ChatMessage
    peers    map[peer.ID]string // peerID -> username
}

type ChatMessage struct {
    From      string    `json:"from"`
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    Signature string    `json:"signature"`
}

func NewP2PChat(dagWrapper *dag.DagWrapper, username string) *P2PChat {
    node, _ := bitswap.NewBitswapNode(dagWrapper)

    chat := &P2PChat{
        node:     node,
        username: username,
        messages: make(chan ChatMessage, 100),
        peers:    make(map[peer.ID]string),
    }

    go chat.messageListener()
    return chat
}

func (pc *P2PChat) SendMessage(message string) error {
    chatMsg := ChatMessage{
        From:      pc.username,
        Message:   message,
        Timestamp: time.Now(),
    }

    // 메시지를 JSON으로 직렬화
    msgData, _ := json.Marshal(chatMsg)

    // 네트워크에 브로드캐스트
    cid, err := pc.node.ProvideBlock(context.Background(), msgData)
    if err != nil {
        return err
    }

    fmt.Printf("메시지 전송됨: %s\n", cid.String()[:20]+"...")
    return nil
}

func (pc *P2PChat) messageListener() {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        // 연결된 피어들로부터 새 메시지 확인
        // 실제 구현에서는 pubsub 패턴 사용
        pc.checkForNewMessages()
    }
}

func (pc *P2PChat) GetMessages() []ChatMessage {
    var messages []ChatMessage

    for {
        select {
        case msg := <-pc.messages:
            messages = append(messages, msg)
        default:
            return messages
        }
    }
}
```

이제 P2P 네트워킹의 기초부터 실용적인 활용까지 완벽하게 이해하실 수 있을 것입니다! 🚀