# 04-bitswap: Peer-to-Peer Block Exchange Protocol

## 🎯 Learning Objectives
- Understand the Bitswap protocol and its role in IPFS
- Learn how blocks are exchanged between peers
- Implement peer-to-peer block sharing mechanisms
- Explore want-lists and block exchange strategies

## 📋 Prerequisites

**Critical Prerequisites (Must Complete):**
- **00-block-cid**: REQUIRED - Content addressing and block identification
  - Bitswap exchanges blocks identified by CIDs
  - Understanding block validation and integrity checking is essential
  - CID computation and verification are core to secure block exchange
- **02-network**: REQUIRED - P2P networking and peer communication
  - Bitswap runs on top of libp2p networking protocols
  - Understanding peer connections, streams, and protocol handlers is mandatory
  - Knowledge of peer discovery and connection management is essential

**Highly Recommended Prerequisites:**
- **03-dht-router**: DHT-based content discovery (strongly recommended)
  - DHT finds peers who have desired blocks
  - Without DHT, Bitswap can only exchange blocks with directly connected peers
  - **Learning Path**: DHT discovery → Bitswap exchange = complete content system
- **01-persistent**: Block storage and persistence concepts
  - Understanding different storage backends for received blocks
  - Knowledge of block storage patterns and performance considerations

**Knowledge Requirements:**
- Understanding of peer-to-peer protocols and trading mechanisms
- Familiarity with want-list and have-list concepts
- Basic understanding of network protocols and data exchange patterns

## 🔑 Key Concepts

### What is Bitswap?
Bitswap is the data trading module for IPFS. It manages requesting and sending blocks to and from other peers in the network. Think of it as a marketplace where peers can request blocks they need and provide blocks they have.

### Core Components
- **Want-list**: List of blocks a peer wants to receive
- **Have-list**: List of blocks a peer can provide
- **Block Exchange**: The actual transfer of data blocks
- **Provider Discovery**: Finding peers who have desired blocks

### How Bitswap Works
1. **Request**: Peer A wants a specific block (CID)
2. **Announce**: Peer A announces its want-list to connected peers
3. **Response**: Peer B checks if it has the block and sends it
4. **Receipt**: Peer A receives the block and removes it from want-list

## 💻 Code Structure

### BitswapWrapper
```go
type BitswapWrapper struct {
    HostWrapper       *network.HostWrapper    // P2P networking
    PersistentWrapper *persistent.PersistentWrapper // Local storage
    *bitswap.Bitswap                         // Core bitswap functionality
}
```

### Key Functions
- `New()`: Create a new bitswap node
- `PutBlockRaw()`: Store and announce a new block
- `GetBlock()`: Request a block from the network
- `GetBlockRaw()`: Get raw block data by CID

## 🏃‍♂️ Running the Examples

### Prerequisites
Make sure you have completed the setup from previous modules:
```bash
# Navigate to the project root
cd boxo-starter-kit

# Install dependencies
go mod download
```

### Run Tests
```bash
# Run bitswap tests
go test ./04-bitswap/... -v

# Run with race detection
go test ./04-bitswap/... -v -race
```

### Example Output
```
=== RUN   TestBitswap
--- PASS: TestBitswap (0.15s)
=== RUN   TestBitswapMultiplePeers
--- PASS: TestBitswapMultiplePeers (0.31s)
```

## 🧪 Practical Examples

### Basic Block Exchange
```go
// Create two bitswap nodes
bswap1, _ := bitswap.New(ctx, nil, nil)
bswap2, _ := bitswap.New(ctx, nil, nil)

// Connect the peers
bswap1.HostWrapper.ConnectToPeer(ctx, bswap2.HostWrapper.GetFullAddresses()...)

// Node 1 stores a block
data := []byte("Hello, Bitswap!")
cid, _ := bswap1.PutBlockRaw(ctx, data)

// Node 2 retrieves the block
retrievedData, _ := bswap2.GetBlockRaw(ctx, cid)
// retrievedData now contains "Hello, Bitswap!"
```

### Multiple Peer Network
The test demonstrates how blocks can be shared across a network of multiple peers, showcasing the distributed nature of IPFS block exchange.

## 🔍 Understanding the Implementation

### Integration with Previous Modules
- **Network Layer**: Uses `02-network` for peer connections and discovery
- **Storage Layer**: Uses `01-persistent` for local block storage
- **Content Addressing**: Uses `00-block-cid` concepts for block identification

### Bitswap Configuration
```go
bswap := bitswap.New(ctx, bsnet, nil, persistentWrapper,
    bitswap.SetSendDontHaves(true),        // Send "don't have" messages
    bitswap.ProviderSearchDelay(time.Second*5), // Delay before searching
)
```

### Block Announcement
When a new block is stored, Bitswap announces it to connected peers:
```go
func (b *BitswapWrapper) PutBlockRaw(ctx context.Context, data []byte) (cid.Cid, error) {
    // Store the block locally
    c, err := b.PersistentWrapper.PutRaw(ctx, data)

    // Create block wrapper
    blk, err := blocks.NewBlockWithCid(data, c)

    // Announce to network
    return c, b.Bitswap.NotifyNewBlocks(ctx, blk)
}
```

## 📚 Next Steps

### Immediate Next Steps
1. **[05-dag-ipld](../05-dag-ipld)**: Learn to structure complex data as interconnected blocks
   - **Connection**: Use Bitswap to exchange DAG nodes across the network
   - **Why Next**: Learn how to organize blocks into meaningful data structures
   - **Learning Focus**: Content-addressed graphs and linked data

2. **[06-unixfs-car](../06-unixfs-car)**: Build file systems on top of block exchange
   - **Connection**: Use Bitswap to transfer file and directory blocks
   - **Why Important**: Practical file storage using the blocks you can now exchange
   - **Learning Focus**: File system abstractions over content-addressed storage

### Related Modules
3. **[08-pin-gc](../08-pin-gc)**: Control which blocks to keep permanently
   - **Connection**: Manage the lifecycle of blocks received via Bitswap
   - **Why Critical**: Prevent important blocks from being garbage collected
   - **Relevance**: Content persistence and storage optimization

4. **[10-gateway](../10-gateway)**: HTTP interfaces for accessing Bitswap content
   - **Connection**: Serve blocks obtained via Bitswap through web interfaces
   - **When to Learn**: When building web applications that serve P2P content

5. **[09-ipns](../09-ipns)**: Mutable pointers to content exchanged via Bitswap
   - **Connection**: Create updateable references to content discovered and exchanged
   - **Advanced Use**: Mutable naming in immutable content systems

### Alternative Learning Paths
- **For Data Structure Focus**: Go directly to **[05-dag-ipld](../05-dag-ipld)** for complex data organization
- **For File System Focus**: Jump to **[06-unixfs-car](../06-unixfs-car)** for practical file storage
- **For Storage Management**: Skip to **[08-pin-gc](../08-pin-gc)** for data lifecycle management
- **For Web Integration**: Go to **[10-gateway](../10-gateway)** for HTTP/web integration
- **For Complete Integration**: Try **[11-kubo-api-demo](../11-kubo-api-demo)** to see Bitswap in complete IPFS context

## 📚 Additional Resources
- [Bitswap Specification](https://specs.ipfs.tech/bitswap-protocol/)
- [IPFS Bitswap Documentation](https://docs.ipfs.tech/concepts/bitswap/)

## 🛠 Troubleshooting

### Common Issues
1. **Connection Timeout**: Ensure peers can discover each other
2. **Block Not Found**: Verify the block was properly announced
3. **Network Isolation**: Check if peers are on the same network

### Debug Tips
- Use `-v` flag with tests to see detailed logs
- Check peer connection status before block exchange
- Verify CID consistency between peers

---

*This module demonstrates the core P2P block exchange mechanism that makes IPFS a distributed system.*