# Go-Voxels

A voxel-based game engine with efficient OpenGL rendering, multiplayer networking, and optimized chunk management written in Go.

![Go-Voxels Demo](assets/images/demo.png)

## Features

- **Optimized Voxel Rendering**
  - Greedy meshing algorithm for efficient triangle reduction
  - Persistent buffer mapping for optimal GPU performance
  - Multi-draw indirect rendering of thousands of chunks
  - Coordinate system utilities for seamless world management

- **Block System**
  - Support for various block types with different properties
  - Specialized rendering for mono-type chunks
  - Block property system (solidity, transparency)

- **Chunked World**
  - Dynamic chunk loading and unloading
  - Efficient storage of chunk data
  - Thread-safe chunk management

- **Multiplayer Support**
  - Client/server architecture
  - Network synchronization of chunks and entities
  - Efficient mono-chunk transmission

## Project Structure

- `cmd/voxels`: Main application entry point
- `pkg/game`: Game logic and chunk management
- `pkg/voxel`: Core voxel engine (blocks, chunks, mesh generation)
- `pkg/render`: OpenGL rendering system
- `pkg/network`: Multiplayer networking
- `internal/openglhelper`: OpenGL abstractions

## Getting Started

### Prerequisites

- Go 1.18 or higher
- OpenGL 4.6 compatible graphics card and drivers
- GLFW dependencies (varies by platform)

### Building

```bash
go build -o voxels ./cmd/voxels
```

### Running

#### Singleplayer Mode
```bash
./voxels
```

#### Multiplayer Mode
```bash
./voxels -server <server-address> -name <player-name> -renderdist <chunk-render-distance>
```

## Controls

- **W/A/S/D**: Move camera in the horizontal plane
- **Space/Shift**: Move camera up/down
- **Mouse**: Look around
- **ESC**: Exit the application

## Configuration

Command-line options:
- `-server`: Server address (empty for singleplayer)
- `-name`: Player name for multiplayer
- `-renderdist`: Render distance in chunks (default: 8)

## Technical Details

### Voxel Rendering

The engine uses several optimizations for efficient voxel rendering:

1. **Greedy Meshing**: Combines adjacent faces of the same block type to reduce triangle count
2. **Packed Vertices**: Compresses vertex data into a single 32-bit integer to reduce memory usage
3. **Mono-Chunk Optimization**: Special fast-path for chunks containing only one block type
4. **Coordinate System Utilities**: Consistent handling of chunk, world, and local coordinates

### Chunk Management

Chunks are managed efficiently with:

1. **Thread-safe Access**: Mutex-protected operations for concurrent chunk processing
2. **Background Processing**: Separate worker goroutine for mesh generation
3. **Change Detection**: Smart tracking of chunk changes to minimize GPU updates

## Dependencies

- [go-gl/gl](https://github.com/go-gl/gl): Go bindings for OpenGL
- [go-gl/glfw](https://github.com/go-gl/glfw): Window management
- [go-gl/mathgl](https://github.com/go-gl/mathgl): OpenGL math library

## License

[MIT License](LICENSE)

## Thread Safety

### OpenGL Thread Safety

All OpenGL operations must be performed on the main thread where the OpenGL context was created. The renderer implements a command queue system to ensure thread safety:

1. The `ExecuteOnMainThread` method allows scheduling functions to run on the main OpenGL thread
2. The `ProcessCommandQueueWithTimeout` method is called in the main render loop to execute these functions with a time budget to prevent frame stuttering
3. Thread-safe wrapper methods (e.g., `ThreadSafeAddDrawCommand`, `SafelyUpdateBufferData`) are provided for operations that need to be performed from background goroutines

For optimal performance, the renderer implements several advanced techniques:

- **Triple Buffering**: Using `SafelyUpdateTripleBuffer` to ensure the GPU and CPU don't access the same buffer segment simultaneously
- **Command Batching**: `BatchCommands` method reduces overhead by executing multiple commands in a single queue operation
- **Fence Synchronization**: Proper fence sync objects are used to synchronize GPU and CPU access to shared resources

Background processing (like chunk generation) should use these methods when interacting with OpenGL resources to avoid segmentation faults. 