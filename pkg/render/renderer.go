package render

import (
	"fmt"
	"log"
	"unsafe"

	"openglhelper"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/leterax/go-voxels/pkg/voxel"
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

	// Voxel rendering data
	cubeVAO *openglhelper.VertexArrayObject

	// Single persistent buffer for chunk data with coherent mapping
	persistentBuffer     *openglhelper.BufferObject
	persistentBufferSize int
	mappedUints          []uint32

	// Multi-draw indirect support
	indirectBuffer       *openglhelper.BufferObject
	drawCommands         []openglhelper.DrawElementsIndirectCommand
	maxDrawCommands      int
	currentDrawCommands  int
	indicesBuffer        *openglhelper.BufferObject
	chunkPositionsBuffer *openglhelper.BufferObject
	chunkPositions       []mgl32.Vec4

	// Rendering modes
	isWireframeMode bool

	// Optimization flags
	verticesNeedUpdate bool

	// Cleanup tracking
	isClosed bool
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
		window: window,
		camera: camera,
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

	// Initialize voxel rendering system
	if err := renderer.initVoxelRenderSystem(); err != nil {
		return nil, fmt.Errorf("failed to initialize voxel render system: %w", err)
	}

	return renderer, nil
}

// initVoxelRenderSystem sets up the geometry and buffers for voxel rendering
func (r *Renderer) initVoxelRenderSystem() error {
	// Create a VAO for the voxel data
	vao := openglhelper.NewVAO()
	r.cubeVAO = vao
	r.cubeVAO.Bind()

	// Calculate the required buffer size for persistent mapping
	// Each chunk contains packed vertices (uint32) for each quad
	const maxQuadsPerChunk = 4096 // Maximum number of quads we expect in a chunk
	const bytesPerVertex = 4      // 4 bytes for each packed uint32 vertex
	const verticesPerQuad = 4     // Each quad has 4 vertices

	// Calculate size requirements
	r.persistentBufferSize = maxQuadsPerChunk * 1024 * verticesPerQuad * bytesPerVertex // Support ~100 chunks

	// Create a single persistent buffer with coherent flag
	persistentBuffer, mappedData, err := openglhelper.CreatePersistentBuffer(
		gl.ARRAY_BUFFER,
		r.persistentBufferSize,
		gl.MAP_WRITE_BIT|gl.MAP_PERSISTENT_BIT|gl.MAP_COHERENT_BIT)

	if err != nil {
		return fmt.Errorf("failed to create persistent buffer: %w", err)
	}

	r.persistentBuffer = persistentBuffer
	r.mappedUints = openglhelper.BytesToUint32(mappedData)

	// Bind the buffer
	r.persistentBuffer.Bind()

	// Set up vertex attribute - we only have one attribute (the packed uint32)
	gl.VertexAttribIPointer(0, 1, gl.UNSIGNED_INT, 4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	// Setup indices for quads
	// For each quad, we need 6 indices to form 2 triangles
	// Create a pattern that's reused for all quads: [0,1,2, 0,2,3, 4,5,6, 4,6,7, ...]
	const maxQuads = maxQuadsPerChunk * 100 // Support for ~100 chunks
	indices := make([]uint32, maxQuads*6)

	for i := uint32(0); i < uint32(maxQuads); i++ {
		// Calculate base vertex index for this quad
		baseVertex := i * 4
		// Index into the indices array
		idxBase := i * 6

		// First triangle: 0,1,2
		indices[idxBase] = baseVertex
		indices[idxBase+1] = baseVertex + 1
		indices[idxBase+2] = baseVertex + 2

		// Second triangle: 0,2,3
		indices[idxBase+3] = baseVertex
		indices[idxBase+4] = baseVertex + 2
		indices[idxBase+5] = baseVertex + 3
	}

	// Create an Element Buffer Object for indices
	r.indicesBuffer = openglhelper.NewEBO(indices, openglhelper.StaticDraw)

	// Setup indirect drawing command buffer
	r.maxDrawCommands = 1024 // Support up to 1024 chunks
	r.drawCommands = make([]openglhelper.DrawElementsIndirectCommand, r.maxDrawCommands)
	r.chunkPositions = make([]mgl32.Vec4, r.maxDrawCommands)

	// Create the indirect command buffer using the helper
	r.indirectBuffer = openglhelper.NewIndirectBuffer(r.maxDrawCommands, openglhelper.DynamicDraw)

	// Create and setup the chunk positions buffer (Shader Storage Buffer Object)
	positionsBufferSize := r.maxDrawCommands * int(unsafe.Sizeof(mgl32.Vec4{}))
	r.chunkPositionsBuffer = openglhelper.NewEmptySSBO(positionsBufferSize, openglhelper.DynamicDraw)

	// Bind the SSBO to the binding point used in the shader
	r.chunkPositionsBuffer.BindBase(0)

	return nil
}

// SetCameraPosition sets the camera position in world space
func (r *Renderer) SetCameraPosition(position mgl32.Vec3) {
	r.camera.SetPosition(position)
}

// SetCameraLookAt makes the camera look at a target position
func (r *Renderer) SetCameraLookAt(target mgl32.Vec3) {
	r.camera.LookAt(target)
}

// SetupOpenGL initializes OpenGL state for rendering
func (r *Renderer) SetupOpenGL() {
	// Set up initial OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0)        // Dark blue background
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL) // Ensure we start in solid mode
	r.isWireframeMode = false                  // Initialize wireframe mode to false

	// Set initial shader uniforms
	r.cubeShader.Use()
}

// ShouldClose returns whether the window should close
func (r *Renderer) ShouldClose() bool {
	return r.window.ShouldClose()
}

// RenderFrame renders a single frame with the given chunks
func (r *Renderer) RenderFrame(chunks []*voxel.Chunk) {
	// Calculate delta time
	currentTime := glfw.GetTime()
	r.deltaTime = float32(currentTime - r.lastFrameTime)
	r.lastFrameTime = currentTime
	r.totalTime += r.deltaTime

	// Process input
	r.camera.ProcessKeyboardInput(r.deltaTime, r.window)

	// Clear the screen
	r.window.Clear()

	// Enable depth testing for proper 3D rendering
	gl.Enable(gl.DEPTH_TEST)

	// Render chunks if we have any
	if len(chunks) > 0 {
		r.RenderChunksIndirect(chunks)
	}

	// Swap buffers and poll events
	r.window.SwapBuffers()
	r.window.PollEvents()
}

// Run starts the main rendering loop with the provided chunks
func (r *Renderer) Run(chunks []*voxel.Chunk) {
	// Set up initial OpenGL state
	r.SetupOpenGL()

	// Initialize draw commands for all chunks at startup
	r.UpdateDrawCommands(chunks)

	// Main loop
	for !r.ShouldClose() {
		// Render a frame
		r.RenderFrame(chunks)
	}

	// Cleanup resources
	r.Cleanup()
}

// Cleanup releases all resources used by the renderer
func (r *Renderer) Cleanup() {
	if r.isClosed {
		return
	}

	// Clean up OpenGL resources
	if r.persistentBuffer != nil {
		r.persistentBuffer.Unmap()
		r.persistentBuffer.Delete()
	}

	if r.indicesBuffer != nil {
		r.indicesBuffer.Delete()
	}

	if r.cubeVAO != nil {
		r.cubeVAO.Delete()
	}

	// Clean up multi-draw indirect resources
	if r.indirectBuffer != nil {
		r.indirectBuffer.Delete()
	}

	if r.chunkPositionsBuffer != nil {
		r.chunkPositionsBuffer.Delete()
	}

	// Close window
	r.window.Close()

	r.isClosed = true
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

	// Toggle wireframe mode with X key
	if key == glfw.KeyX && action == glfw.Press {
		r.ToggleWireframeMode()
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

// RenderChunk renders a voxel chunk using the packed vertex format
func (r *Renderer) RenderChunk(chunk *voxel.Chunk) {
	if chunk.Mesh == nil || len(chunk.Mesh.PackedVertices) == 0 {
		return // Nothing to render
	}

	// Ensure we have the persistent buffer set up
	r.cubeVAO.Bind()
	r.persistentBuffer.Bind()

	// Get the number of vertices to render
	vertexCount := len(chunk.Mesh.PackedVertices)
	if vertexCount == 0 {
		return
	}

	// Copy vertices directly to mapped memory
	// Since we're rendering a single chunk, we'll use the first part of the buffer
	copy(r.mappedUints[:vertexCount], chunk.Mesh.PackedVertices)

	// Use the shader
	r.cubeShader.Use()

	// Set up view and projection matrices
	view := r.camera.ViewMatrix()
	projection := r.camera.ProjectionMatrix()
	r.cubeShader.SetMat4("view", view)
	r.cubeShader.SetMat4("projection", projection)

	// Set chunk position
	chunkWorldPos := chunk.WorldPosition()
	r.cubeShader.SetVec3("chunkPosition", chunkWorldPos)

	// Set up lighting parameters
	r.cubeShader.SetVec3("viewPos", r.camera.Position())
	r.cubeShader.SetVec3("lightPos", mgl32.Vec3{30.0, 30.0, 30.0})
	r.cubeShader.SetVec3("lightColor", mgl32.Vec3{1.0, 1.0, 1.0})

	// Set model matrix (identity for now, position is handled through chunkPosition uniform)
	r.cubeShader.SetMat4("model", mgl32.Ident4())

	// Draw the quads (each quad has 4 vertices)
	gl.DrawArrays(gl.QUADS, 0, int32(vertexCount))
}

// UpdateDrawCommands updates the indirect drawing commands buffer for all renderable chunks
func (r *Renderer) UpdateDrawCommands(chunks []*voxel.Chunk) {
	// Reset current count
	r.currentDrawCommands = 0

	// Early exit if no chunks
	if len(chunks) == 0 {
		fmt.Println("UpdateDrawCommands: No chunks to render")
		return
	}

	fmt.Printf("UpdateDrawCommands: Processing %d chunks\n", len(chunks))

	// Always update vertices when updating draw commands
	r.verticesNeedUpdate = true

	// Limit to max commands
	chunkCount := len(chunks)
	if chunkCount > r.maxDrawCommands {
		chunkCount = r.maxDrawCommands
		fmt.Printf("UpdateDrawCommands: Limited to %d chunks (max commands)\n", r.maxDrawCommands)
	}

	// Reset total quads for vertex base calculation
	totalQuads := 0

	// Reset vertex buffer for new data (if needed)
	vertexOffset := 0
	maxVertices := r.persistentBufferSize / 4
	shouldUpdateVertices := r.verticesNeedUpdate

	fmt.Printf("UpdateDrawCommands: shouldUpdateVertices=%v\n", shouldUpdateVertices)

	// If updating vertices, bind the buffer
	if shouldUpdateVertices {
		r.persistentBuffer.Bind()
	}

	// Process each chunk - handle both draw commands and vertex data in the same loop
	// to ensure perfect alignment between buffers
	for i := 0; i < chunkCount; i++ {
		chunk := chunks[i]

		// Skip chunks with no mesh or no packed vertices
		if chunk == nil || chunk.Mesh == nil || len(chunk.Mesh.PackedVertices) == 0 {
			continue
		}

		// Calculate number of quads in this chunk
		vertexCount := len(chunk.Mesh.PackedVertices)
		quads := vertexCount / 4
		if quads == 0 {
			continue
		}

		// Skip if would exceed buffer
		if shouldUpdateVertices && vertexOffset+vertexCount > maxVertices {
			log.Printf("Warning: Buffer limit reached, some chunks not rendered. Used: %d/%d vertices.",
				vertexOffset, maxVertices)
			break
		}

		// Create command
		cmd := &r.drawCommands[r.currentDrawCommands]

		// Fill command data
		cmd.Count = uint32(quads * 6)           // 6 indices per quad
		cmd.InstanceCount = 1                   // No instancing
		cmd.FirstIndex = uint32(totalQuads * 6) // Offset into index buffer
		cmd.BaseVertex = int32(totalQuads * 4)  // Offset into vertex buffer
		cmd.BaseInstance = 0                    // No instancing

		// Store the chunk position in the chunk positions array
		worldPos := chunk.WorldPosition()
		r.chunkPositions[r.currentDrawCommands] = mgl32.Vec4{worldPos.X(), worldPos.Y(), worldPos.Z(), 0.0}

		// If we need to update vertices, copy them to the buffer
		if shouldUpdateVertices {
			// Copy vertices directly to mapped memory
			copy(r.mappedUints[vertexOffset:vertexOffset+vertexCount], chunk.Mesh.PackedVertices)
			vertexOffset += vertexCount
		}

		// Increment counters
		totalQuads += quads
		r.currentDrawCommands++
	}

	fmt.Printf("UpdateDrawCommands: Prepared %d draw commands with %d total quads\n", r.currentDrawCommands, totalQuads)

	// Update the draw commands buffer if we have commands
	if r.currentDrawCommands > 0 {
		// Update indirect command buffer using the helper method
		r.indirectBuffer.UpdateIndirectCommands(r.drawCommands[:r.currentDrawCommands])

		// Update chunk positions buffer
		r.chunkPositionsBuffer.Bind()
		positionsBufferSize := r.currentDrawCommands * int(unsafe.Sizeof(mgl32.Vec4{}))
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, positionsBufferSize, gl.Ptr(r.chunkPositions))
	}

	// Reset vertex update flag
	r.verticesNeedUpdate = false
}

// RenderChunksIndirect renders all chunks using multidrawindirect
func (r *Renderer) RenderChunksIndirect(chunks []*voxel.Chunk) {
	// Skip if no commands
	if r.currentDrawCommands == 0 {
		fmt.Println("RenderChunksIndirect: No draw commands, skipping render")
		return
	}

	fmt.Printf("RenderChunksIndirect: Rendering %d chunks with %d draw commands\n", len(chunks), r.currentDrawCommands)

	// Ensure we have the persistent buffer set up
	r.cubeVAO.Bind()
	r.persistentBuffer.Bind()
	r.indicesBuffer.Bind()

	// Use the shader
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

	// Set model matrix (identity for now)
	r.cubeShader.SetMat4("model", mgl32.Ident4())

	// Set a global chunk position (will be overridden by vertex shader)
	r.cubeShader.SetVec3("chunkPosition", mgl32.Vec3{0, 0, 0})

	// Update attribute pointer to point to the start of the buffer
	gl.VertexAttribIPointer(0, 1, gl.UNSIGNED_INT, 4, gl.PtrOffset(0))

	// Bind the indirect buffer
	r.indirectBuffer.Bind()

	// Draw all chunks in a single call using multidrawindirect
	openglhelper.MultiDrawElementsIndirect(
		gl.TRIANGLES,
		gl.UNSIGNED_INT,
		r.currentDrawCommands,
	)
}

// ToggleWireframeMode switches between solid and wireframe rendering
func (r *Renderer) ToggleWireframeMode() {
	r.isWireframeMode = !r.isWireframeMode

	if r.isWireframeMode {
		// Set GL to wireframe mode
		gl.PolygonMode(gl.FRONT_AND_BACK, gl.LINE)
	} else {
		// Set GL back to fill mode
		gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL)
	}
}
