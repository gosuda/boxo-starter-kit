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
- [03-dht-router](./03-dht-router): DHT routing for peer discovery
- [04-bitswap](./04-bitswap): Bitswap protocol for data exchange
- [05-dag-ipld](./05-dag-ipld): DagService and IPLD format
- [06-unixfs-car](./06-unixfs-car): UnixFS file system and CAR format
- [07-mfs](./07-mfs): Mutable File System (MFS)
- [08-pin-gc](./08-pin-gc): Pinning and Garbage Collection
- [09-ipns](./09-ipns): IPNS and mutable data
- [10-gateway](./10-gateway): Building a HTTP Gateway
- [11-kubo-api-demo](./11-kubo-api-demo): Mini Kubo API server

🟧 **Part 2: Advanced**
- [12-ipld-prime](./12-ipld-prime): Using ipld-prime for IPLD data
- [13-dasl](./13-dasl): DASL for schema and type system
- [14-traversal-selector](./14-traversal-selector): Traversal and Selector
- [15-graphsync](./15-graphsync): GraphSync protocol & Data Transfer Layer (DTL)
- [16-trustless-gateway](./16-trustless-gateway): Trustless Gateway (Subdomain and DNSLink)
- [17-ipni](./17-ipni): IPNI and content indexing
- [18-multifetcher](./18-multifetcher): Multifetcher using Bitswap, GraphSync, and HTTP in parallel

## Contributing

All contributions are welcome!

If you find a bug, have an idea for improvement, or want to add a new chapter, feel free to open an [issue](https://github.com/gosuda/boxo-starter-kit/issues) or submit a [pull request](https://github.com/gosuda/boxo-starter-kit/pulls).