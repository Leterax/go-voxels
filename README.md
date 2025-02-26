# Go-Voxels

A high-performance OpenGL 4.6 renderer demo showcasing thousands of animated 3D cubes using persistent buffer mapping in Go.

![Go-Voxels Demo](assets/images/demo.png)

## Features

- Render 50,000+ 3D cubes with real-time animation
- Advanced OpenGL techniques with persistent buffer mapping for optimal performance
- Physics-based animation with bouncing behavior
- Phong lighting model with ambient, diffuse, and specular components
- Dynamic updates with partial buffer processing to maintain smooth framerates

## Project Structure

- `cmd/voxels`: Main application entry point
- `pkg/render`: Rendering engine and camera system
- `internal/openglhelper`: OpenGL abstractions for buffers, shaders, and window management

## Performance

This demo showcases high-performance rendering techniques:

- Persistent buffer mapping for direct GPU memory access
- Partial buffer updates (only updating a subset of cubes each frame)
- Single draw call for all cubes with indexed rendering
- Backface culling optimization
- Efficient memory management with unsafe pointer manipulation

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

```bash
./voxels
```

## Controls

- **W/A/S/D**: Move camera in the horizontal plane
- **Space/Shift**: Move camera up/down
- **Mouse**: Look around (when mouse is captured)
- **C**: Toggle mouse capture mode
- **Escape**: Exit the application

## Configuration

You can modify the rendering parameters in `pkg/render/renderer.go`:

```go
renderer := &Renderer{
    window:             window,
    camera:             camera,
    numCubes:           50000, // Total number of cubes to render
    cubesPerUpdate:     5000,  // Number of cubes to update each frame
    currentUpdateIndex: 0,
}
```

Adjust these values based on your hardware capabilities:
- For higher-end systems, increase `numCubes` for more visual density
- For lower-end systems, decrease `numCubes` and/or `cubesPerUpdate` for better performance

## Dependencies

This project uses the following external libraries:

- [go-gl/gl](https://github.com/go-gl/gl): Go bindings for OpenGL 4.6
- [go-gl/glfw](https://github.com/go-gl/glfw): Go bindings for GLFW (window management)
- [go-gl/mathgl](https://github.com/go-gl/mathgl): Go math library for OpenGL

## How It Works

The demo uses a persistently mapped OpenGL buffer to directly write vertex data to GPU memory, avoiding the overhead of traditional buffer updates. Each frame:

1. A subset of cubes are updated (positions, velocities)
2. The updated data is written directly to the mapped GPU memory
3. All cubes are rendered in a single draw call using indexed rendering
4. FPS and performance statistics are displayed

This approach provides much higher performance than traditional buffer update methods, especially when rendering many thousands of moving objects.

## License

[MIT License](LICENSE) 