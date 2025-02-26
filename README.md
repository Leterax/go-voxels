# Go-Voxels

A simple OpenGL 4.6 window with basic input handling using Go.

## Project Structure

- `cmd/voxels`: Main application entry point
- `pkg/render`: OpenGL and GLFW window handling

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

- W: Move forward
- A: Move left
- S: Move backward
- D: Move right
- Space: Jump
- Escape: Exit the application

## Dependencies

This project uses the following external libraries:

- [go-gl/gl](https://github.com/go-gl/gl): Go bindings for OpenGL 4.6
- [go-gl/glfw](https://github.com/go-gl/glfw): Go bindings for GLFW (window management)

## License

[MIT License](LICENSE) 