# 17-multifetcher: Multi-Source Content Fetching (Placeholder)

## 📍 Module Status

This module directory is currently a placeholder. The multifetcher implementation has been moved to:

**➡️ [18-multifetcher](../18-multifetcher)**

## 🎯 What is MultiFetcher?

MultiFetcher is a sophisticated content retrieval system that:
- **Combines Multiple Sources**: Fetch content from HTTP, GraphSync, Bitswap, and other protocols
- **Intelligent Racing**: Race multiple sources and use the fastest response
- **Fault Tolerance**: Automatic fallback when sources fail
- **Performance Optimization**: Choose optimal retrieval strategies
- **Provider Integration**: Works with IPNI for provider discovery

## 🔗 Implementation Location

The complete MultiFetcher implementation including:
- Core fetching logic
- HTTP fetcher implementation
- Multi-source coordination
- Comprehensive tests
- Usage examples

Can be found in: **[18-multifetcher](../18-multifetcher)**

## 📚 Learning Path

For the complete MultiFetcher learning experience:

1. **Start Here**: [18-multifetcher](../18-multifetcher) - Complete implementation
2. **Prerequisites**:
   - [17-ipni](../17-ipni) - Provider discovery
   - [15-graphsync](../15-graphsync) - P2P data transfer
   - [16-trustless-gateway](../16-trustless-gateway) - HTTP gateways
3. **Related**:
   - [02-network](../02-network) - Networking fundamentals
   - [06-unixfs-car](../06-unixfs-car) - CAR file handling

---

**Note**: This directory structure reflects the evolutionary development of the boxo-starter-kit. The multifetcher functionality has been consolidated in the 18-multifetcher module for better organization and completeness.