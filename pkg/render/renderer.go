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
	numCubes          int
	cubesPerBatch     int     // How many cubes to update per frame
	currentBatchIndex int     // Current batch index being updated
	dataTransferRate  float32 // MB/s written to GPU
	colorCycleTime    float32 // Time for complete color cycle

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

	// Worker thread communication
	workerChan       chan workerCommand      // Channel to send commands to worker
	workerResultChan chan workerUpdateResult // Channel to receive results from worker
	workerQuitChan   chan bool               // Channel to signal worker to quit
	isWorkerBusy     bool                    // Whether the worker is currently processing a task
	updatedBufferIdx int                     // Index of buffer most recently updated by worker
	isClosed         bool                    // Whether the renderer has been closed
}

// workerCommand represents a command to be processed by the worker thread
type workerCommand struct {
	bufferIdx    int          // Which buffer to update
	startIdx     int          // Start index of cubes to update
	endIdx       int          // End index of cubes to update
	deltaTime    float32      // Time step for physics
	totalTime    float32      // Total elapsed time
	bufferOffset int          // Offset in mappedMemory for this buffer
	mappedMemory []float32    // Slice of mapped memory
	positions    []mgl32.Vec3 // Cube positions to use
	colors       []mgl32.Vec3 // Direct reference to main thread's colors array
	colorPhase   []float32    // Phase offset for color cycling
}

// workerUpdateResult represents the result of a worker update operation
type workerUpdateResult struct {
	bufferIdx    int // Which buffer was updated
	numUpdated   int // How many cubes were updated
	bytesWritten int // How many bytes were written
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

		// Initialize worker channels
		workerChan:       make(chan workerCommand, 1),      // Buffer of 1 to avoid blocking
		workerResultChan: make(chan workerUpdateResult, 1), // Buffer of 1 to avoid blocking
		workerQuitChan:   make(chan bool, 1),               // Channel to signal worker to quit
		isWorkerBusy:     false,
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

	// Load shader
	fmt.Println("\nLoading shaders...")
	shader, err := openglhelper.LoadShaderFromFiles("pkg/render/shaders/vert.glsl", "pkg/render/shaders/frag.glsl")
	if err != nil {
		return nil, fmt.Errorf("failed to load shader: %w", err)
	}
	fmt.Println("Shader loaded successfully!")
	renderer.cubeShader = shader

	// Initialize cube rendering system
	if err := renderer.initCubeRenderSystem(); err != nil {
		return nil, fmt.Errorf("failed to initialize cube render system: %w", err)
	}

	// Start the worker goroutine that will handle color updates
	go renderer.workerThread()
	fmt.Println("Started worker thread for color updates")

	return renderer, nil
}

// initCubeRenderSystem sets up the cube geometry and persistent buffer
func (r *Renderer) initCubeRenderSystem() error {
	fmt.Println("Initializing cube render system...")

	// 1. Define cube vertices - each vertex contains position, normal and color
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

	// 2. Create indices for all cubes
	indices := r.createCubeIndices()

	// 3. Create VAO and EBO
	r.cubeVAO = openglhelper.NewVAO()
	r.cubeVAO.Bind()

	// Create a temporary static VBO for vertex setup - we'll replace this with the persistent one
	tempVBO := openglhelper.NewVBO(baseCubeVertices, openglhelper.StaticDraw)

	// Create an EBO for all the cube indices
	r.cubeEBO = openglhelper.NewEBO(indices, openglhelper.StaticDraw)

	// 4. Set up vertex attributes
	// Position attribute
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 9*4, 0)
	// Normal attribute
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 9*4, 3*4)
	// Color attribute
	r.cubeVAO.SetVertexAttribPointer(2, 3, gl.FLOAT, false, 9*4, 6*4)

	// Unbind temporary VBO (we don't need it anymore)
	tempVBO.Unbind()
	tempVBO.Delete()

	// 5. Initialize cube data (positions, colors)
	r.initializeCubeData()

	// 6. Calculate buffer sizes and offsets
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

	fmt.Printf("Creating persistent mapped buffer with triple buffering:\n")
	fmt.Printf("- Each buffer section: %.2f MB\n", float32(r.bufferSize)/1048576.0)
	fmt.Printf("- Total buffer size: %.2f MB\n", float32(totalBufferSize)/1048576.0)
	fmt.Printf("- Number of cubes: %d (updating %d per frame)\n", r.numCubes, r.cubesPerBatch)

	// 7. Create the persistent mapped buffer
	// Create the buffer with immutable storage using GL_BUFFER_STORAGE
	// Use GL_MAP_PERSISTENT_BIT, GL_MAP_WRITE_BIT, and GL_MAP_COHERENT_BIT
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

	// 8. Bind the persistent VBO to the VAO and set up attributes
	r.cubeVAO.Bind()
	r.cubeVBO.Bind()

	// Re-specify the vertex attributes for our new persistent buffer
	r.cubeVAO.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 9*4, 0)
	r.cubeVAO.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 9*4, 3*4)
	r.cubeVAO.SetVertexAttribPointer(2, 3, gl.FLOAT, false, 9*4, 6*4)

	// 9. Get a Go slice that points to the mapped memory
	mappedPtr := r.cubeVBO.GetMappedPointer()
	if mappedPtr == nil {
		return fmt.Errorf("failed to get mapped pointer")
	}

	// Create a slice that refers to the mapped memory
	r.mappedMemory = unsafe.Slice((*float32)(mappedPtr), totalFloats*r.numBuffers)

	// 10. Initialize all buffer sections with the cube data
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
	fmt.Println("Cube render system initialized successfully!")

	return nil
}

// initializeCubeData sets up the initial positions, colors, and color phases for all cubes
func (r *Renderer) initializeCubeData() {
	// Initialize arrays
	r.positions = make([]mgl32.Vec3, r.numCubes)
	r.velocities = make([]mgl32.Vec3, r.numCubes) // Keep this for potential future use
	r.colors = make([]mgl32.Vec3, r.numCubes)
	r.colorPhase = make([]float32, r.numCubes)

	// Create cubes in a volume
	bounds := float32(30.0)
	rand.Seed(42) // Use fixed seed for reproducibility

	for i := 0; i < r.numCubes; i++ {
		// Random position within the bounds (use 90% of bounds to prevent initial collisions)
		r.positions[i] = mgl32.Vec3{
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
			(rand.Float32()*2.0 - 1.0) * bounds * 0.9,
		}

		// Keep velocities for future use if needed
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

	// Wait for the sync object to be signaled with a timeout
	// Use a timeout to prevent infinite waiting
	const timeout uint64 = 1000000000 // 1 second in nanoseconds

	for {
		waitReturn := gl.ClientWaitSync(r.syncObjects[bufferIdx], gl.SYNC_FLUSH_COMMANDS_BIT, timeout)
		if waitReturn == gl.ALREADY_SIGNALED || waitReturn == gl.CONDITION_SATISFIED {
			// Sync is complete, we can proceed
			return
		} else if waitReturn == gl.WAIT_FAILED {
			// Something went wrong, but we'll proceed anyway to avoid deadlock
			fmt.Println("Warning: GL sync wait failed")
			return
		} else if waitReturn == gl.TIMEOUT_EXPIRED {
			// Timeout occurred, but we'll proceed to avoid deadlock
			// This should be rare in normal operation
			fmt.Println("Warning: GL sync wait timeout expired")
			return
		}
	}
}

// workerThread runs in a separate goroutine and handles color updates
func (r *Renderer) workerThread() {
	fmt.Println("Worker thread started")

	for {
		select {
		case cmd := <-r.workerChan:
			// Received a command to update a buffer
			bufferOffset := cmd.bufferOffset
			startIdx := cmd.startIdx
			endIdx := cmd.endIdx
			numCubesUpdated := endIdx - startIdx
			totalTime := cmd.totalTime

			// Calculate update parameters
			floatsPerVertex := 9
			verticesPerCube := 24
			floatsPerCube := floatsPerVertex * verticesPerCube

			bytesWritten := 0

			// Update only the current batch of cubes
			for i := startIdx; i < endIdx; i++ {
				// Calculate a cycling color based on time and phase
				phase := cmd.colorPhase[i]
				cyclePos := float32(math.Mod(float64(totalTime/r.colorCycleTime)+float64(phase), 1.0))

				// Create a flowing rainbow effect
				r := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi)))
				g := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi+2*math.Pi/3)))
				b := float32(0.5 + 0.5*math.Sin(float64(cyclePos*2*math.Pi+4*math.Pi/3)))

				// Update the color directly in the main thread's array
				cmd.colors[i] = mgl32.Vec3{r, g, b}

				// Update this cube's vertices in the mapped memory
				baseIdx := i*floatsPerCube + bufferOffset

				// Update each vertex for this cube
				for v := 0; v < verticesPerCube; v++ {
					vIdx := baseIdx + v*floatsPerVertex

					// Update just the color component - directly write to GPU memory
					cmd.mappedMemory[vIdx+6] = r
					cmd.mappedMemory[vIdx+7] = g
					cmd.mappedMemory[vIdx+8] = b

					bytesWritten += 3 * 4 // 3 color components * 4 bytes
				}
			}

			// Send the result back to the main thread - just signaling completion, no data copying
			result := workerUpdateResult{
				bufferIdx:    cmd.bufferIdx,
				numUpdated:   numCubesUpdated,
				bytesWritten: bytesWritten,
			}

			// Try to send the result, but don't block if channel is full
			select {
			case r.workerResultChan <- result:
				// Result sent successfully
			default:
				// Channel is full, continue anyway (main thread will detect this)
				fmt.Println("Warning: Worker result channel full, result dropped")
			}

		case <-r.workerQuitChan:
			// Received signal to quit
			fmt.Println("Worker thread shutting down")
			return
		}
	}
}

// updateCubes schedules an update for a batch of cubes on the worker thread
func (r *Renderer) updateCubes() {
	// If worker is busy, check if it has results
	if r.isWorkerBusy {
		select {
		case result := <-r.workerResultChan:
			// Worker has completed an update
			r.isWorkerBusy = false
			r.updatedBufferIdx = result.bufferIdx

			// No need to update colors here - worker writes directly to our color array
			// Calculate data transfer rate for statistics
			totalBytesTransferred := float32(result.bytesWritten)
			totalMBTransferred := totalBytesTransferred / (1024.0 * 1024.0)
			r.dataTransferRate = totalMBTransferred / r.deltaTime // MB/s

		default:
			// Worker is still busy, do nothing
			return
		}
	}

	// Get the next buffer index to update (the one not currently in use by the GPU)
	// Skip the current buffer and the one that was just rendered
	nextBufferIdx := (r.currentBufferIdx + 1) % r.numBuffers
	if nextBufferIdx == r.updatedBufferIdx {
		nextBufferIdx = (nextBufferIdx + 1) % r.numBuffers
	}

	// Wait for the GPU to finish using this buffer section before writing to it
	r.waitForSync(nextBufferIdx)

	// Compute start and end indices for the current batch of cubes
	startIdx := r.currentBatchIndex
	endIdx := startIdx + r.cubesPerBatch
	if endIdx > r.numCubes {
		endIdx = r.numCubes
	}

	// Schedule the update on the worker thread
	cmd := workerCommand{
		bufferIdx:    nextBufferIdx,
		startIdx:     startIdx,
		endIdx:       endIdx,
		deltaTime:    r.deltaTime,
		totalTime:    r.totalTime,
		bufferOffset: r.bufferOffsets[nextBufferIdx],
		mappedMemory: r.mappedMemory,
		positions:    r.positions,
		colors:       r.colors, // Pass direct reference to main thread's colors
		colorPhase:   r.colorPhase,
	}

	// Try to send the command without blocking
	select {
	case r.workerChan <- cmd:
		r.isWorkerBusy = true
	default:
		// Channel is full, worker is busy - we'll try again next frame
		fmt.Println("Worker busy, skipping cube update this frame")
	}

	// Move to the next batch for the next frame
	r.currentBatchIndex = endIdx
	if r.currentBatchIndex >= r.numCubes {
		r.currentBatchIndex = 0
	}
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

	// Enable backface culling for performance
	// gl.Enable(gl.CULL_FACE)

	// Draw all cubes
	gl.DrawElements(gl.TRIANGLES, int32(r.numCubes*36), gl.UNSIGNED_INT, gl.PtrOffset(0))

	// Disable culling and depth testing
	// gl.Disable(gl.CULL_FACE)
	// gl.Disable(gl.DEPTH_TEST)

	// Create a fence sync object for this buffer section after drawing
	// r.createFenceSync(renderBufferIdx)

	// Print FPS every second
	if int(r.totalTime) > int(r.totalTime-r.deltaTime) {
		currentFPS := 1.0 / r.deltaTime
		fmt.Printf("FPS: %.1f | Cubes: %d | Color updates: %d (%.1f%%) | Buffer: %d/%d | Zero-copy GPU Write: %.2f MB/s | Worker: %v\n",
			currentFPS, r.numCubes, r.cubesPerBatch, float32(r.cubesPerBatch)/float32(r.numCubes)*100.0,
			renderBufferIdx+1, r.numBuffers, r.dataTransferRate, r.isWorkerBusy)
	}

	// Advance to the next buffer section for the next frame
	r.currentBufferIdx = (r.currentBufferIdx + 1) % r.numBuffers
}

// Run starts the main rendering loop
func (r *Renderer) Run() {
	// Set up initial OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0) // Dark blue background

	fmt.Println("\nStarting rendering loop...")
	fmt.Printf("Rendering %d cubes (updating colors for %d per frame) with triple-buffered persistent mapping and zero-copy worker thread\n",
		r.numCubes, r.cubesPerBatch)

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
		// This schedules the update on a worker thread
		r.updateCubes()

		// Render using the available buffer section
		r.render()

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

	// Print average FPS before cleanup
	if r.frameCount > 0 {
		avgFPS := r.totalFPS / float32(r.frameCount)
		fmt.Printf("\nApplication closing. Average FPS: %.1f over %d frames\n", avgFPS, r.frameCount)
	}

	// Cleanup resources
	r.Cleanup()
}

// Cleanup frees all resources
func (r *Renderer) Cleanup() {
	// Signal worker thread to exit
	if !r.isClosed {
		r.isClosed = true
		r.workerQuitChan <- true
		close(r.workerChan)
		close(r.workerResultChan)
		close(r.workerQuitChan)
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
			// All faces need to be counter-clockwise when viewed from outside the cube
			if face == 1 || face == 3 || face == 5 {
				// Back, Right, and Top faces need reversed winding compared to the others
				// First triangle of the face (counter-clockwise winding)
				indices[faceBaseIndex] = faceBaseVertex       // 0
				indices[faceBaseIndex+1] = faceBaseVertex + 2 // 2
				indices[faceBaseIndex+2] = faceBaseVertex + 1 // 1

				// Second triangle of the face (counter-clockwise winding)
				indices[faceBaseIndex+3] = faceBaseVertex     // 0
				indices[faceBaseIndex+4] = faceBaseVertex + 3 // 3
				indices[faceBaseIndex+5] = faceBaseVertex + 2 // 2
			} else {
				// Front, Left, and Bottom faces keep original winding
				// First triangle of the face (counter-clockwise winding)
				indices[faceBaseIndex] = faceBaseVertex       // 0
				indices[faceBaseIndex+1] = faceBaseVertex + 1 // 1
				indices[faceBaseIndex+2] = faceBaseVertex + 2 // 2

				// Second triangle of the face (counter-clockwise winding)
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
