package render

import (
	"fmt"
	"math"
	"math/rand"
	"openglhelper"
	"sync"
	"time"
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

	// Cube rendering data
	numCubes          int
	cubesPerBatch     int // How many cubes to update per frame
	currentBatchIndex int // Current batch index being updated
	colorCycleTime    float32

	// Cube mesh data
	cubeVAO *openglhelper.VertexArrayObject
	cubeVBO *openglhelper.BufferObject
	cubeEBO *openglhelper.BufferObject

	// Cube instance data
	positions  []mgl32.Vec3
	velocities []mgl32.Vec3
	colors     []mgl32.Vec3
	colorPhase []float32 // Phase offset for color cycling

	// Persistent buffer mapping with triple buffering
	mappedMemory     []float32 // Go slice backed by persistent mapped memory
	bufferSize       int       // Size of each buffer section in bytes
	numBuffers       int       // Number of buffer sections (3 for triple buffering)
	currentBufferIdx int       // Index of the buffer section currently being written to
	bufferOffsets    []int     // Offsets for each buffer section (in float32 units)
	syncObjects      []uintptr // Fence sync objects for each buffer section

	// Single thread update control
	updateMutex        sync.Mutex
	updatedBufferIdx   int
	updateQuitChan     chan struct{}
	isUpdateThreadBusy bool
	lastUpdateTime     time.Time
	updateBufferIdx    int
	isClosed           bool
}

// NewRenderer creates a new renderer with the specified dimensions and title
func NewRenderer(width, height int, title string) (*Renderer, error) {
	// Create window
	window, err := openglhelper.NewWindow(width, height, title, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	// Create camera
	cameraPos := mgl32.Vec3{0, 0, 25}
	camera := NewCamera(cameraPos)
	camera.LookAt(mgl32.Vec3{0, 0, 0})

	renderer := &Renderer{
		window:            window,
		camera:            camera,
		numCubes:          50000, // 50,000 cubes
		cubesPerBatch:     5000,  // Update 5,000 cubes per frame (10% of total)
		currentBatchIndex: 0,     // Start with first batch
		numBuffers:        3,     // Triple buffering
		currentBufferIdx:  0,
		updatedBufferIdx:  -1,  // No buffer updated yet
		colorCycleTime:    5.0, // 5 seconds for a complete color cycle

		// Initialize update control
		updateQuitChan:     make(chan struct{}),
		isUpdateThreadBusy: false,
	}

	// Set up callbacks
	window.GLFWWindow().SetKeyCallback(renderer.keyCallback)
	window.GLFWWindow().SetCursorPosCallback(renderer.cursorPosCallback)
	window.GLFWWindow().SetMouseButtonCallback(renderer.mouseButtonCallback)
	window.GLFWWindow().SetScrollCallback(renderer.scrollCallback)
	window.GLFWWindow().SetFramebufferSizeCallback(renderer.framebufferSizeCallback)

	// Load shader
	shader, err := openglhelper.LoadShaderFromFiles("pkg/render/shaders/vert.glsl", "pkg/render/shaders/frag.glsl")
	if err != nil {
		return nil, fmt.Errorf("failed to load shader: %w", err)
	}
	renderer.cubeShader = shader

	// Initialize cube rendering system
	if err := renderer.initCubeRenderSystem(); err != nil {
		return nil, fmt.Errorf("failed to initialize cube render system: %w", err)
	}

	// Start the update thread that will handle color updates
	go renderer.updateThread()

	return renderer, nil
}

// initCubeRenderSystem sets up the cube geometry and persistent buffer
func (r *Renderer) initCubeRenderSystem() error {
	// Define cube vertices - each vertex contains position, normal and color
	// Format: x, y, z, nx, ny, nz, r, g, b
	baseCubeVertices := []float32{
		// Front face
		-0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, 1.0, // Bottom-left
		0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, 1.0, // Bottom-right
		0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, 1.0, // Top-left

		// Back face
		-0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, 1.0, // Bottom-left
		0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, 1.0, // Bottom-right
		0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, 1.0, // Top-left

		// Left face
		-0.5, 0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, -0.5, -1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-left
		-0.5, -0.5, -0.5, -1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Bottom-left
		-0.5, -0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Bottom-right

		// Right face
		0.5, 0.5, 0.5, 1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-left
		0.5, 0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		0.5, -0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Bottom-right
		0.5, -0.5, 0.5, 1.0, 0.0, 0.0, 1.0, 1.0, 1.0, // Bottom-left

		// Bottom face
		-0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 1.0, 1.0, 1.0, // Bottom-left
		0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 1.0, 1.0, 1.0, // Bottom-right
		0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 1.0, 1.0, 1.0, // Top-left

		// Top face
		-0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 1.0, 1.0, 1.0, // Top-left
		0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 1.0, 1.0, 1.0, // Bottom-right
		-0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 1.0, 1.0, 1.0, // Bottom-left
	}

	// Create indices for all cubes
	indices := r.createCubeIndices()

	// Create VAO and EBO
	r.cubeVAO = openglhelper.NewVAO()
	r.cubeVAO.Bind()

	// Create a temporary static VBO for vertex setup - we'll replace this with the persistent one
	tempVBO := openglhelper.NewVBO(baseCubeVertices, openglhelper.StaticDraw)

	// Create an EBO for all the cube indices
	r.cubeEBO = openglhelper.NewEBO(indices, openglhelper.StaticDraw)

	// Set up vertex attributes
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 9*4, 0)
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 9*4, 3*4)
	r.cubeVAO.SetVertexAttribPointer(2, 3, gl.FLOAT, false, 9*4, 6*4)

	// Unbind temporary VBO (we don't need it anymore)
	tempVBO.Unbind()
	tempVBO.Delete()

	// Initialize cube data (positions, colors)
	r.initializeCubeData()

	// Calculate buffer sizes and offsets
	floatsPerVertex := 9  // 3 position + 3 normal + 3 color
	verticesPerCube := 24 // 6 faces * 4 vertices per face
	floatsPerCube := floatsPerVertex * verticesPerCube
	totalFloats := r.numCubes * floatsPerCube

	// Size of each buffer section in bytes
	r.bufferSize = totalFloats * 4 // 4 bytes per float

	// Total size of the buffer in bytes (all sections)
	totalBufferSize := r.bufferSize * r.numBuffers

	// Initialize buffer offsets array (in float32 units, not bytes)
	r.bufferOffsets = make([]int, r.numBuffers)
	for i := 0; i < r.numBuffers; i++ {
		r.bufferOffsets[i] = i * totalFloats
	}

	// Initialize sync objects array
	r.syncObjects = make([]uintptr, r.numBuffers)
	for i := 0; i < r.numBuffers; i++ {
		r.syncObjects[i] = 0 // Will be initialized when first used
	}

	// Create the persistent mapped buffer
	persistentVBO, err := openglhelper.NewPersistentBuffer(
		gl.ARRAY_BUFFER,
		totalBufferSize,
		false, // read
		true,  // write
	)
	if err != nil {
		return fmt.Errorf("failed to create persistent buffer: %w", err)
	}
	r.cubeVBO = persistentVBO

	// Bind the persistent VBO to the VAO and set up attributes
	r.cubeVAO.Bind()
	r.cubeVBO.Bind()

	// Re-specify the vertex attributes for our new persistent buffer
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 9*4, 0)
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 9*4, 3*4)
	r.cubeVAO.SetVertexAttribPointer(2, 3, gl.FLOAT, false, 9*4, 6*4)

	// Get a Go slice that points to the mapped memory
	mappedPtr := r.cubeVBO.GetMappedPointer()
	if mappedPtr == nil {
		return fmt.Errorf("failed to get mapped pointer")
	}

	// Create a slice that refers to the mapped memory
	r.mappedMemory = unsafe.Slice((*float32)(mappedPtr), totalFloats*r.numBuffers)

	// Initialize all buffer sections with the cube data
	// Create a temporary slice for cube vertex data
	vertexData := make([]float32, totalFloats)

	// Generate initial vertex data for all cubes
	for i := 0; i < r.numCubes; i++ {
		baseIdx := i * floatsPerCube

		// Copy the template cube into the cube's vertex data
		for v := 0; v < verticesPerCube; v++ {
			for j := 0; j < floatsPerVertex; j++ {
				vertexData[baseIdx+v*floatsPerVertex+j] = baseCubeVertices[v*floatsPerVertex+j]
			}
		}

		// Apply position and color
		pos := r.positions[i]
		color := r.colors[i]

		for v := 0; v < verticesPerCube; v++ {
			vertexBase := baseIdx + v*floatsPerVertex

			// Update position for this vertex
			vertexData[vertexBase] += pos[0]
			vertexData[vertexBase+1] += pos[1]
			vertexData[vertexBase+2] += pos[2]

			// Set color for this vertex
			vertexData[vertexBase+6] = color[0]
			vertexData[vertexBase+7] = color[1]
			vertexData[vertexBase+8] = color[2]
		}
	}

	// Copy the initial vertex data to all buffer sections
	for i := 0; i < r.numBuffers; i++ {
		copy(r.mappedMemory[r.bufferOffsets[i]:r.bufferOffsets[i]+totalFloats], vertexData)
	}

	r.cubeVAO.Unbind()

	return nil
}

// initializeCubeData sets up the initial positions, colors, and color phases for all cubes
func (r *Renderer) initializeCubeData() {
	// Initialize arrays
	r.positions = make([]mgl32.Vec3, r.numCubes)
	r.velocities = make([]mgl32.Vec3, r.numCubes)
	r.colors = make([]mgl32.Vec3, r.numCubes)
	r.colorPhase = make([]float32, r.numCubes)

	// Create cubes in a volume
	bounds := float32(30.0)
	rand.Seed(42) // Use fixed seed for reproducibility

	for i := 0; i < r.numCubes; i++ {
		// Random position within the bounds
		r.positions[i] = mgl32.Vec3{
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
		}

		// Keep velocities for future use
		r.velocities[i] = mgl32.Vec3{
			(rand.Float32()*2.0 - 1.0) * 0.8,
			(rand.Float32()*2.0 - 1.0) * 0.8,
			(rand.Float32()*2.0 - 1.0) * 0.8,
		}

		// Initial color (bright)
		r.colors[i] = mgl32.Vec3{
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
			rand.Float32()*0.5 + 0.5, // 0.5-1.0
		}

		// Random phase offset for color cycling (0.0 - 1.0)
		r.colorPhase[i] = rand.Float32()
	}
}

// createFenceSync creates a fence sync object to track when the GPU is done using a buffer
func (r *Renderer) createFenceSync(bufferIdx int) {
	// Delete any existing sync object
	if r.syncObjects[bufferIdx] != 0 {
		gl.DeleteSync(r.syncObjects[bufferIdx])
	}

	// Create a new sync object
	r.syncObjects[bufferIdx] = gl.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
}

// waitForSync waits until the GPU has finished using the buffer at the given index
func (r *Renderer) waitForSync(bufferIdx int) {
	// Early exit if no sync object exists
	if r.syncObjects[bufferIdx] == 0 {
		return
	}

	const timeout uint64 = 1000000000 // 1 second in nanoseconds

	for {
		waitReturn := gl.ClientWaitSync(r.syncObjects[bufferIdx], gl.SYNC_FLUSH_COMMANDS_BIT, timeout)
		if waitReturn == gl.ALREADY_SIGNALED || waitReturn == gl.CONDITION_SATISFIED {
			// Sync is complete, we can proceed
			return
		} else if waitReturn == gl.WAIT_FAILED {
			// Something went wrong, but we'll proceed anyway to avoid deadlock
			return
		} else if waitReturn == gl.TIMEOUT_EXPIRED {
			// Timeout occurred, but we'll proceed to avoid deadlock
			return
		}
	}
}

// updateThread continuously updates cube colors in a separate thread
func (r *Renderer) updateThread() {
	for {
		// Check if we should quit
		select {
		case <-r.updateQuitChan:
			return
		default:
			// Continue execution
		}

		// Wait a bit to avoid busy waiting if there's nothing to do
		if !r.isUpdateThreadBusy {
			time.Sleep(1 * time.Millisecond)
			continue
		}

		// Get state information (under mutex protection)
		r.updateMutex.Lock()
		bufferIdx := r.updateBufferIdx
		startIdx := r.currentBatchIndex
		endIdx := startIdx + r.cubesPerBatch
		if endIdx > r.numCubes {
			endIdx = r.numCubes
		}
		totalTime := r.totalTime
		bufferOffset := r.bufferOffsets[bufferIdx]
		r.updateMutex.Unlock()

		// Calculate update parameters
		floatsPerVertex := 9
		verticesPerCube := 24
		floatsPerCube := floatsPerVertex * verticesPerCube

		// Update only the current batch of cubes
		for i := startIdx; i < endIdx; i++ {
			// Calculate a cycling color based on time and phase
			phase := r.colorPhase[i]
			cyclePos := float32(math.Mod(float64(totalTime/r.colorCycleTime)+float64(phase), 1.0))

			// Create a flowing rainbow effect
			red := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi)))
			green := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi+2*math.Pi/3)))
			blue := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi+4*math.Pi/3)))

			// Update the color directly in the main thread's array
			r.colors[i] = mgl32.Vec3{red, green, blue}

			// Update this cube's vertices in the mapped memory
			baseIdx := i*floatsPerCube + bufferOffset

			// Update each vertex for this cube
			for v := 0; v < verticesPerCube; v++ {
				vIdx := baseIdx + v*floatsPerVertex

				// Update just the color component - directly write to GPU memory
				r.mappedMemory[vIdx+6] = red
				r.mappedMemory[vIdx+7] = green
				r.mappedMemory[vIdx+8] = blue
			}
		}

		// Store the results (under mutex protection)
		r.updateMutex.Lock()
		r.isUpdateThreadBusy = false
		r.updatedBufferIdx = bufferIdx

		// Move to the next batch for the next frame
		r.currentBatchIndex = endIdx
		if r.currentBatchIndex >= r.numCubes {
			r.currentBatchIndex = 0
		}
		r.updateMutex.Unlock()
	}
}

// updateCubes schedules an update for a batch of cubes on the update thread
func (r *Renderer) updateCubes() {
	r.updateMutex.Lock()
	defer r.updateMutex.Unlock()

	// If update thread is busy, wait for it to complete
	if r.isUpdateThreadBusy {
		return
	}

	// Get the next buffer index to update (the one not currently in use by the GPU)
	// Skip the current buffer and the one that was just rendered
	nextBufferIdx := (r.currentBufferIdx + 1) % r.numBuffers
	if nextBufferIdx == r.updatedBufferIdx {
		nextBufferIdx = (nextBufferIdx + 1) % r.numBuffers
	}

	// Set up the update task for the update thread
	r.isUpdateThreadBusy = true
	r.updateBufferIdx = nextBufferIdx
	r.lastUpdateTime = time.Now()
}

// render renders the scene with the current buffer section
func (r *Renderer) render() {
	// Clear the screen
	r.window.Clear()

	// Enable depth testing for proper 3D rendering
	gl.Enable(gl.DEPTH_TEST)

	// Use our shader
	r.cubeShader.Use()

	// Set up view and projection matrices
	view := r.camera.ViewMatrix()
	projection := r.camera.ProjectionMatrix()
	r.cubeShader.SetMat4("view", view)
	r.cubeShader.SetMat4("projection", projection)

	// Set up lighting parameters
	r.cubeShader.SetVec3("viewPos", r.camera.Position())
	r.cubeShader.SetVec3("lightPos", mgl32.Vec3{30.0, 30.0, 30.0})
	r.cubeShader.SetVec3("lightColor", mgl32.Vec3{1.0, 1.0, 1.0})

	// Set model matrix (identity since we're transforming vertices directly)
	r.cubeShader.SetMat4("model", mgl32.Ident4())

	// Bind the VAO
	r.cubeVAO.Bind()

	// If we have an updated buffer from the worker, use that
	// Otherwise, use the current buffer
	renderBufferIdx := r.currentBufferIdx
	if r.updatedBufferIdx >= 0 {
		renderBufferIdx = r.updatedBufferIdx
		r.updatedBufferIdx = -1 // Mark as used
	}

	// Calculate buffer offset in bytes for the current buffer section
	bufferOffsetBytes := r.bufferOffsets[renderBufferIdx] * 4 // Convert from floats to bytes

	// Update attribute pointers to point to the current buffer section
	gl.BindBuffer(gl.ARRAY_BUFFER, r.cubeVBO.ID)
	stride := int32(9 * 4) // 9 floats per vertex, 4 bytes per float

	// Position attribute
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, stride, gl.PtrOffset(bufferOffsetBytes))

	// Normal attribute
	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, stride, gl.PtrOffset(bufferOffsetBytes+3*4))

	// Color attribute
	gl.VertexAttribPointer(2, 3, gl.FLOAT, false, stride, gl.PtrOffset(bufferOffsetBytes+6*4))

	// Bind element buffer
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, r.cubeEBO.ID)

	// Draw all cubes
	gl.DrawElements(gl.TRIANGLES, int32(r.numCubes*36), gl.UNSIGNED_INT, gl.PtrOffset(0))

	// Advance to the next buffer section for the next frame
	r.currentBufferIdx = (r.currentBufferIdx + 1) % r.numBuffers
}

// Run starts the main rendering loop
func (r *Renderer) Run() {
	// Set up initial OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0) // Dark blue background

	// Main loop
	for !r.window.ShouldClose() {
		// Calculate delta time
		currentTime := glfw.GetTime()
		r.deltaTime = float32(currentTime - r.lastFrameTime)
		r.lastFrameTime = currentTime
		r.totalTime += r.deltaTime

		// Process input
		r.camera.ProcessKeyboardInput(r.deltaTime, r.window)

		// Update cube colors and write to the next buffer section
		r.updateCubes()

		// Render using the available buffer section
		r.render()

		// Swap buffers and poll events
		r.window.SwapBuffers()
		r.window.PollEvents()
	}

	// Cleanup resources
	r.Cleanup()
}

// Cleanup frees all resources
func (r *Renderer) Cleanup() {
	// Signal update thread to exit
	if !r.isClosed {
		r.isClosed = true
		close(r.updateQuitChan)
	}

	// Clean up sync objects
	for i := 0; i < r.numBuffers; i++ {
		if r.syncObjects[i] != 0 {
			gl.DeleteSync(r.syncObjects[i])
			r.syncObjects[i] = 0
		}
	}

	// Clean up OpenGL resources
	if r.cubeVBO != nil {
		r.cubeVBO.Unmap()
		r.cubeVBO.Delete()
	}

	if r.cubeEBO != nil {
		r.cubeEBO.Delete()
	}

	if r.cubeVAO != nil {
		r.cubeVAO.Delete()
	}

	// Close window
	r.window.Close()
}

// createCubeIndices creates indices for all cubes
func (r *Renderer) createCubeIndices() []uint32 {
	totalIndices := r.numCubes * 6 * 6 // 6 faces per cube, 2 triangles per face (3 indices per triangle)
	indices := make([]uint32, totalIndices)

	// Create index buffer for all cubes
	// Each cube has 24 vertices (4 vertices per face * 6 faces)
	for i := 0; i < r.numCubes; i++ {
		baseVertex := uint32(i * 24) // Base vertex index for this cube
		baseIndex := i * 36          // Base index for this cube (6 faces * 6 indices)

		// For each face of the cube (6 faces)
		for face := 0; face < 6; face++ {
			faceBaseVertex := baseVertex + uint32(face*4) // Base vertex for this face
			faceBaseIndex := baseIndex + face*6           // Base index for this face

			// Define winding order based on which face we're processing
			if face == 1 || face == 3 || face == 5 {
				// Back, Right, and Top faces need reversed winding
				indices[faceBaseIndex] = faceBaseVertex       // 0
				indices[faceBaseIndex+1] = faceBaseVertex + 2 // 2
				indices[faceBaseIndex+2] = faceBaseVertex + 1 // 1

				indices[faceBaseIndex+3] = faceBaseVertex     // 0
				indices[faceBaseIndex+4] = faceBaseVertex + 3 // 3
				indices[faceBaseIndex+5] = faceBaseVertex + 2 // 2
			} else {
				// Front, Left, and Bottom faces keep original winding
				indices[faceBaseIndex] = faceBaseVertex       // 0
				indices[faceBaseIndex+1] = faceBaseVertex + 1 // 1
				indices[faceBaseIndex+2] = faceBaseVertex + 2 // 2

				indices[faceBaseIndex+3] = faceBaseVertex     // 0
				indices[faceBaseIndex+4] = faceBaseVertex + 2 // 2
				indices[faceBaseIndex+5] = faceBaseVertex + 3 // 3
			}
		}
	}

	return indices
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
