# boxo-starter-kit
> **Boxo is Cool, but How to Use It?**

Step-by-step guide to using [Boxo](https://github.com/ipfs/boxo), the modular Go libraries extracted from IPFS.  
This repository provides small, incremental examples showing how to build with Boxo—from the basics to advanced topics.

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

🟦 **Part 1: Boxo Basics**
- [00-block-cid](./00-block-cid): Block storage and Content Identifiers (CIDs)
- [01-persistent](./01-persistent): Persistent storage backends
- [02-network](./02-network): Peer-to-peer networking with libp2p
- [03-bitswap](./03-bitswap): Bitswap protocol for data exchange
- [04-dag-ipld](./04-dag-ipld): DagService and IPLD format
- [05-unixfs-car](./05-unixfs-car): UnixFS file system and CAR format
- [06-mfs](./06-mfs): Mutable File System (MFS)
- [07-pin-gc](./07-pin-gc): Pinning and Garbage Collection
- [08-ipns](./08-ipns): IPNS and mutable data
- [09-gateway](./09-gateway): Building a HTTP Gateway
- [10-kubo-api-demo](./10-kubo-api-demo): Mini Kubo API server

🟧 **Part 2: Advanced**
- [11-ipld-prime](./11-ipld-prime): Using ipld-prime for IPLD data
- [12-dasl](./12-dasl): DASL for schema and type system
- [13-traversal-selector](./13-traversal-selector): Traversal and Selector
- [14-graphsync](./14-graphsync): GraphSync protocol & Data Transfer Layer (DTL)
- [15-trustless-gateway](./15-trustless-gateway): Trustless Gateway (Subdomain and DNSLink)
- [16-ipni](./16-ipni): IPNI and content indexing
- [17-multifetcher](./17-multifetcher): Multifetcher using Bitswap, GraphSync, and HTTP in parallel

## Contributing

All contributions are welcome!

If you find a bug, have an idea for improvement, or want to add a new chapter, feel free to open an [issue](https://github.com/gosuda/boxo-starter-kit/issues) or submit a [pull request](https://github.com/gosuda/boxo-starter-kit/pulls).