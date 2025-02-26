package render

import (
	"fmt"
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

	// Triple-buffered persistent memory for chunk data
	tripleBuffer *openglhelper.TripleBuffer

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
	const maxQuadsPerChunk = 65536 // Maximum number of quads we expect in a chunk
	const bytesPerVertex = 4       // 4 bytes for each packed uint32 vertex
	const verticesPerQuad = 4      // Each quad has 4 vertices
	const numBuffers = 3           // Triple buffering

	// Calculate size requirements
	vertexBufferSize := maxQuadsPerChunk * verticesPerQuad * bytesPerVertex

	// Create the triple-buffered persistent vertex buffer
	tripleBuffer, err := openglhelper.NewTripleBuffer(gl.ARRAY_BUFFER, vertexBufferSize, numBuffers)
	if err != nil {
		return fmt.Errorf("failed to create triple buffer: %w", err)
	}
	r.tripleBuffer = tripleBuffer

	// Bind the buffer
	r.tripleBuffer.Buffer.Bind()

	// Set up vertex attribute - we only have one attribute now (the packed uint32)
	gl.VertexAttribIPointer(0, 1, gl.UNSIGNED_INT, 4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	// Setup indices for quads
	// For each quad, we need 6 indices to form 2 triangles
	// Create a pattern that's reused for all quads: [0,1,2, 0,2,3, 4,5,6, 4,6,7, ...]
	const maxQuads = maxQuadsPerChunk * numBuffers // Across all buffers
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

// Run starts the main rendering loop with the provided chunks
func (r *Renderer) Run(chunks []*voxel.Chunk) {
	// Set up initial OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.ClearColor(0.05, 0.05, 0.1, 1.0)        // Dark blue background
	gl.PolygonMode(gl.FRONT_AND_BACK, gl.FILL) // Ensure we start in solid mode
	r.isWireframeMode = false                  // Initialize wireframe mode to false

	// Set initial shader uniforms
	r.cubeShader.Use()

	// Initialize draw commands for all chunks at startup
	r.UpdateDrawCommands(chunks)

	// Main loop
	for !r.window.ShouldClose() {
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

		// Render all chunks using multi-draw indirect
		if len(chunks) > 0 {
			// Update draw commands every frame in case chunks have changed
			// This could be optimized to only update when chunks actually change
			r.UpdateDrawCommands(chunks)
			r.RenderChunksIndirect(chunks)
		}

		// Swap buffers and poll events
		r.window.SwapBuffers()
		r.window.PollEvents()
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
	if r.tripleBuffer != nil {
		r.tripleBuffer.Cleanup()
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
	if key == KeyX && action == glfw.Press {
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
	r.tripleBuffer.Buffer.Bind()

	// Wait for the current buffer to be ready
	r.tripleBuffer.WaitForSync()

	// Get the number of vertices to render
	vertexCount := len(chunk.Mesh.PackedVertices)
	if vertexCount == 0 {
		return
	}

	// Get the offset for the current buffer section
	offset := r.tripleBuffer.CurrentOffsetFloats()

	// Copy the vertices directly to the mapped uint32 slice
	copy(r.tripleBuffer.MappedUints[offset:offset+vertexCount], chunk.Mesh.PackedVertices)

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

	// Calculate buffer offset in bytes for the current buffer section
	bufferOffsetBytes := r.tripleBuffer.CurrentOffsetBytes()

	// Update attribute pointer to point to the current buffer section
	gl.VertexAttribIPointer(0, 1, gl.UNSIGNED_INT, 4, gl.PtrOffset(bufferOffsetBytes))

	// Draw the quads (each quad has 4 vertices)
	gl.DrawArrays(gl.QUADS, 0, int32(vertexCount))

	// Create sync object for this buffer
	r.tripleBuffer.CreateFenceSync()

	// Advance to the next buffer section for the next frame
	r.tripleBuffer.Advance()
}

// UpdateDrawCommands updates the indirect drawing commands buffer for all renderable chunks
func (r *Renderer) UpdateDrawCommands(chunks []*voxel.Chunk) {
	// Reset current count
	r.currentDrawCommands = 0

	// Early exit if no chunks
	if len(chunks) == 0 {
		return
	}

	// Limit to max commands
	chunkCount := len(chunks)
	if chunkCount > r.maxDrawCommands {
		chunkCount = r.maxDrawCommands
	}

	// Fill commands
	totalQuads := 0
	for i := 0; i < chunkCount; i++ {
		chunk := chunks[i]

		// Skip chunks with no mesh or no packed vertices
		if chunk == nil || chunk.Mesh == nil || len(chunk.Mesh.PackedVertices) == 0 {
			continue
		}

		// Calculate number of quads in this chunk
		quads := len(chunk.Mesh.PackedVertices) / 4
		if quads == 0 {
			continue
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

		// Increment counters
		totalQuads += quads
		r.currentDrawCommands++
	}

	// Update the draw commands buffer if we have commands
	if r.currentDrawCommands > 0 {
		// Update indirect command buffer using the helper method
		r.indirectBuffer.UpdateIndirectCommands(r.drawCommands[:r.currentDrawCommands])

		// Update chunk positions buffer
		r.chunkPositionsBuffer.Bind()
		positionsBufferSize := r.currentDrawCommands * int(unsafe.Sizeof(mgl32.Vec4{}))
		gl.BufferSubData(gl.SHADER_STORAGE_BUFFER, 0, positionsBufferSize, gl.Ptr(r.chunkPositions))
	}
}

// RenderChunksIndirect renders all chunks using multidrawindirect
func (r *Renderer) RenderChunksIndirect(chunks []*voxel.Chunk) {
	// Skip if no commands
	if r.currentDrawCommands == 0 {
		return
	}

	// Ensure we have the persistent buffer set up
	r.cubeVAO.Bind()
	r.tripleBuffer.Buffer.Bind()
	r.indicesBuffer.Bind()

	// Wait for the current buffer to be ready
	r.tripleBuffer.WaitForSync()

	// Copy all chunk vertex data to the persistent buffer
	currentOffset := 0

	// Copy vertices from each chunk
	for _, chunk := range chunks {
		if chunk == nil || chunk.Mesh == nil || len(chunk.Mesh.PackedVertices) == 0 {
			continue
		}

		vertexCount := len(chunk.Mesh.PackedVertices)

		// Skip if would exceed buffer
		if currentOffset+vertexCount >= r.tripleBuffer.BufferSize/4 {
			break
		}

		// Copy vertices directly to mapped memory
		copy(r.tripleBuffer.MappedUints[r.tripleBuffer.CurrentOffsetFloats()+currentOffset:], chunk.Mesh.PackedVertices)

		currentOffset += vertexCount
	}

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

	// Calculate buffer offset in bytes for the current buffer section
	bufferOffsetBytes := r.tripleBuffer.CurrentOffsetBytes()

	// Update attribute pointer to point to the current buffer section
	gl.VertexAttribIPointer(0, 1, gl.UNSIGNED_INT, 4, gl.PtrOffset(bufferOffsetBytes))

	// Bind the indirect buffer
	r.indirectBuffer.Bind()

	// Draw all chunks in a single call using multidrawindirect
	openglhelper.MultiDrawElementsIndirect(
		gl.TRIANGLES,
		gl.UNSIGNED_INT,
		r.currentDrawCommands,
	)

	// Create fence sync for this buffer
	r.tripleBuffer.CreateFenceSync()

	// Advance to the next buffer section
	r.tripleBuffer.Advance()
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
