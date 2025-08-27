# walpb Package

The `walpb` package provides Protocol Buffers definitions and utilities for etcd's **Write-Ahead Log (WAL)** functionality. This package handles the serialization format for WAL records and snapshots used in etcd's persistent storage.

## 📁 File Overview

### Core Files

| File | Type | Purpose | Edit Policy |
|------|------|---------|-------------|
| `record.proto` | Protocol Buffers Definition | Defines the schema for WAL records and snapshots | ✅ Manual edits |
| `record.pb.go` | Generated Go Code | Auto-generated structs and serialization methods | ❌ **DO NOT EDIT** |
| `record.go` | Business Logic | Custom validation and utility methods | ✅ Manual edits |

---

## 📋 Detailed File Descriptions

### `record.proto`
**Protocol Buffers schema definition**

- **Messages Defined:**
  - `Record`: Core WAL entry with type, CRC, and data fields
  - `Snapshot`: Snapshot metadata with index, term, and configuration state

- **Key Features:**
  - Uses `proto2` syntax for backward compatibility
  - Includes GoGoProto optimizations for performance
  - Imports raftpb for Raft consensus types

- **GoGoProto Options:**
  ```proto
  option (gogoproto.marshaler_all) = true;      // Custom marshal methods
  option (gogoproto.sizer_all) = true;          // Size calculation methods
  option (gogoproto.unmarshaler_all) = true;    // Custom unmarshal methods
  option (gogoproto.goproto_getters_all) = false; // Disable getter methods