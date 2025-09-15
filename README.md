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
3. [02-dag-ipld](./02-dag-ipld): IPLD data
4. [03-unixfs](./03-unixfs): UnixFS file system abstraction
5. [04-network-bitswap](./04-network-bitswap): Peer-to-peer networking with Bitswap
6. [05-pin-gc](./05-pin-gc): Pinning and Garbage Collection
7. [06-gateway](./06-gateway): Building a read-only HTTP Gateway
8. [07-ipns](./07-ipns): IPNS and mutable data
9. [99-kubo-api-demo](./99-kubo-api-demo): Mini Kubo API server
