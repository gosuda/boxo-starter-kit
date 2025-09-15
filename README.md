# boxo-starter-kit
> **Boxo is Cool, but How to Use It?**

Step-by-step guide to using [Boxo](https://github.com/ipfs/boxo), the modular Go libraries extracted from IPFS.  
This repository provides small, incremental examples that show how to build with Boxo, from the very basics to higher-level concepts.

## Prerequisites
- Go 1.25+

## 🚀 How to Use
1. Clone this repository
```bash
git clone http://github.com/gosuda/boxo-starter-kit.git
cd boxo-starter-kit
```

2. run tests in chapters
```bash
go test ./00-block-cid/...
```

## 📚 Chapters

1. [00-block-cid](./00-block-cid): Block storage and Content Identifiers (CIDs)
2. [01-persistent](./01-persistent): Persistent storage backends
3. [02-network](./02-network): Peer-to-peer networking with libp2p
4. [03-bitswap](./03-bitswap): Bitswap protocol for data exchange
5. [04-dag-ipld](./04-dag-ipld): IPLD data
6. [05-unixfs](./05-unixfs): UnixFS file system abstraction
7. [06-pin-gc](./06-pin-gc): Pinning and Garbage Collection
8. [07-gateway](./07-gateway): Building a read-only HTTP Gateway
9. [08-ipns](./08-ipns): IPNS and mutable data
10. [99-kubo-api-demo](./99-kubo-api-demo): Mini Kubo API server
