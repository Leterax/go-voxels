package openglhelper

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
)

// BufferObject represents an OpenGL buffer object (VBO, EBO, SSBO, etc.)
type BufferObject struct {
	ID         uint32
	Type       uint32         // GL_ARRAY_BUFFER, GL_ELEMENT_ARRAY_BUFFER, GL_SHADER_STORAGE_BUFFER, etc.
	Size       int            // Size of the buffer in bytes
	Usage      uint32         // GL_STATIC_DRAW, GL_DYNAMIC_DRAW, etc.
	IsMapped   bool           // Whether the buffer is currently mapped
	MappedPtr  unsafe.Pointer // Pointer to mapped memory (if mapped)
	Persistent bool           // Whether the buffer is persistently mapped
}

// BufferUsage represents different buffer usage patterns
type BufferUsage uint32

const (
	// Static buffers - data is set once and used many times
	StaticDraw BufferUsage = gl.STATIC_DRAW
	StaticRead BufferUsage = gl.STATIC_READ
	StaticCopy BufferUsage = gl.STATIC_COPY

	// Dynamic buffers - data is changed frequently and used many times
	DynamicDraw BufferUsage = gl.DYNAMIC_DRAW
	DynamicRead BufferUsage = gl.DYNAMIC_READ
	DynamicCopy BufferUsage = gl.DYNAMIC_COPY

	// Stream buffers - data is set once and used a few times
	StreamDraw BufferUsage = gl.STREAM_DRAW
	StreamRead BufferUsage = gl.STREAM_READ
	StreamCopy BufferUsage = gl.STREAM_COPY
)

// VertexArrayObject represents an OpenGL vertex array object (VAO)
type VertexArrayObject struct {
	ID uint32
}

// DrawElementsIndirectCommand represents the structure for a single indirect draw command
type DrawElementsIndirectCommand struct {
	Count         uint32 // Number of indices to render
	InstanceCount uint32 // Number of instances to render
	FirstIndex    uint32 // Starting index in the index buffer
	BaseVertex    int32  // Base vertex to add to each index
	BaseInstance  uint32 // Base instance for instanced attributes
}

// Size of the DrawElementsIndirectCommand struct in bytes
const DrawElementsIndirectCommandSize = 20 // 5 * 4 bytes

// NewVBO creates a new Vertex Buffer Object
func NewVBO(data []float32, usage BufferUsage) *BufferObject {
	var vboID uint32
	gl.GenBuffers(1, &vboID)

	vbo := &BufferObject{
		ID:    vboID,
		Type:  gl.ARRAY_BUFFER,
		Size:  4 * len(data),
		Usage: uint32(usage),
	}

	vbo.Bind()
	gl.BufferData(gl.ARRAY_BUFFER, vbo.Size, gl.Ptr(data), vbo.Usage)

	return vbo
}

// NewDynamicVBO creates a new dynamic VBO optimized for frequent updates
func NewDynamicVBO(sizeInBytes int) *BufferObject {
	var vboID uint32
	gl.GenBuffers(1, &vboID)

	vbo := &BufferObject{
		ID:    vboID,
		Type:  gl.ARRAY_BUFFER,
		Size:  sizeInBytes,
		Usage: uint32(DynamicDraw),
	}

	vbo.Bind()
	gl.BufferData(gl.ARRAY_BUFFER, vbo.Size, nil, vbo.Usage)

	return vbo
}

// NewEBO creates a new Element Buffer Object (Index Buffer)
func NewEBO(indices []uint32, usage BufferUsage) *BufferObject {
	var eboID uint32
	gl.GenBuffers(1, &eboID)

	ebo := &BufferObject{
		ID:    eboID,
		Type:  gl.ELEMENT_ARRAY_BUFFER,
		Size:  4 * len(indices),
		Usage: uint32(usage),
	}

	ebo.Bind()
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, ebo.Size, gl.Ptr(indices), ebo.Usage)

	return ebo
}

// NewSSBO creates a new Shader Storage Buffer Object
func NewSSBO(dataSize int, data unsafe.Pointer, usage BufferUsage) *BufferObject {
	var ssboID uint32
	gl.GenBuffers(1, &ssboID)

	ssbo := &BufferObject{
		ID:    ssboID,
		Type:  gl.SHADER_STORAGE_BUFFER,
		Size:  dataSize,
		Usage: uint32(usage),
	}

	ssbo.Bind()
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, ssbo.Size, data, ssbo.Usage)

	return ssbo
}

// NewEmptySSBO creates an SSBO with no initial data
func NewEmptySSBO(sizeInBytes int, usage BufferUsage) *BufferObject {
	return NewSSBO(sizeInBytes, nil, usage)
}

// NewPersistentBuffer creates a buffer that can be persistently mapped
// This allows the application to keep the buffer mapped while using it with OpenGL
func NewPersistentBuffer(type_ uint32, sizeInBytes int, read, write bool) (*BufferObject, error) {
	var bufferID uint32
	gl.GenBuffers(1, &bufferID)

	buffer := &BufferObject{
		ID:         bufferID,
		Type:       type_,
		Size:       sizeInBytes,
		Persistent: true,
	}

	// Determine flags based on read/write requirements
	var flags uint32 = gl.MAP_PERSISTENT_BIT | gl.MAP_COHERENT_BIT

	if read {
		flags |= gl.MAP_READ_BIT
	}
	if write {
		flags |= gl.MAP_WRITE_BIT
	}

	// Create immutable storage for the buffer with appropriate flags
	buffer.Bind()
	gl.BufferStorage(type_, buffer.Size, nil, flags)

	// Map the buffer immediately
	err := buffer.MapPersistent(read, write)
	if err != nil {
		buffer.Delete()
		return nil, err
	}

	return buffer, nil
}

// MapPersistent maps the buffer persistently so it can be accessed while in use by OpenGL
func (bo *BufferObject) MapPersistent(read, write bool) error {
	if bo.IsMapped {
		return fmt.Errorf("buffer is already mapped")
	}

	// Determine flags based on read/write requirements
	var flags uint32 = gl.MAP_PERSISTENT_BIT | gl.MAP_COHERENT_BIT

	if read {
		flags |= gl.MAP_READ_BIT
	}
	if write {
		flags |= gl.MAP_WRITE_BIT
	}

	bo.Bind()
	bo.MappedPtr = gl.MapBufferRange(bo.Type, 0, bo.Size, flags)

	if bo.MappedPtr == nil {
		return fmt.Errorf("failed to map buffer")
	}

	bo.IsMapped = true
	bo.Persistent = true

	return nil
}

// Unmap unmaps a mapped buffer
func (bo *BufferObject) Unmap() bool {
	if !bo.IsMapped {
		return false
	}

	bo.Bind()
	success := gl.UnmapBuffer(bo.Type)

	if success {
		bo.IsMapped = false
		bo.MappedPtr = nil
	}

	return success
}

// GetMappedPointer returns the pointer to the mapped buffer memory
func (bo *BufferObject) GetMappedPointer() unsafe.Pointer {
	if !bo.IsMapped {
		return nil
	}
	return bo.MappedPtr
}

// Bind binds the buffer object
func (bo *BufferObject) Bind() {
	gl.BindBuffer(bo.Type, bo.ID)
}

// Unbind unbinds the buffer object
func (bo *BufferObject) Unbind() {
	gl.BindBuffer(bo.Type, 0)
}

// BindBase binds a buffer to an indexed buffer target
func (bo *BufferObject) BindBase(index uint32) {
	gl.BindBufferBase(bo.Type, index, bo.ID)
}

// BindRange binds a range of a buffer to an indexed buffer target
func (bo *BufferObject) BindRange(index uint32, offset int, size int) {
	gl.BindBufferRange(bo.Type, index, bo.ID, offset, size)
}

// UpdateData updates the entire buffer with new data
func (bo *BufferObject) UpdateData(data unsafe.Pointer) {
	bo.Bind()
	gl.BufferSubData(bo.Type, 0, bo.Size, data)
}

// UpdateSubData updates a portion of the buffer with new data
func (bo *BufferObject) UpdateSubData(offset int, size int, data unsafe.Pointer) {
	bo.Bind()
	gl.BufferSubData(bo.Type, offset, size, data)
}

// InvalidateData invalidates the buffer data to prepare for full update
func (bo *BufferObject) InvalidateData() {
	bo.Bind()
	gl.InvalidateBufferData(bo.ID)
}

// InvalidateSubData invalidates a portion of the buffer data
func (bo *BufferObject) InvalidateSubData(offset int, length int) {
	bo.Bind()
	gl.InvalidateBufferSubData(bo.ID, offset, length)
}

// Orphan orphans the buffer by reallocating its data store
// This is useful for dynamic buffers to avoid waiting for GPU operations to finish
func (bo *BufferObject) Orphan() {
	bo.Bind()
	gl.BufferData(bo.Type, bo.Size, nil, bo.Usage)
}

// Delete releases the buffer object
func (bo *BufferObject) Delete() {
	if bo.IsMapped {
		bo.Unmap()
	}
	gl.DeleteBuffers(1, &bo.ID)
}

// NewVAO creates a new Vertex Array Object
func NewVAO() *VertexArrayObject {
	var vaoID uint32
	gl.GenVertexArrays(1, &vaoID)

	return &VertexArrayObject{
		ID: vaoID,
	}
}

// Bind binds the vertex array object
func (vao *VertexArrayObject) Bind() {
	gl.BindVertexArray(vao.ID)
}

// Unbind unbinds the vertex array object
func (vao *VertexArrayObject) Unbind() {
	gl.BindVertexArray(0)
}

// Delete releases the vertex array object
func (vao *VertexArrayObject) Delete() {
	gl.DeleteVertexArrays(1, &vao.ID)
}

// SetVertexAttribPointer sets up a vertex attribute pointer
func (vao *VertexArrayObject) SetVertexAttribPointer(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset int) {
	gl.VertexAttribPointer(index, size, xtype, normalized, stride, gl.PtrOffset(offset))
	gl.EnableVertexAttribArray(index)
}

// NewIndirectBuffer creates a buffer for multi-draw indirect commands
func NewIndirectBuffer(maxCommands int, usage BufferUsage) *BufferObject {
	sizeInBytes := maxCommands * DrawElementsIndirectCommandSize

	var bufferID uint32
	gl.GenBuffers(1, &bufferID)

	buffer := &BufferObject{
		ID:    bufferID,
		Type:  gl.DRAW_INDIRECT_BUFFER,
		Size:  sizeInBytes,
		Usage: uint32(usage),
	}

	buffer.Bind()
	gl.BufferData(gl.DRAW_INDIRECT_BUFFER, buffer.Size, nil, buffer.Usage)

	return buffer
}

// UpdateIndirectCommands updates the indirect buffer with an array of draw commands
func (bo *BufferObject) UpdateIndirectCommands(commands []DrawElementsIndirectCommand) {
	if bo.Type != gl.DRAW_INDIRECT_BUFFER {
		panic("Buffer is not an indirect buffer")
	}

	bo.Bind()

	// Calculate size in bytes
	sizeInBytes := len(commands) * DrawElementsIndirectCommandSize

	// Only update if we have commands and they fit in the buffer
	if len(commands) > 0 && sizeInBytes <= bo.Size {
		gl.BufferSubData(gl.DRAW_INDIRECT_BUFFER, 0, sizeInBytes, gl.Ptr(commands))
	}
}

// NewPackedVertexBuffer creates a VBO specifically for packed vertices (uint32 format)
func NewPackedVertexBuffer(vertices []uint32, usage BufferUsage) *BufferObject {
	var vboID uint32
	gl.GenBuffers(1, &vboID)

	vbo := &BufferObject{
		ID:    vboID,
		Type:  gl.ARRAY_BUFFER,
		Size:  4 * len(vertices), // uint32 = 4 bytes
		Usage: uint32(usage),
	}

	vbo.Bind()
	gl.BufferData(gl.ARRAY_BUFFER, vbo.Size, gl.Ptr(vertices), vbo.Usage)

	return vbo
}

// MultiDrawElementsIndirect is a convenience function for drawing multiple batches
// with a single call
func MultiDrawElementsIndirect(mode uint32, indexType uint32, commandCount int) {
	gl.MultiDrawElementsIndirect(mode, indexType, nil, int32(commandCount), 0)
}

// TripleBuffer manages a triple-buffered persistent buffer for efficient CPU-GPU data transfer
type TripleBuffer struct {
	Buffer           *BufferObject  // The underlying buffer object
	NumBuffers       int            // Number of buffer sections (typically 3)
	BufferSize       int            // Size of each buffer section in bytes
	CurrentBufferIdx int            // Index of the buffer section currently being written to
	BufferOffsets    []int          // Offsets for each buffer section in float32 units
	SyncObjects      []uintptr      // Fence sync objects for each buffer section
	MappedMemory     unsafe.Pointer // Pointer to the mapped memory
	MappedFloats     []float32      // Go slice backed by persistent mapped memory (as float32)
	MappedUints      []uint32       // Go slice backed by persistent mapped memory (as uint32)
}

// NewTripleBuffer creates a new triple-buffered persistent buffer
func NewTripleBuffer(bufferType uint32, sectionSizeBytes int, numBuffers int) (*TripleBuffer, error) {
	if numBuffers < 2 {
		numBuffers = 2 // At least double buffering
	}

	totalSize := sectionSizeBytes * numBuffers

	// Create the persistent buffer
	buffer, err := NewPersistentBuffer(bufferType, totalSize, false, true)
	if err != nil {
		return nil, err
	}

	// Create the triple buffer
	tb := &TripleBuffer{
		Buffer:           buffer,
		NumBuffers:       numBuffers,
		BufferSize:       sectionSizeBytes,
		CurrentBufferIdx: 0,
		BufferOffsets:    make([]int, numBuffers),
		SyncObjects:      make([]uintptr, numBuffers),
		MappedMemory:     buffer.MappedPtr,
	}

	// Calculate offsets for each buffer section
	for i := 0; i < numBuffers; i++ {
		tb.BufferOffsets[i] = i * (sectionSizeBytes / 4) // Convert bytes to float32/uint32 offset
	}

	// Create slices that view the mapped memory
	tb.MappedFloats = unsafe.Slice((*float32)(tb.MappedMemory), totalSize/4)
	tb.MappedUints = unsafe.Slice((*uint32)(tb.MappedMemory), totalSize/4)

	return tb, nil
}

// CurrentOffsetFloats returns the offset of the current buffer section in float32 units
func (tb *TripleBuffer) CurrentOffsetFloats() int {
	return tb.BufferOffsets[tb.CurrentBufferIdx]
}

// CurrentOffsetBytes returns the offset of the current buffer section in bytes
func (tb *TripleBuffer) CurrentOffsetBytes() int {
	return tb.BufferOffsets[tb.CurrentBufferIdx] * 4
}

// WaitForSync waits until the GPU has finished using the current buffer section
func (tb *TripleBuffer) WaitForSync() {
	// Early exit if no sync object exists
	if tb.SyncObjects[tb.CurrentBufferIdx] == 0 {
		return
	}

	const timeout uint64 = 1000000000 // 1 second in nanoseconds

	for {
		waitReturn := gl.ClientWaitSync(tb.SyncObjects[tb.CurrentBufferIdx], gl.SYNC_FLUSH_COMMANDS_BIT, timeout)
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

// CreateFenceSync creates a fence sync object for the current buffer
func (tb *TripleBuffer) CreateFenceSync() {
	// Delete any existing sync object
	if tb.SyncObjects[tb.CurrentBufferIdx] != 0 {
		gl.DeleteSync(tb.SyncObjects[tb.CurrentBufferIdx])
	}

	// Create a new sync object
	tb.SyncObjects[tb.CurrentBufferIdx] = gl.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
}

// Advance moves to the next buffer section
func (tb *TripleBuffer) Advance() {
	tb.CurrentBufferIdx = (tb.CurrentBufferIdx + 1) % tb.NumBuffers
}

// WriteFloats writes float32 data to the current buffer section
func (tb *TripleBuffer) WriteFloats(data []float32, offsetInSection int) {
	if offsetInSection+len(data) > tb.BufferSize/4 {
		panic("data would exceed buffer section size")
	}

	baseOffset := tb.BufferOffsets[tb.CurrentBufferIdx] + offsetInSection
	copy(tb.MappedFloats[baseOffset:baseOffset+len(data)], data)
}

// WriteUints writes uint32 data to the current buffer section
func (tb *TripleBuffer) WriteUints(data []uint32, offsetInSection int) {
	if offsetInSection+len(data) > tb.BufferSize/4 {
		panic("data would exceed buffer section size")
	}

	baseOffset := tb.BufferOffsets[tb.CurrentBufferIdx] + offsetInSection
	copy(tb.MappedUints[baseOffset:baseOffset+len(data)], data)
}

// Cleanup releases all resources
func (tb *TripleBuffer) Cleanup() {
	// Delete sync objects
	for i, sync := range tb.SyncObjects {
		if sync != 0 {
			gl.DeleteSync(sync)
			tb.SyncObjects[i] = 0
		}
	}

	// Delete buffer
	if tb.Buffer != nil {
		tb.Buffer.Delete()
		tb.Buffer = nil
	}
}
