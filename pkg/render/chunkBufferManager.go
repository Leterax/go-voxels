// Package render provides utilities for rendering 3D voxel worlds efficiently using modern OpenGL techniques.
// It handles buffer management, rendering, and other graphics-related operations.
package render

import (
	"sync"
	"unsafe"

	"openglhelper"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// DrawElementsIndirectCommand mirrors the OpenGL structure for indirect drawing.
type DrawElementsIndirectCommand struct {
	Count         uint32 // Actual number of indices for this chunk.
	InstanceCount uint32 // Always 1.
	FirstIndex    uint32 // Offset in the index buffer (in units of 4 bytes).
	BaseVertex    int32  // Offset in the vertex buffer (in units of 4 bytes).
	BaseInstance  uint32 // The chunk index.
}

// Vec3i is used to represent a chunk's world position.
type Vec3i = mgl32.Vec3

// GLSync is a type alias for OpenGL sync objects
type GLSync = uintptr

// ChunkBufferManager is responsible for managing GPU buffers for voxel chunks.
// It handles the allocation, updating, and rendering of chunk data using persistent
// mapped buffers and triple buffering for optimal performance.
type ChunkBufferManager struct {
	maxChunks int // Total number of chunks that can be stored.

	// Maximum allocated bytes per chunk for vertex and index data.
	chunkSizeBytes     int // Maximum bytes allocated for vertex data per chunk.
	maxQuadsPerChunk   int // Maximum number of quads (faces) per chunk.
	maxIndicesPerChunk int // Maximum number of indices per chunk (6 indices per quad).

	// OpenGL buffer objects
	vertexBuffer   *openglhelper.BufferObject // Persistent mapped vertex buffer (with triple buffering)
	indexBuffer    *openglhelper.BufferObject // Shared buffer for index data with repeating pattern
	indirectBuffer *openglhelper.BufferObject // Buffer holding indirect draw commands
	chunkPosSSBO   *openglhelper.BufferObject // SSBO holding each chunk's world position

	// Pointer to the persistently mapped vertex buffer.
	vertexBufferPtr unsafe.Pointer

	// Triple buffering using a fence pool.
	fencePool       []GLSync // One fence per buffer region (3 total).
	currentFenceIdx int      // The index of the current triple buffering region.
	fenceMutex      sync.Mutex

	// Data management.
	chunkToIndexMap  map[Vec3i]int                              // Maps a chunk position to its buffer index.
	chunkPositions   []Vec3i                                    // Slice of chunk positions (indexed by draw command).
	indirectCommands []openglhelper.DrawElementsIndirectCommand // Indirect draw commands per chunk.
}

// NewChunkBufferManager creates and initializes a new ChunkBufferManager.
// It allocates OpenGL buffers for storing chunk data and sets up triple buffering.
//
// Parameters:
//   - maxChunks: The maximum number of chunks that can be managed
//   - chunkSizeBytes: The maximum vertex data size per chunk in bytes
//   - maxQuadsPerChunk: The maximum number of quads (faces) per chunk
//
// Returns a new ChunkBufferManager ready for use.
func NewChunkBufferManager(maxChunks, chunkSizeBytes, maxQuadsPerChunk int) *ChunkBufferManager {
	// Each quad uses 6 indices (two triangles)
	maxIndicesPerChunk := maxQuadsPerChunk * 6

	m := &ChunkBufferManager{
		maxChunks:          maxChunks,
		chunkSizeBytes:     chunkSizeBytes,
		maxQuadsPerChunk:   maxQuadsPerChunk,
		maxIndicesPerChunk: maxIndicesPerChunk,
		fencePool:          make([]GLSync, 3), // Triple buffering: 3 regions.
		chunkToIndexMap:    make(map[Vec3i]int),
		chunkPositions:     make([]Vec3i, maxChunks),
		indirectCommands:   make([]openglhelper.DrawElementsIndirectCommand, maxChunks),
	}
	m.createBuffers()

	// Initialize the fence pool.
	for i := range 3 {
		m.fencePool[i] = m.createFence()
	}
	return m
}

// createBuffers allocates and maps the required OpenGL buffers.
func (m *ChunkBufferManager) createBuffers() {
	// Create persistent mapped vertex buffer with triple buffering
	totalVertexSize := m.maxChunks * m.chunkSizeBytes * 3
	var err error
	m.vertexBuffer, err = openglhelper.NewPersistentBuffer(gl.ARRAY_BUFFER, totalVertexSize, false, true)
	if err != nil {
		panic("Error creating persistent vertex buffer: " + err.Error())
	}
	m.vertexBufferPtr = m.vertexBuffer.GetMappedPointer()
	if m.vertexBufferPtr == nil {
		panic("Error mapping vertex buffer!")
	}

	// Create a single shared index buffer with the repeating pattern
	// Pattern: [0,1,2,0,2,3, 4,5,6,4,6,7, ...] for all possible quads
	indexData := m.generateSharedIndexPattern(m.maxQuadsPerChunk)
	indexBufferSize := len(indexData) * 4 // 4 bytes per uint32
	m.indexBuffer = openglhelper.NewBufferObject(gl.ELEMENT_ARRAY_BUFFER, indexBufferSize, unsafe.Pointer(&indexData[0]), openglhelper.StaticDraw)

	// Create and allocate the indirect command buffer.
	indirectBufferSize := m.maxChunks * openglhelper.DrawElementsIndirectCommandSize
	m.indirectBuffer = openglhelper.NewBufferObject(gl.DRAW_INDIRECT_BUFFER, indirectBufferSize, nil, openglhelper.DynamicDraw)

	// Create and allocate the chunk position SSBO.
	ssboSize := m.maxChunks * int(unsafe.Sizeof(mgl32.Vec4{}))
	m.chunkPosSSBO = openglhelper.NewBufferObject(gl.SHADER_STORAGE_BUFFER, ssboSize, nil, openglhelper.DynamicDraw)
}

// generateSharedIndexPattern creates the index pattern for the shared index buffer.
// The pattern is a repeating sequence of indices for quads: [0,1,2, 0,2,3, 4,5,6, 4,6,7, ...].
func (m *ChunkBufferManager) generateSharedIndexPattern(maxQuads int) []uint32 {
	// 6 indices per quad (two triangles)
	indices := make([]uint32, maxQuads*6)

	for i := range maxQuads {
		// Base vertex index for this quad
		base := uint32(i * 4)

		// Two triangles per quad
		idx := i * 6

		// First triangle
		indices[idx+0] = base + 0
		indices[idx+1] = base + 1
		indices[idx+2] = base + 2

		// Second triangle
		indices[idx+3] = base + 0
		indices[idx+4] = base + 2
		indices[idx+5] = base + 3
	}

	return indices
}

// createFence creates a new OpenGL fence sync object.
func (m *ChunkBufferManager) createFence() GLSync {
	return gl.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
}

// resetFence deletes a given fence.
func (m *ChunkBufferManager) resetFence(fence GLSync) {
	gl.DeleteSync(fence)
}

// waitForFence waits until the current triple buffering region is free.
func (m *ChunkBufferManager) waitForFence() {
	m.fenceMutex.Lock()
	currentFence := m.fencePool[m.currentFenceIdx]
	m.fenceMutex.Unlock()

	// Wait up to 10ms for the GPU to finish processing.
	status := gl.ClientWaitSync(currentFence, gl.SYNC_FLUSH_COMMANDS_BIT, 10000000)
	if status == gl.TIMEOUT_EXPIRED {
		println("Fence wait timeout!")
	}

	m.fenceMutex.Lock()
	// Delete the old fence and create a new one for this region.
	m.resetFence(m.fencePool[m.currentFenceIdx])
	m.fencePool[m.currentFenceIdx] = m.createFence()
	// Advance to the next buffer region.
	m.currentFenceIdx = (m.currentFenceIdx + 1) % len(m.fencePool)
	m.fenceMutex.Unlock()
}

// AddChunk adds new chunk data into the GPU buffers.
// It uploads vertex data for a chunk at the specified world position.
// If the chunk already exists, its data will be updated.
//
// Parameters:
//   - chunkPos: The world position of the chunk
//   - packedVertexData: The vertex data in a packed format
//   - numQuads: The number of quads (faces) in this chunk's mesh
//
// Panics if the data exceeds the maximum allocated size per chunk.
func (m *ChunkBufferManager) AddChunk(chunkPos Vec3i, packedVertexData []uint32, numQuads int) {
	// Ensure that the current triple buffering region is free.
	m.waitForFence()

	// Check that data sizes do not exceed the maximum allocations.
	vertexDataBytes := len(packedVertexData) * 4
	if vertexDataBytes > m.chunkSizeBytes {
		panic("Packed vertex data exceeds maximum allocated size for this chunk")
	}

	// Check the number of quads doesn't exceed the maximum
	if numQuads > m.maxQuadsPerChunk {
		panic("Number of quads exceeds maximum allocated size for this chunk")
	}

	// Get or allocate a chunk index.
	m.fenceMutex.Lock()
	chunkIndex, exists := m.chunkToIndexMap[chunkPos]
	if !exists {
		chunkIndex = m.getAvailableChunkIndex()
		m.chunkToIndexMap[chunkPos] = chunkIndex
		m.chunkPositions[chunkIndex] = chunkPos
	}
	m.fenceMutex.Unlock()

	// Compute the offset into the persistent mapped vertex buffer.
	// The vertex buffer is divided into 3 regions (triple buffering),
	// each of size regionSize = maxChunks * chunkSizeBytes.
	regionSize := m.maxChunks * m.chunkSizeBytes
	tripleRegionOffset := m.currentFenceIdx * regionSize
	vertexOffset := tripleRegionOffset + chunkIndex*m.chunkSizeBytes

	// Copy the packed vertex data into the vertex buffer.
	destPtr := unsafe.Pointer(uintptr(m.vertexBufferPtr) + uintptr(vertexOffset))
	dstSlice := unsafe.Slice((*byte)(destPtr), vertexDataBytes)
	srcSlice := unsafe.Slice((*byte)(unsafe.Pointer(&packedVertexData[0])), vertexDataBytes)
	copy(dstSlice, srcSlice)

	// Calculate number of indices based on number of quads
	numIndices := numQuads * 6

	// Update the indirect draw command for this chunk.
	cmd := openglhelper.DrawElementsIndirectCommand{
		Count:         uint32(numIndices),      // Number of indices (6 per quad)
		InstanceCount: 1,                       // One instance
		FirstIndex:    0,                       // Start at beginning of the shared index buffer
		BaseVertex:    int32(vertexOffset / 4), // Convert byte offset to element offset
		BaseInstance:  uint32(chunkIndex),
	}
	m.indirectCommands[chunkIndex] = cmd
	m.updateIndirectBuffer()

	// Also update the chunk position SSBO.
	posOffset := chunkIndex * int(unsafe.Sizeof(mgl32.Vec4{}))
	// Convert Vec3i to Vec4 (with w = 1.0).
	pos := mgl32.Vec4{chunkPos[0], chunkPos[1], chunkPos[2], 1.0}
	m.chunkPosSSBO.UpdateSubData(posOffset, int(unsafe.Sizeof(pos)), unsafe.Pointer(&pos[0]))
}

// updateIndirectBuffer writes all indirect draw commands to the GPU buffer.
func (m *ChunkBufferManager) updateIndirectBuffer() {
	m.indirectBuffer.UpdateIndirectCommands(m.indirectCommands)
}

// getAvailableChunkIndex returns an available chunk slot.
// It finds an unused slot or returns the first slot if none are available.
func (m *ChunkBufferManager) getAvailableChunkIndex() int {
	for i, pos := range m.chunkPositions {
		// Assuming a zero Vec3 indicates an unused slot.
		if pos.X() == 0 && pos.Y() == 0 && pos.Z() == 0 {
			return i
		}
	}
	// Fallback: replace the first slot.
	return 0
}

// RemoveChunk removes a chunk from the buffers given its world position.
// If the chunk doesn't exist, this method does nothing.
//
// Parameters:
//   - chunkPos: The world position of the chunk to remove
func (m *ChunkBufferManager) RemoveChunk(chunkPos Vec3i) {
	m.waitForFence()

	m.fenceMutex.Lock()
	chunkIndex, exists := m.chunkToIndexMap[chunkPos]
	if !exists {
		m.fenceMutex.Unlock()
		return
	}
	delete(m.chunkToIndexMap, chunkPos)
	m.chunkPositions[chunkIndex] = mgl32.Vec3{0, 0, 0} // Mark slot as free.
	m.fenceMutex.Unlock()

	// Clear the vertex data in the current triple buffering region.
	regionSize := m.maxChunks * m.chunkSizeBytes
	tripleRegionOffset := m.currentFenceIdx * regionSize
	vertexOffset := tripleRegionOffset + chunkIndex*m.chunkSizeBytes
	destPtr := unsafe.Pointer(uintptr(m.vertexBufferPtr) + uintptr(vertexOffset))
	clearSlice := unsafe.Slice((*byte)(destPtr), m.chunkSizeBytes)
	for i := range clearSlice {
		clearSlice[i] = 0
	}

	// Disable the indirect draw command by setting its instance count to 0.
	cmd := m.indirectCommands[chunkIndex]
	cmd.InstanceCount = 0
	m.indirectCommands[chunkIndex] = cmd
	m.updateIndirectBuffer()

	// Clear the chunk position in the SSBO.
	posOffset := chunkIndex * int(unsafe.Sizeof(mgl32.Vec4{}))
	zeroVec := mgl32.Vec4{0, 0, 0, 0}
	m.chunkPosSSBO.UpdateSubData(posOffset, int(unsafe.Sizeof(zeroVec)), unsafe.Pointer(&zeroVec[0]))
}

// Bind binds all the necessary buffers for rendering.
// This should be called before rendering chunks.
func (m *ChunkBufferManager) Bind() {
	m.vertexBuffer.Bind()
	m.indexBuffer.Bind()
	m.indirectBuffer.Bind()
	m.chunkPosSSBO.BindBase(0) // Bind to binding point 0
}

// Render renders all chunks with a single multi-draw indirect call.
// This is an efficient way to render many chunks with minimal CPU overhead.
func (m *ChunkBufferManager) Render() {
	m.Bind()
	openglhelper.MultiDrawElementsIndirect(gl.TRIANGLES, gl.UNSIGNED_INT, len(m.indirectCommands))
}

// Cleanup releases all resources used by the ChunkBufferManager.
// This should be called when the ChunkBufferManager is no longer needed,
// typically during application shutdown.
func (m *ChunkBufferManager) Cleanup() {
	// Delete all OpenGL buffers
	if m.vertexBuffer != nil {
		m.vertexBuffer.Delete()
	}
	if m.indexBuffer != nil {
		m.indexBuffer.Delete()
	}
	if m.indirectBuffer != nil {
		m.indirectBuffer.Delete()
	}
	if m.chunkPosSSBO != nil {
		m.chunkPosSSBO.Delete()
	}

	// Delete all fence sync objects
	for i, fence := range m.fencePool {
		if fence != 0 {
			gl.DeleteSync(fence)
			m.fencePool[i] = 0
		}
	}
}
