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
5. [04-dag-ipld](./04-dag-ipld): DagService and IPLD format
6. [05-unixfs-car](./05-unixfs-car): UnixFS file system and CAR format
7. [06-mfs](./06-mfs): Mutable File System (MFS)
8. [07-pin-gc](./07-pin-gc): Pinning and Garbage Collection
9. [08-ipns](./08-ipns): IPNS and mutable data
10. [09-ipni](./09-ipni): IPNI and content indexing
11. [10-gateway](./10-gateway): Building a read-only HTTP Gateway
12. [99-kubo-api-demo](./99-kubo-api-demo): Mini Kubo API server

## Contributing

All contributions are welcome!

If you find a bug, have an idea for improvement, or want to add a new chapter, feel free to open an [issue](https://github.com/gosuda/boxo-starter-kit/issues) or submit a [pull request](https://github.com/gosuda/boxo-starter-kit/pulls).