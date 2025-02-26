package render

import (
	"fmt"
	"math"
	"math/rand"
	"openglhelper"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Renderer handles rendering logic and game loop
type Renderer struct {
	window *openglhelper.Window
	camera *Camera

	cubeShader *openglhelper.Shader

	// Timing
	lastFrameTime float64
	deltaTime     float32
	totalTime     float32

	// FPS tracking
	frameCount int
	totalFPS   float32

	// Cube rendering data
	numCubes           int
	cubesPerUpdate     int
	currentUpdateIndex int

	// Cube mesh data
	cubeVAO *openglhelper.VertexArrayObject
	cubeVBO *openglhelper.BufferObject
	cubeEBO *openglhelper.BufferObject

	// Cube instance data
	positions  []mgl32.Vec3
	velocities []mgl32.Vec3
	colors     []mgl32.Vec3

	// Persistent buffer mapping
	mappedArray []float32
	vertexData  []float32
}

// NewRenderer creates a new renderer with the specified dimensions and title
func NewRenderer(width, height int, title string) (*Renderer, error) {
	// Create window
	window, err := openglhelper.NewWindow(width, height, title, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	// Create initial camera position
	cameraPos := mgl32.Vec3{0, 0, 25} // Start farther back to see more cubes
	camera := NewCamera(cameraPos)

	camera.LookAt(mgl32.Vec3{0, 0, 0})

	renderer := &Renderer{
		window:             window,
		camera:             camera,
		numCubes:           50000, // 50,000 cubes
		cubesPerUpdate:     5000,  // Update 5,000 cubes per frame
		currentUpdateIndex: 0,
	}

	// Set up callbacks
	window.GLFWWindow().SetKeyCallback(renderer.keyCallback)
	window.GLFWWindow().SetCursorPosCallback(renderer.cursorPosCallback)
	window.GLFWWindow().SetMouseButtonCallback(renderer.mouseButtonCallback)
	window.GLFWWindow().SetScrollCallback(renderer.scrollCallback)
	window.GLFWWindow().SetFramebufferSizeCallback(renderer.framebufferSizeCallback)

	// Print control instructions
	fmt.Println("Control instructions:")
	fmt.Println("- WASD: Camera movement")
	fmt.Println("- Mouse: Look around")
	fmt.Println("- Space/Shift: Move up/down")
	fmt.Println("- C: Toggle mouse capture")
	fmt.Println("- Escape: Close the application")

	fmt.Println("\nLoading shaders...")
	shader, err := openglhelper.LoadShaderFromFiles("pkg/render/shaders/vert.glsl", "pkg/render/shaders/frag.glsl")
	if err != nil {
		return nil, fmt.Errorf("failed to load shader: %w", err)
	}
	fmt.Println("Shader loaded successfully!")
	fmt.Println("Shader ID:", shader.ID)

	renderer.cubeShader = shader

	// Initialize the cube rendering system
	if err := renderer.initCubeRenderSystem(); err != nil {
		return nil, fmt.Errorf("failed to initialize cube render system: %w", err)
	}

	return renderer, nil
}

// initCubeRenderSystem sets up the cube geometry and persistent buffer
func (r *Renderer) initCubeRenderSystem() error {
	// Define cube vertices - each vertex contains position, normal and texture coordinates
	// Format: x, y, z, nx, ny, nz, tx, ty
	baseCubeVertices := []float32{
		// Front face
		-0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 0.0, 0.0, // Bottom-left
		0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 0.0, // Bottom-right
		0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 0.0, 1.0, // Top-left

		// Back face
		-0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 0.0, // Bottom-left
		0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 0.0, 0.0, // Bottom-right
		0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 0.0, 1.0, // Top-right
		-0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, // Top-left

		// Left face
		-0.5, 0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, -0.5, -1.0, 0.0, 0.0, 0.0, 1.0, // Top-left
		-0.5, -0.5, -0.5, -1.0, 0.0, 0.0, 0.0, 0.0, // Bottom-left
		-0.5, -0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 0.0, // Bottom-right

		// Right face
		0.5, 0.5, 0.5, 1.0, 0.0, 0.0, 0.0, 1.0, // Top-left
		0.5, 0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 1.0, // Top-right
		0.5, -0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 0.0, // Bottom-right
		0.5, -0.5, 0.5, 1.0, 0.0, 0.0, 0.0, 0.0, // Bottom-left

		// Bottom face
		-0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 0.0, 0.0, // Bottom-left
		0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 1.0, 0.0, // Bottom-right
		0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 1.0, 1.0, // Top-right
		-0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 0.0, 1.0, // Top-left

		// Top face
		-0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 0.0, 1.0, // Top-left
		0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 1.0, 1.0, // Top-right
		0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 1.0, 0.0, // Bottom-right
		-0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 0.0, 0.0, // Bottom-left
	}

	// Define indices for a single cube
	singleCubeIndices := []uint32{
		0, 1, 2, 2, 3, 0, // Front face
		4, 5, 6, 6, 7, 4, // Back face
		8, 9, 10, 10, 11, 8, // Left face
		12, 13, 14, 14, 15, 12, // Right face
		16, 17, 18, 18, 19, 16, // Bottom face
		20, 21, 22, 22, 23, 20, // Top face
	}

	// Create indices for all cubes
	allIndices := make([]uint32, len(singleCubeIndices)*r.numCubes)
	for i := 0; i < r.numCubes; i++ {
		baseVertex := uint32(i * 24) // 24 vertices per cube
		for j := 0; j < len(singleCubeIndices); j++ {
			allIndices[i*len(singleCubeIndices)+j] = singleCubeIndices[j] + baseVertex
		}
	}

	// Create a VAO for the cube
	r.cubeVAO = openglhelper.NewVAO()
	r.cubeVAO.Bind()

	// Create a temporary static VBO for vertex setup - we'll replace this with the persistent one
	tempVBO := openglhelper.NewVBO(baseCubeVertices, openglhelper.StaticDraw)

	// Create an EBO for all the cube indices
	r.cubeEBO = openglhelper.NewEBO(allIndices, openglhelper.StaticDraw)

	// Set up vertex attributes
	// Position attribute
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 8*4, 0)
	// Normal attribute
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 8*4, 3*4)
	// Texture coordinate attribute
	r.cubeVAO.SetVertexAttribPointer(2, 2, gl.FLOAT, false, 8*4, 6*4)

	// Unbind temporary VBO (we don't need it anymore)
	tempVBO.Unbind()
	tempVBO.Delete()

	// Initialize cube positions and other data
	r.initializeCubeData()

	// Create memory mapped buffer for the cube data
	fmt.Println("Creating persistent buffer for", r.numCubes, "cubes...")
	vertexDataSize := r.numCubes * 24 * 8 * 4 // numCubes * vertices per cube * floats per vertex * bytes per float

	// Initialize vertex data to be mapped
	r.vertexData = make([]float32, r.numCubes*24*8) // Preallocate a large slice for all cube data

	// Copy the base cube vertices to each cube's vertices section
	for i := 0; i < r.numCubes; i++ {
		for v := 0; v < 24; v++ {
			for j := 0; j < 8; j++ {
				r.vertexData[i*24*8+v*8+j] = baseCubeVertices[v*8+j]
			}
		}
	}

	for i := 0; i < r.numCubes; i++ {
		// Generate transformed vertices for this cube
		r.updateCubeVertices(i)
	}

	fmt.Println("Initializing persistent mapped buffer...")
	mappedVBO, err := openglhelper.NewPersistentBuffer(gl.ARRAY_BUFFER, vertexDataSize, false, true)
	if err != nil {
		return fmt.Errorf("failed to create persistent buffer: %w", err)
	}

	// Store the VBO for later use
	r.cubeVBO = mappedVBO

	// Bind the persistent VBO to the VAO
	r.cubeVAO.Bind()
	r.cubeVBO.Bind()

	// Re-specify the vertex attributes on our new persistent buffer
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 8*4, 0)
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 8*4, 3*4)
	r.cubeVAO.SetVertexAttribPointer(2, 2, gl.FLOAT, false, 8*4, 6*4)

	// Get the mapped pointer and create a slice wrapper
	mappedPtr := r.cubeVBO.GetMappedPointer()
	if mappedPtr == nil {
		return fmt.Errorf("failed to get mapped pointer")
	}

	// Create a slice that maps to the persistent buffer
	r.mappedArray = unsafe.Slice((*float32)(mappedPtr), r.numCubes*24*8)

	// Initialize the mapped buffer with the vertex data
	copy(r.mappedArray, r.vertexData)

	r.cubeVAO.Unbind()

	fmt.Println("Cube render system initialized successfully!")
	fmt.Printf("Rendering %d cubes with persistent buffer mapping\n", r.numCubes)

	return nil
}

// initializeCubeData sets up the initial positions, velocities, and colors for all cubes
func (r *Renderer) initializeCubeData() {
	// Initialize positions, velocities, and colors
	r.positions = make([]mgl32.Vec3, r.numCubes)
	r.velocities = make([]mgl32.Vec3, r.numCubes)
	r.colors = make([]mgl32.Vec3, r.numCubes)

	// Create cubes in a volume
	bounds := float32(30.0) // Distribute cubes within this boundary

	for i := 0; i < r.numCubes; i++ {
		// Random position within the bounds
		r.positions[i] = mgl32.Vec3{
			(rand.Float32()*2.0 - 1.0) * bounds,
			(rand.Float32()*2.0 - 1.0) * bounds,
			(rand.Float32()*2.0 - 1.0) * bounds,
		}

		// Random velocity
		r.velocities[i] = mgl32.Vec3{
			(rand.Float32()*2.0 - 1.0) * 0.1,
			(rand.Float32()*2.0 - 1.0) * 0.1,
			(rand.Float32()*2.0 - 1.0) * 0.1,
		}

		// Random color (bright)
		r.colors[i] = mgl32.Vec3{
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
		}
	}
}

// updateCubeVertices updates the vertex data for a specific cube
func (r *Renderer) updateCubeVertices(cubeIndex int) {
	pos := r.positions[cubeIndex]
	color := r.colors[cubeIndex]

	// Each cube has 24 vertices (4 per face * 6 faces)
	// Each vertex has 8 components: position (3), normal (3), and texture coords (2)
	baseIndex := cubeIndex * 24 * 8

	// For each vertex of the cube
	for v := 0; v < 24; v++ {
		vertexBase := baseIndex + v*8

		// Get the base cube vertex
		baseX := r.vertexData[vertexBase]
		baseY := r.vertexData[vertexBase+1]
		baseZ := r.vertexData[vertexBase+2]

		// Update position (offset by cube position)
		r.vertexData[vertexBase] = baseX + pos[0]
		r.vertexData[vertexBase+1] = baseY + pos[1]
		r.vertexData[vertexBase+2] = baseZ + pos[2]

		// Normals stay the same

		// Could add color as texture coordinates
		r.vertexData[vertexBase+6] = color[0] // s
		r.vertexData[vertexBase+7] = color[1] // t
	}
}

// updateCubes updates a subset of cubes each frame
func (r *Renderer) updateCubes() {
	startIdx := r.currentUpdateIndex
	endIdx := startIdx + r.cubesPerUpdate
	if endIdx > r.numCubes {
		endIdx = r.numCubes
	}

	// Update cube positions
	bounds := float32(30.0)

	for i := startIdx; i < endIdx; i++ {
		// Update position
		r.positions[i] = r.positions[i].Add(r.velocities[i])

		// Bounce off boundaries
		for j := 0; j < 3; j++ {
			if math.Abs(float64(r.positions[i][j])) > float64(bounds) {
				r.velocities[i][j] = -r.velocities[i][j]
			}
		}

		// Update the vertex data for this cube
		r.updateCubeVertices(i)

		// Copy data to mapped buffer
		baseIndex := i * 24 * 8
		for j := 0; j < 24*8; j++ {
			r.mappedArray[baseIndex+j] = r.vertexData[baseIndex+j]
		}
	}

	// Move to the next block of cubes
	r.currentUpdateIndex = endIdx % r.numCubes
}

// render is where you implement your rendering logic
func (r *Renderer) render(totalTime float32, deltaTime float32) {
	// Get view and projection matrices from camera
	view := r.camera.ViewMatrix()
	projection := r.camera.ProjectionMatrix()

	// Set up the shader with the matrices
	r.cubeShader.Use()
	r.cubeShader.SetMat4("view", view)
	r.cubeShader.SetMat4("projection", projection)

	// Always update camera position uniform for correct specular highlights
	r.cubeShader.SetVec3("viewPos", r.camera.Position())

	// Update a block of cubes each frame
	r.updateCubes()

	// Debugging - Print visual confirmation once that the shader is running
	if r.totalTime < 0.1 { // Only print once at the start
		fmt.Println("\nStarting rendering...")
		fmt.Println("Using shader ID:", r.cubeShader.ID)
		fmt.Println("Rendering", r.numCubes, "cubes with persistent mapping")
	}

	// Set up lighting
	lightPos := mgl32.Vec3{30.0, 30.0, 30.0}
	r.cubeShader.SetVec3("lightPos", lightPos)
	r.cubeShader.SetVec3("lightColor", mgl32.Vec3{1.0, 1.0, 1.0})

	// Bind the VAO
	r.cubeVAO.Bind()

	// Enable backface culling for better performance
	gl.Enable(gl.CULL_FACE)

	// Set model matrix (identity since we're transforming vertices directly)
	model := mgl32.Ident4()
	r.cubeShader.SetMat4("model", model)

	// Draw all cubes in a single draw call using the index buffer that references all cubes
	// We have 36 indices per cube (6 faces * 2 triangles * 3 vertices)
	gl.DrawElements(gl.TRIANGLES, int32(36*r.numCubes), gl.UNSIGNED_INT, gl.PtrOffset(0))

	// Disable backface culling
	gl.Disable(gl.CULL_FACE)

	// Unbind VAO
	r.cubeVAO.Unbind()

	// Print frame completion message once
	if r.totalTime < 0.05 {
		fmt.Println("First frame rendered")
	}

	// Print FPS every second
	if int(r.totalTime) > int(r.totalTime-r.deltaTime) {
		currentFPS := 1.0 / r.deltaTime
		fmt.Printf("FPS: %.1f | Cubes: %d | Updated: %d\n", currentFPS, r.numCubes, r.cubesPerUpdate)
	}
}

// Run starts the main rendering loop
func (r *Renderer) Run() {
	// Main loop
	for !r.window.ShouldClose() {
		// Calculate delta time
		currentTime := glfw.GetTime()
		r.deltaTime = float32(currentTime - r.lastFrameTime)
		r.lastFrameTime = currentTime
		r.totalTime += r.deltaTime

		// Process camera input
		r.camera.ProcessKeyboardInput(r.deltaTime, r.window)

		// Clear the screen
		r.window.Clear()

		// Render frame
		r.render(r.totalTime, r.deltaTime)

		// Swap buffers and poll events
		r.window.SwapBuffers()
		r.window.PollEvents()

		// Track FPS for average calculation
		if r.deltaTime > 0 {
			currentFPS := 1.0 / r.deltaTime
			r.totalFPS += currentFPS
			r.frameCount++
		}
	}

	// Calculate and display average FPS before cleanup
	if r.frameCount > 0 {
		avgFPS := r.totalFPS / float32(r.frameCount)
		fmt.Printf("\nApplication closing. Average FPS: %.1f over %d frames\n", avgFPS, r.frameCount)
	}

	// Cleanup
	r.Cleanup()
}

func (r *Renderer) Cleanup() {
	// Clean up resources
	r.cubeVBO.Unmap()
	r.cubeVBO.Delete()
	r.cubeEBO.Delete()
	r.cubeVAO.Delete()

	r.window.Close()
}

// Callback functions
func (r *Renderer) keyCallback(window *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	// Handle key presses
	if key == glfw.KeyEscape && action == glfw.Press {
		r.window.GLFWWindow().SetShouldClose(true)
	}

	// Toggle mouse capture with C key
	if key == glfw.KeyC && action == glfw.Press {
		r.window.ToggleMouseCaptured()
		r.camera.ResetMouseState()
	}
}

func (r *Renderer) cursorPosCallback(_ *glfw.Window, xpos, ypos float64) {
	if r.window.IsMouseCaptured() {
		r.camera.HandleMouseMovement(xpos, ypos)
	}
}

func (r *Renderer) mouseButtonCallback(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
	// Handle mouse button events
}

func (r *Renderer) scrollCallback(_ *glfw.Window, xoffset, yoffset float64) {
	r.camera.HandleMouseScroll(yoffset)
}

func (r *Renderer) framebufferSizeCallback(_ *glfw.Window, width, height int) {
	r.window.OnResize(width, height)
	r.camera.UpdateProjectionMatrix(width, height)
}
