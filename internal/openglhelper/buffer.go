// Package openglhelper provides utilities for working with OpenGL buffers and other resources.
// It wraps the low-level OpenGL functions in a more Go-friendly API.
package openglhelper

import (
	"fmt"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
)

// DrawElementsIndirectCommand represents the structure for a single indirect draw command
// used with OpenGL's multi-draw indirect rendering.
type DrawElementsIndirectCommand struct {
	Count         uint32 // Number of indices to render
	InstanceCount uint32 // Number of instances to render
	FirstIndex    uint32 // Starting index in the index buffer
	BaseVertex    int32  // Base vertex to add to each index
	BaseInstance  uint32 // Base instance for instanced attributes
}

// Size of the DrawElementsIndirectCommand struct in bytes
const DrawElementsIndirectCommandSize = int(unsafe.Sizeof(DrawElementsIndirectCommand{}))

// BufferObject represents an OpenGL buffer object (VBO, EBO, SSBO, etc.)
// It provides a higher-level abstraction over raw OpenGL buffer IDs and operations.
type BufferObject struct {
	ID         uint32
	Type       uint32         // GL_ARRAY_BUFFER, GL_ELEMENT_ARRAY_BUFFER, GL_SHADER_STORAGE_BUFFER, etc.
	Size       int            // Size of the buffer in bytes
	Usage      uint32         // GL_STATIC_DRAW, GL_DYNAMIC_DRAW, etc.
	IsMapped   bool           // Whether the buffer is currently mapped
	MappedPtr  unsafe.Pointer // Pointer to mapped memory (if mapped)
	Persistent bool           // Whether the buffer is persistently mapped
}

// BufferUsage represents different buffer usage patterns for OpenGL buffers.
type BufferUsage uint32

const (
	// StaticDraw indicates buffer contents will be specified once and used many times for drawing
	StaticDraw BufferUsage = gl.STATIC_DRAW
	// StaticRead indicates buffer contents will be specified once and read many times by the application
	StaticRead BufferUsage = gl.STATIC_READ
	// StaticCopy indicates buffer contents will be specified once and used many times to copy data
	StaticCopy BufferUsage = gl.STATIC_COPY

	// DynamicDraw indicates buffer contents will be changed frequently and used many times for drawing
	DynamicDraw BufferUsage = gl.DYNAMIC_DRAW
	// DynamicRead indicates buffer contents will be changed frequently and read many times by the application
	DynamicRead BufferUsage = gl.DYNAMIC_READ
	// DynamicCopy indicates buffer contents will be changed frequently and used many times to copy data
	DynamicCopy BufferUsage = gl.DYNAMIC_COPY

	// StreamDraw indicates buffer contents will be specified once and used a few times for drawing
	StreamDraw BufferUsage = gl.STREAM_DRAW
	// StreamRead indicates buffer contents will be specified once and read a few times by the application
	StreamRead BufferUsage = gl.STREAM_READ
	// StreamCopy indicates buffer contents will be specified once and used a few times to copy data
	StreamCopy BufferUsage = gl.STREAM_COPY
)

// VertexArrayObject represents an OpenGL vertex array object (VAO) that stores vertex attribute configurations.
type VertexArrayObject struct {
	ID uint32
}

// NewBufferObject creates a general buffer object with the specified parameters.
// It returns a new BufferObject initialized with the given type, size, data, and usage.
func NewBufferObject(bufferType uint32, sizeInBytes int, data unsafe.Pointer, usage BufferUsage) *BufferObject {
	var bufferID uint32
	gl.GenBuffers(1, &bufferID)

	buffer := &BufferObject{
		ID:    bufferID,
		Type:  bufferType,
		Size:  sizeInBytes,
		Usage: uint32(usage),
	}

	buffer.Bind()
	gl.BufferData(bufferType, sizeInBytes, data, uint32(usage))

	return buffer
}

// NewPersistentBuffer creates a buffer that can be persistently mapped.
// This allows CPU and GPU to simultaneously access the buffer.
// The read and write parameters determine whether the buffer will be mapped for reading and/or writing.
// Returns the created buffer object and any error that occurred during creation.
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

// MapPersistent maps the buffer persistently so it can be accessed while in use by OpenGL.
// Returns an error if mapping fails or if the buffer is already mapped.
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

// Unmap unmaps a mapped buffer.
// Returns true if the buffer was successfully unmapped, false otherwise.
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

// GetMappedPointer returns the pointer to the mapped buffer memory.
// Returns nil if the buffer is not mapped.
func (bo *BufferObject) GetMappedPointer() unsafe.Pointer {
	if !bo.IsMapped {
		return nil
	}
	return bo.MappedPtr
}

// Bind binds the buffer object to its type target.
func (bo *BufferObject) Bind() {
	gl.BindBuffer(bo.Type, bo.ID)
}

// Unbind unbinds the buffer object from its type target.
func (bo *BufferObject) Unbind() {
	gl.BindBuffer(bo.Type, 0)
}

// BindBase binds a buffer to an indexed buffer target.
// Used for shader storage buffers, uniform buffers, etc.
func (bo *BufferObject) BindBase(index uint32) {
	gl.BindBufferBase(bo.Type, index, bo.ID)
}

// UpdateData updates the entire buffer with new data.
func (bo *BufferObject) UpdateData(data unsafe.Pointer) {
	bo.Bind()
	gl.BufferSubData(bo.Type, 0, bo.Size, data)
}

// UpdateSubData updates a portion of the buffer with new data.
// The offset is in bytes from the start of the buffer.
func (bo *BufferObject) UpdateSubData(offset int, size int, data unsafe.Pointer) {
	bo.Bind()
	gl.BufferSubData(bo.Type, offset, size, data)
}

// Delete releases the buffer object and frees its resources.
// It automatically unmaps the buffer if it is mapped.
func (bo *BufferObject) Delete() {
	if bo.IsMapped {
		bo.Unmap()
	}
	gl.DeleteBuffers(1, &bo.ID)
}

// NewVAO creates a new Vertex Array Object.
// It returns a pointer to a new VertexArrayObject.
func NewVAO() *VertexArrayObject {
	var vaoID uint32
	gl.GenVertexArrays(1, &vaoID)

	return &VertexArrayObject{
		ID: vaoID,
	}
}

// Bind binds the vertex array object.
func (vao *VertexArrayObject) Bind() {
	gl.BindVertexArray(vao.ID)
}

// Unbind unbinds the vertex array object.
func (vao *VertexArrayObject) Unbind() {
	gl.BindVertexArray(0)
}

// Delete releases the vertex array object and frees its resources.
func (vao *VertexArrayObject) Delete() {
	gl.DeleteVertexArrays(1, &vao.ID)
}

// SetVertexAttribPointer sets up a vertex attribute pointer and enables the attribute.
// This configures how OpenGL will interpret vertex data for a specific attribute.
func (vao *VertexArrayObject) SetVertexAttribPointer(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset int) {
	gl.VertexAttribPointer(index, size, xtype, normalized, stride, gl.PtrOffset(offset))
	gl.EnableVertexAttribArray(index)
}

// NewIndirectBuffer creates a buffer for multi-draw indirect commands.
// Returns a new buffer object configured for indirect drawing commands.
func NewIndirectBuffer(maxCommands int, usage BufferUsage) *BufferObject {
	sizeInBytes := maxCommands * DrawElementsIndirectCommandSize

	return NewBufferObject(gl.DRAW_INDIRECT_BUFFER, sizeInBytes, nil, usage)
}

// UpdateIndirectCommands updates the indirect buffer with an array of draw commands.
// Panics if the buffer is not an indirect buffer or if the commands don't fit in the buffer.
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
	} else if len(commands) > 0 {
		panic("Buffer is not large enough to hold the commands")
	}
}

// MultiDrawElementsIndirect is a convenience function for drawing multiple batches with a single call.
// The mode parameter specifies the primitive type (e.g., gl.TRIANGLES).
// The indexType parameter specifies the type of indices (e.g., gl.UNSIGNED_INT).
// The commandCount parameter specifies the number of draw commands to execute.
func MultiDrawElementsIndirect(mode uint32, indexType uint32, commandCount int) {
	gl.MultiDrawElementsIndirect(mode, indexType, nil, int32(commandCount), 0)
}

// TripleBuffer manages a triple-buffered persistent buffer for efficient CPU-GPU data transfer.
// It helps avoid stalls by allowing the CPU to write to one buffer while the GPU reads from another.
type TripleBuffer struct {
	Buffer           *BufferObject  // The underlying buffer object
	NumBuffers       int            // Number of buffer sections (typically 3)
	BufferSize       int            // Size of each buffer section in bytes
	CurrentBufferIdx int            // Index of the buffer section currently being written to
	BufferOffsets    []int          // Offsets for each buffer section in bytes
	SyncObjects      []uintptr      // Fence sync objects for each buffer section
	MappedMemory     unsafe.Pointer // Pointer to the mapped memory
}

// NewTripleBuffer creates a new triple-buffered persistent buffer.
// The bufferType parameter specifies the buffer target.
// The sectionSizeBytes parameter specifies the size of each buffer section in bytes.
// The numBuffers parameter specifies the number of buffer sections (typically 3).
// Returns the created triple buffer and any error that occurred during creation.
func NewTripleBuffer(bufferType uint32, sectionSizeBytes int, numBuffers int) (*TripleBuffer, error) {
	if numBuffers < 2 {
		numBuffers = 3 // Default to triple buffering
	}

	totalSize := sectionSizeBytes * numBuffers

	// Create the persistent buffer with write flag
	buffer, err := NewPersistentBuffer(bufferType, totalSize, false, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create persistent buffer: %w", err)
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
	for i := range numBuffers {
		tb.BufferOffsets[i] = i * sectionSizeBytes
	}

	return tb, nil
}

// WaitForSync waits until the GPU has finished using the current buffer section.
// Returns true if synchronization was successful, false otherwise.
func (tb *TripleBuffer) WaitForSync() bool {
	// Early exit if no sync object exists
	if tb.SyncObjects[tb.CurrentBufferIdx] == 0 {
		return true
	}

	const timeout uint64 = 10000000 // 10 milliseconds in nanoseconds

	waitReturn := gl.ClientWaitSync(tb.SyncObjects[tb.CurrentBufferIdx], gl.SYNC_FLUSH_COMMANDS_BIT, timeout)

	// Once we're done waiting, delete the sync object
	if tb.SyncObjects[tb.CurrentBufferIdx] != 0 {
		gl.DeleteSync(tb.SyncObjects[tb.CurrentBufferIdx])
		tb.SyncObjects[tb.CurrentBufferIdx] = 0
	}

	return waitReturn == gl.ALREADY_SIGNALED || waitReturn == gl.CONDITION_SATISFIED
}

// CreateFenceSync creates a fence sync object for the current buffer.
// This marks the point in the command stream when all previous commands must complete.
func (tb *TripleBuffer) CreateFenceSync() {
	// Delete any existing sync object
	if tb.SyncObjects[tb.CurrentBufferIdx] != 0 {
		gl.DeleteSync(tb.SyncObjects[tb.CurrentBufferIdx])
	}

	// Create a new sync object
	tb.SyncObjects[tb.CurrentBufferIdx] = gl.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
}

// Advance moves to the next buffer section in the rotation.
// This allows for triple buffering by cycling through the available buffer sections.
func (tb *TripleBuffer) Advance() {
	tb.CurrentBufferIdx = (tb.CurrentBufferIdx + 1) % tb.NumBuffers
}

// CurrentOffsetBytes returns the offset of the current buffer section in bytes.
// This is useful for calculating memory offsets when writing to the buffer.
func (tb *TripleBuffer) CurrentOffsetBytes() int {
	return tb.BufferOffsets[tb.CurrentBufferIdx]
}

// Cleanup releases all resources associated with the triple buffer.
// This should be called when the triple buffer is no longer needed.
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
