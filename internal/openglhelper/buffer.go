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
