## Information
- Use **BIG ENDIAN** to communicate with the server
- Server doesn't send empty chunk
- TCP uses port **20000**
- Chunk size is **16**
- Chunks are cubic

## Current Protocol

### Client bound

Identification: `0x00`
| id   | entityId |
|------|----------|
| U8   | U32      |

Add Entity: `0x01`
| id   | entityId | x     | y     | z     | yaw   | pitch | name     |
|------|----------|-------|-------|-------|-------|-------|----------|
| U8   | U32      | F32   | F32   | F32   | F32   | F32   | U8[64]   |

Remove Entity: `0x02`
| id   | entityId |
|------|----------|
| U8   | U32      |


Update Entity Position: `0x03`
| id   | entityId | x     | y     | z     | yaw   | pitch |
|------|----------|-------|-------|-------|-------|-------|
| U8   | U32      | F32   | F32   | F32   | F32   | F32   |

Send Chunk: `0x04`
| id   | x   | y   | z   | BlockType       |
|------|-----|-----|-----|-----------------|
| U8   | I32 | I32 | I32 | U8[CHUNK_SIZE^3] |

Send Mono Type Chunk: `0x05`
| id   | x   | y   | z   | BlockType |
|------|-----|-----|-----|-----------|
| U8   | I32 | I32 | I32 | U8        |

Chat: `0x06`
| id   | message    |
|------|------------|
| U8   | U8[4096]   |

Update Entity Metadata: `0x07`
| id   | entityId | name     |
|------|----------|----------|
| U8   | U32      | U8[64]   |

### Server bound
Update Entity: `0x00`
| id   | x     | y     | z     | yaw   | pitch |
|------|-------|-------|-------|-------|-------|
| U8   | F32   | F32   | F32   | F32   | F32   |

Update Block: `0x01`
| id   | BlockType | x   | y   | z   |
|------|-----------|-----|-----|-----|
| U8   | U8        | I32 | I32 | I32 |

Block Bulk Edit: `0x02`
| id   | blockCount | BlockType | x   | y   | z   | BlockType | x   | y   | z   | ... |
|------|------------|-----------|-----|-----|-----|-----------|-----|-----|-----|-----|
| U8   | U32        | U8        | I32 | I32 | I32 | U8        | I32 | I32 | I32 | ... |

Chat: `0x03`
| id   | message    |
|------|------------|
| U8   | U8[4096]   |

Client metadata: `0x04`
| id   | renderDistance | name       |
|------|----------------|------------|
| U8   | U8             | U8[64]     |


### BlockType
| id | Name         |
|----|--------------|
| 0  | Air          |
| 1  | Grass        |
| 2  | Dirt         |
| 3  | Stone        |
| 4  | Oak Log      |
| 5  | Oak Leaves   |
| 6  | Glass        |
| 7  | Water        |
| 8  | Sand         |
| 9  | Snow         |
| 10 | Oak Planks   |
| 11 | Stone Bricks |
| 12 | Netherrack   |
| 13 | Gold Block   |
| 14 | Packed Ice   |
| 15 | Lava         |
| 16 | Barrel       |
| 17 | Bookshelf    |
