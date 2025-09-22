# 04-bitswap: Peer-to-Peer Block Exchange Protocol

## 🎯 Learning Objectives
- Understand the Bitswap protocol and its role in IPFS
- Learn how blocks are exchanged between peers
- Implement peer-to-peer block sharing mechanisms
- Explore want-lists and block exchange strategies

## 📋 Prerequisites
- **02-network**: Understanding of libp2p networking and peer connections
- **01-persistent**: Knowledge of block storage and persistence
- **00-block-cid**: Basic concepts of blocks and CIDs

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

## 🔗 Next Steps
- **05-dag-ipld**: Learn about structured data with DAGs
- **06-pin-gc**: Understand data persistence and garbage collection
- **10-gateway**: Explore HTTP interfaces for IPFS

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