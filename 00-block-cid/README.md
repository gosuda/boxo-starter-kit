# 00-block-cid: IPFS의 기초 - Block과 CID

## 🎯 학습 목표

이 모듈을 통해 다음을 학습할 수 있습니다:
- IPFS의 핵심 개념인 **Content Addressing** 이해
- **Block**과 **CID(Content Identifier)**의 역할과 구조
- **CID v0**와 **CID v1**의 차이점과 사용 시나리오
- 다양한 **해시 알고리즘**(SHA2-256, BLAKE3 등)의 특성
- **Blockstore**를 통한 데이터 저장 및 검색 방법

## 📋 사전 요구사항

- Go 프로그래밍 기초 지식
- 암호학적 해시 함수의 기본 개념
- JSON 데이터 구조에 대한 이해

## 🔑 핵심 개념

### Content Addressing이란?

기존 파일시스템에서는 **위치 기반 주소**(예: `/home/user/document.txt`)를 사용합니다. 반면 IPFS는 **내용 기반 주소**를 사용합니다.

```
기존 방식: "어디에 있는가?" → /path/to/file.txt
IPFS 방식: "무엇인가?" → QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```

### Block이란?

**Block**은 IPFS에서 데이터를 저장하는 기본 단위입니다:

```go
type Block interface {
    RawData() []byte    // 실제 데이터
    Cid() cid.Cid      // 이 블록의 고유 식별자
}
```

### CID(Content Identifier)란?

**CID**는 IPFS에서 콘텐츠를 식별하는 고유한 주소입니다:

```
CID 구조: <version><codec><multihash>
예시: QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
```

#### CID v0 vs v1

| 특성 | CID v0 | CID v1 |
|------|--------|--------|
| 형식 | Base58 | Multibase (base32, base64 등) |
| 예시 | `QmYwAPJ...` | `bafybeig...` |
| 코덱 | DAG-PB 고정 | 다양한 코덱 지원 |
| 사용처 | 레거시 호환성 | 최신 애플리케이션 |

## 💻 코드 분석

### 1. Block Wrapper 구현

```go
// pkg/block.go:19-30
type BlockWrapper struct {
    blockstore blockstore.Blockstore
}

func New(bs blockstore.Blockstore) *BlockWrapper {
    if bs == nil {
        bs = blockstore.NewBlockstore(datastore.NewMapDatastore())
    }
    return &BlockWrapper{blockstore: bs}
}
```

**설계 결정**:
- `blockstore`가 nil인 경우 메모리 기반 저장소 자동 생성
- 의존성 주입을 통한 유연한 저장소 선택 가능

### 2. 데이터 저장 및 CID 생성

```go
// pkg/block.go:33-46
func (bw *BlockWrapper) Put(ctx context.Context, data []byte) (cid.Cid, error) {
    // 1. 해시 계산
    hash := sha256.Sum256(data)
    mhash, err := multihash.Encode(hash[:], multihash.SHA2_256)

    // 2. CID 생성 (v1, raw codec)
    c := cid.NewCidV1(cid.Raw, mhash)

    // 3. Block 생성 및 저장
    block, err := blocks.NewBlockWithCid(data, c)
    return c, bw.blockstore.Put(ctx, block)
}
```

**핵심 과정**:
1. **해시 계산**: SHA2-256으로 데이터의 지문 생성
2. **Multihash 인코딩**: 해시 알고리즘 정보 포함
3. **CID 생성**: v1 + Raw 코덱 조합
4. **Block 저장**: Blockstore에 영구 보관

### 3. 다양한 해시 알고리즘 지원

```go
// pkg/block.go:85-102
func (bw *BlockWrapper) PutWithHash(ctx context.Context, data []byte, hashType uint64) (cid.Cid, error) {
    switch hashType {
    case multihash.SHA2_256:
        hash := sha256.Sum256(data)
        mhash, err := multihash.Encode(hash[:], hashType)
        // ...
    case multihash.BLAKE3:
        hasher := blake3.New(32, nil)
        hasher.Write(data)
        hash := hasher.Sum(nil)
        mhash, err := multihash.Encode(hash, hashType)
        // ...
    }
}
```

**해시 알고리즘 비교**:

| 알고리즘 | 속도 | 보안성 | 사용 사례 |
|----------|------|--------|-----------|
| SHA2-256 | 보통 | 높음 | 기본 권장 |
| BLAKE3 | 빠름 | 높음 | 성능 중요 시 |

### 4. CID v0 호환성

```go
// pkg/block.go:149-163
func (bw *BlockWrapper) PutCIDv0(ctx context.Context, data []byte) (cid.Cid, error) {
    hash := sha256.Sum256(data)
    mhash, err := multihash.Encode(hash[:], multihash.SHA2_256)

    // CID v0: DAG-PB 코덱 사용
    c := cid.NewCidV0(mhash)

    block, err := blocks.NewBlockWithCid(data, c)
    return c, bw.blockstore.Put(ctx, block)
}
```

## 🏃‍♂️ 실습 가이드

### 1. 기본 실행

```bash
cd 00-block-cid
go run main.go
```

**예상 출력**:
```
=== Block and CID Demo ===

1. Basic Block Operations:
   ✅ Stored data → bafkreibvjvcv2i...
   ✅ Retrieved data matches original
   ✅ Block exists in blockstore

2. CID Version Comparison:
   📋 Same data, different CIDs:
      CID v0: QmYwAPJzv5CZsnA625s3Xf2ne...
      CID v1: bafkreibvjvcv2i5ijlrkflt...
   🔍 Both CIDs point to same data: true
```

### 2. 해시 알고리즘 비교 실험

코드에서 다음 부분을 관찰하세요:

```go
// main.go:111-125
// 같은 데이터, 다른 해시 알고리즘
data := []byte("Hash algorithm comparison")

sha256CID, _ := blockWrapper.PutWithHash(ctx, data, multihash.SHA2_256)
blake3CID, _ := blockWrapper.PutWithHash(ctx, data, multihash.BLAKE3)

fmt.Printf("   SHA2-256: %s\n", sha256CID.String()[:25]+"...")
fmt.Printf("   BLAKE3:   %s\n", blake3CID.String()[:25]+"...")
```

### 3. 코덱 영향 실험

```go
// main.go:135-149
// 같은 데이터, 다른 코덱
rawCID, _ := blockWrapper.PutWithCodec(ctx, data, cid.Raw)
dagPBCID, _ := blockWrapper.PutWithCodec(ctx, data, cid.DagProtobuf)

// 결과: 다른 CID가 생성됨
```

**학습 포인트**: 같은 데이터라도 코덱이 다르면 다른 CID가 생성됩니다.

### 4. 테스트 실행

```bash
go test -v ./...
```

**주요 테스트 케이스**:
- ✅ 기본 저장/검색 기능
- ✅ CID v0/v1 호환성
- ✅ 다양한 해시 알고리즘
- ✅ 에러 처리 (존재하지 않는 CID)

## 🔗 실제 활용 사례

### 1. 파일 무결성 검증

```go
// 파일 업로드 시 무결성 보장
originalCID, _ := blockWrapper.Put(ctx, fileData)

// 나중에 다운로드 시 검증
retrievedData, _ := blockWrapper.Get(ctx, originalCID)
// retrievedData == fileData 보장됨
```

### 2. 중복 제거 (Deduplication)

```go
// 같은 내용의 파일은 같은 CID 생성
file1CID, _ := blockWrapper.Put(ctx, []byte("Hello"))
file2CID, _ := blockWrapper.Put(ctx, []byte("Hello"))
// file1CID == file2CID (자동 중복 제거)
```

### 3. 버전 관리

```go
// 문서의 각 버전이 고유한 CID를 가짐
v1CID, _ := blockWrapper.Put(ctx, []byte("Document v1"))
v2CID, _ := blockWrapper.Put(ctx, []byte("Document v2"))
// 변경사항 추적 가능
```

## ⚠️ 주의사항 및 모범 사례

### 1. CID 버전 선택 가이드

```go
// ✅ 권장: 새로운 애플리케이션
cid := cid.NewCidV1(cid.Raw, mhash)

// ⚠️ 주의: 레거시 호환성이 필요한 경우만
cid := cid.NewCidV0(mhash)
```

### 2. 해시 알고리즘 선택

```go
// ✅ 범용: SHA2-256 (기본 권장)
hashType := multihash.SHA2_256

// ✅ 성능 중요: BLAKE3
hashType := multihash.BLAKE3

// ❌ 피하기: MD5, SHA1 (보안 취약)
```

### 3. 에러 처리

```go
// ✅ 항상 에러 확인
data, err := blockWrapper.Get(ctx, someCID)
if err != nil {
    if err == blockstore.ErrNotFound {
        // 블록이 존재하지 않음
    }
    return err
}
```

## 🔧 트러블슈팅

### 문제 1: "block not found" 에러

**원인**: 존재하지 않는 CID로 데이터 요청
```go
// 해결: 먼저 존재 여부 확인
exists, err := blockWrapper.Has(ctx, someCID)
if !exists {
    log.Printf("Block %s does not exist", someCID)
}
```

### 문제 2: 메모리 사용량 증가

**원인**: 대용량 데이터를 메모리 blockstore에 저장
```go
// 해결: 영구 저장소 사용 (다음 모듈에서 학습)
// 01-persistent 모듈 참조
```

### 문제 3: CID 형식 에러

**원인**: 잘못된 CID 문자열 파싱
```go
// 해결: CID 유효성 검사
if !cid.IsValid() {
    return fmt.Errorf("invalid CID format")
}
```

## 📚 추가 학습 자료

### 관련 문서
- [IPFS Concepts: Content Addressing](https://docs.ipfs.io/concepts/content-addressing/)
- [CID Specification](https://github.com/multiformats/cid)
- [Multihash Specification](https://github.com/multiformats/multihash)

### 다음 단계
1. **01-persistent**: 다양한 저장소 백엔드 학습
2. **02-dag-ipld**: 복잡한 데이터 구조와 DAG 학습
3. **03-unixfs**: 파일시스템 추상화 학습

## 🎓 연습 문제

### 기초 연습
1. 문자열 "Hello IPFS!"를 저장하고 CID를 출력하세요
2. 같은 데이터를 SHA2-256과 BLAKE3로 저장했을 때 CID 차이를 확인하세요
3. 존재하지 않는 CID로 데이터를 조회할 때의 에러를 처리하세요

### 심화 연습
1. JSON 객체를 직렬화하여 저장하고 다시 역직렬화하는 함수를 작성하세요
2. 파일의 CID를 계산하여 무결성을 검증하는 유틸리티를 만들어보세요
3. CID v0를 v1으로 변환하는 함수를 구현해보세요

이제 IPFS의 기초인 Block과 CID에 대해 이해하셨을 것입니다. 다음 모듈에서는 이 데이터를 영구적으로 저장하는 방법을 학습하겠습니다! 🚀