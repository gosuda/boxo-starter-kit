# 13-dasl: IPLD Schema & Data Modeling with DASL

## 🎯 Learning Objectives

By the end of this module, you will understand:
- How to define and use IPLD schemas with DASL (Data Schema Language)
- Implementing type-safe data structures with schema validation
- Code generation and binding between Go structs and IPLD schemas
- Working with linked data using schema-defined relationships
- Advanced features of IPLD schema system for data modeling
- Performance benefits of schema-based serialization

## 📋 Prerequisites

- Completion of [12-ipld-prime](../12-ipld-prime) - IPLD-prime fundamentals
- Understanding of [05-dag-ipld](../05-dag-ipld) - Basic IPLD concepts
- Familiarity with [00-block-cid](../00-block-cid) - Content addressing
- Knowledge of Go struct tags and reflection
- Basic understanding of schema definition languages

## 🔑 Core Concepts

### IPLD Schema & DASL

**IPLD Schema** provides:
- **Type Safety**: Strict typing and validation for IPLD data
- **Documentation**: Self-documenting data structures
- **Code Generation**: Automatic Go struct generation from schemas
- **Validation**: Runtime validation of data against schemas
- **Interoperability**: Language-agnostic data structure definitions

**DASL (Data Schema Language)** is the DSL for defining IPLD schemas:
- Concise syntax for type definitions
- Support for primitives, structs, unions, and links
- Reference types and collections
- Embedded in Go code or external files

### Key Schema Features

#### 1. Type System
```dasl
type User struct {
  id        String
  name      String
  email     String
  friends   [&User]    # List of User references
  avatar    Bytes
}
```

#### 2. Link Types
```dasl
type Post struct {
  author    &User      # Link to User
  title     String
  tags      [String]   # List of strings
  createdAt Int
}
```

#### 3. Composite Types
```dasl
type Root struct {
  users User
  posts Post
}
```

## 💻 Code Architecture

### Module Structure
```
13-dasl/
├── pkg/
│   ├── dasl.go              # Main wrapper with schema operations
│   ├── types.go             # Go struct definitions with IPLD tags
│   └── codegen/
│       ├── schema.dasl      # IPLD schema definition
│       ├── main.go          # Code generation utility
│       └── ipldsch_minima.go # Generated schema bindings
└── dasl_test.go            # Comprehensive tests
```

### Core Components

#### DaslWrapper
The main wrapper providing schema-based operations:

```go
type DaslWrapper struct {
    ipld *ipldprime.IpldWrapper  // Underlying IPLD operations
    ts   *schema.TypeSystem      // Compiled schema type system

    // Schema types
    tRoot schema.Type
    tUser schema.Type
    tPost schema.Type
}
```

#### Go Struct Definitions
Type-safe Go representations with IPLD tags:

```go
type User struct {
    Id      string    `ipld:"id"`
    Name    string    `ipld:"name"`
    Email   string    `ipld:"email"`
    Friends []cid.Cid `ipld:"friends"`
    Avatar  []byte    `ipld:"avatar"`
}

type Post struct {
    Id        string   `ipld:"id"`
    Author    cid.Cid  `ipld:"author"`
    Title     string   `ipld:"title"`
    Body      string   `ipld:"body"`
    Tags      []string `ipld:"tags"`
    CreatedAt int64    `ipld:"createdAt"`
}
```

#### Schema Operations
Type-safe CRUD operations for each schema type:

- `PutUser(ctx, *User) -> CID`: Store user with schema validation
- `GetUser(ctx, CID) -> *User`: Retrieve and validate user
- `PutPost(ctx, *Post) -> CID`: Store post with schema validation
- `GetPost(ctx, CID) -> *Post`: Retrieve and validate post
- `PutRoot(ctx, *Root) -> CID`: Store root with schema validation
- `GetRoot(ctx, CID) -> *Root`: Retrieve and validate root

## 🏃‍♂️ Usage Examples

### Basic Schema Operations

```go
package main

import (
    "context"
    "fmt"
    "time"

    dasl "github.com/gosuda/boxo-starter-kit/13-dasl/pkg"
)

func main() {
    ctx := context.Background()

    // Create DASL wrapper with compiled schema
    wrapper, err := dasl.NewDaslWrapper(nil)
    if err != nil {
        panic(err)
    }

    // Create and store a user
    user := &dasl.User{
        Id:     "alice_2024",
        Name:   "Alice Smith",
        Email:  "alice@example.com",
        Avatar: []byte("profile-image-data"),
    }

    userCID, err := wrapper.PutUser(ctx, user)
    if err != nil {
        panic(err)
    }

    fmt.Printf("User stored with CID: %s\\n", userCID)

    // Retrieve and verify the user
    retrievedUser, err := wrapper.GetUser(ctx, userCID)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Retrieved user: %s <%s>\\n",
        retrievedUser.Name, retrievedUser.Email)
}
```

### Working with Linked Data

```go
// Create users
alice := &dasl.User{
    Id:    "alice",
    Name:  "Alice",
    Email: "alice@example.com",
}
aliceCID, _ := wrapper.PutUser(ctx, alice)

bob := &dasl.User{
    Id:    "bob",
    Name:  "Bob",
    Email: "bob@example.com",
}
bobCID, _ := wrapper.PutUser(ctx, bob)

// Create a post by Alice
post := &dasl.Post{
    Id:        "post_001",
    Author:    aliceCID,           // Link to Alice
    Title:     "Introduction to IPLD Schemas",
    Body:      "IPLD schemas provide type safety...",
    Tags:      []string{"ipld", "tutorial", "schemas"},
    CreatedAt: time.Now().Unix(),
}

postCID, err := wrapper.PutPost(ctx, post)
if err != nil {
    panic(err)
}

// Create root structure linking everything
root := &dasl.Root{
    Users: *alice,  // Embedded user data
    Posts: *post,   // Embedded post data
}

rootCID, err := wrapper.PutRoot(ctx, root)
if err != nil {
    panic(err)
}

fmt.Printf("Complete data structure stored at: %s\\n", rootCID)
```

### Schema Validation in Action

```go
// This will succeed - valid data
validUser := &dasl.User{
    Id:    "valid_user",
    Name:  "Valid Name",
    Email: "valid@email.com",
}
_, err := wrapper.PutUser(ctx, validUser)
// err == nil

// The schema ensures type safety at compile time
// Invalid operations won't compile:
// invalidUser := &dasl.User{
//     Id: 123,  // Compile error: cannot use int as string
// }

// Runtime validation happens during Put operations
nilUser := (*dasl.User)(nil)
_, err = wrapper.PutUser(ctx, nilUser)
// err != nil: "PutUser: nil value"
```

## 🏃‍♂️ Running the Examples

### Run Tests
```bash
cd 13-dasl
go test -v
```

### Expected Output
```
=== RUN   TestDaslWrapperPutGet
--- PASS: TestDaslWrapperPutGet (0.01s)
PASS
ok      github.com/gosuda/boxo-starter-kit/13-dasl    0.123s
```

### Code Generation (Optional)
```bash
cd 13-dasl/pkg/codegen
go run main.go
```

This regenerates Go bindings from the DASL schema if you modify `schema.dasl`.

## 🔧 Schema Definition Guide

### DASL Syntax Reference

#### Basic Types
```dasl
type SimpleUser struct {
    name     String    # Text data
    age      Int       # Numeric data
    active   Bool      # Boolean flag
    avatar   Bytes     # Binary data
}
```

#### Collections
```dasl
type UserWithCollections struct {
    tags     [String]  # List of strings
    scores   [Int]     # List of integers
    metadata {String:String}  # Map of string to string
}
```

#### Links and References
```dasl
type NetworkedUser struct {
    profile  &Profile  # Link to Profile object
    friends  [&User]   # List of User links
    posts    [&Post]   # List of Post links
}
```

#### Union Types
```dasl
type Content union {
    | TextPost   "text"
    | ImagePost  "image"
    | VideoPost  "video"
} representation keyed
```

### Go Struct Mapping

#### IPLD Tags
Map DASL fields to Go struct fields:
```go
type User struct {
    ID       string   `ipld:"id"`        # Required mapping
    Name     string   `ipld:"name"`      # Field name mapping
    IsActive bool     `ipld:"active"`    # Type conversion
    Tags     []string `ipld:"tags"`      # Collection mapping
}
```

#### Link Handling
Links in DASL become `cid.Cid` in Go:
```dasl
# DASL
type Post struct {
    author &User
}
```

```go
// Go
type Post struct {
    Author cid.Cid `ipld:"author"`
}
```

## 🧪 Testing Guide

The module includes comprehensive tests demonstrating:

1. **Schema Compilation**: Loading and compiling DASL schemas
2. **Type-Safe Operations**: Putting and getting with validation
3. **Link Resolution**: Working with CID references
4. **Data Integrity**: Round-trip data consistency
5. **Error Handling**: Schema validation failures

### Key Test Scenarios

```go
func TestDaslWrapperPutGet(t *testing.T) {
    // 1. Create schema wrapper
    wrapper, err := dasl.NewDaslWrapper(nil)

    // 2. Create linked data structure
    user := &dasl.User{...}
    userCID, _ := wrapper.PutUser(ctx, user)

    post := &dasl.Post{
        Author: userCID,  // Link to user
        ...
    }
    postCID, _ := wrapper.PutPost(ctx, post)

    // 3. Verify data integrity
    retrievedPost, _ := wrapper.GetPost(ctx, postCID)
    assert.Equal(t, userCID, retrievedPost.Author)
}
```

## 🔍 Troubleshooting

### Common Issues

1. **Schema Compilation Errors**
   ```
   Error: schema parse: type "User" not found
   Solution: Check DASL syntax and ensure all referenced types are defined
   ```

2. **Type Assertion Failures**
   ```
   Error: unwrap User: type assertion to *User failed
   Solution: Verify Go struct matches DASL schema exactly
   ```

3. **IPLD Tag Mismatches**
   ```
   Error: field "name" not found in schema
   Solution: Ensure ipld tags match DASL field names
   ```

4. **Link Resolution Issues**
   ```
   Error: invalid link: not a valid CID
   Solution: Ensure CID fields contain valid content identifiers
   ```

### Debugging Tips

- Use `bindnode.Prototype()` to inspect node prototypes
- Validate DASL syntax with `schemadsl.ParseBytes()`
- Check type system compilation with `schemadmt.Compile()`
- Use schema type inspection: `ts.TypeByName("TypeName")`

## 📊 Performance Benefits

### Schema-Based Advantages

1. **Validation**: Early error detection during serialization
2. **Optimization**: Efficient encoding/decoding paths
3. **Type Safety**: Compile-time error prevention
4. **Documentation**: Self-documenting data structures
5. **Tooling**: Code generation and IDE support

### Benchmarking
```go
// Schema-based operations are typically faster due to:
// - Pre-compiled type information
// - Optimized serialization paths
// - Reduced runtime type checking
```

## 📚 Next Steps

### Immediate Next Steps
With DASL schema expertise, advance to these specialized areas:

1. **[14-traversal-selector](../14-traversal-selector)**: Schema-Aware Navigation
   - Apply schema knowledge to sophisticated traversal patterns
   - Build efficient selector systems with type validation
   - Master schema-guided data exploration techniques

2. **Advanced Schema Applications**: Choose your specialization:
   - **[15-graphsync](../15-graphsync)**: Schema-validated network synchronization
   - **[17-ipni](../17-ipni)**: Schema-based content indexing systems

### Related Modules
**Prerequisites (Essential foundation):**
- [12-ipld-prime](../12-ipld-prime): IPLD-prime operations and datamodel.Node
- [05-dag-ipld](../05-dag-ipld): Basic IPLD concepts and DAG structures
- [00-block-cid](../00-block-cid): Content addressing fundamentals

**Advanced Applications:**
- [15-graphsync](../15-graphsync): Network sync with schema validation
- [16-trustless-gateway](../16-trustless-gateway): Schema-aware gateway operations
- [17-ipni](../17-ipni): Schema-based indexing and discovery
- [18-multifetcher](../18-multifetcher): Multi-source fetching with schema support

### Alternative Learning Paths

**For Type-Safe Development:**
13-dasl → Custom Schema Projects → 14-traversal-selector → Production Applications

**For Network Protocol Focus:**
13-dasl → 14-traversal-selector → 15-graphsync → Distributed Systems

**For Data Architecture Focus:**
13-dasl → 17-ipni → Schema-based Indexing Systems → Enterprise Solutions

**For Web Integration:**
13-dasl → 16-trustless-gateway → Schema-validated Web APIs

## 📚 Further Reading

- [IPLD Schema Specification](https://ipld.io/docs/schemas/)
- [DASL Reference](https://ipld.io/docs/schemas/dsl/)
- [Bindnode Documentation](https://pkg.go.dev/github.com/ipld/go-ipld-prime/node/bindnode)
- [Schema DMT](https://pkg.go.dev/github.com/ipld/go-ipld-prime/schema/dmt)
- [Code Generation Guide](https://github.com/ipld/go-ipld-prime/tree/master/schema)

---

This module demonstrates how IPLD schemas provide type safety, validation, and code generation for robust content-addressed data structures. Master these patterns to build reliable, typed data systems with IPFS and IPLD.