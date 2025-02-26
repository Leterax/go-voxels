package openglhelper

import (
	"github.com/go-gl/gl/v4.6-core/gl"
)

// BufferObject represents an OpenGL buffer object (VBO or EBO)
type BufferObject struct {
	ID   uint32
	Type uint32 // GL_ARRAY_BUFFER or GL_ELEMENT_ARRAY_BUFFER
}

// VertexArrayObject represents an OpenGL vertex array object (VAO)
type VertexArrayObject struct {
	ID uint32
}

// NewVBO creates a new Vertex Buffer Object
func NewVBO(data []float32) *BufferObject {
	var vboID uint32
	gl.GenBuffers(1, &vboID)

	vbo := &BufferObject{
		ID:   vboID,
		Type: gl.ARRAY_BUFFER,
	}

	vbo.Bind()
	gl.BufferData(gl.ARRAY_BUFFER, 4*len(data), gl.Ptr(data), gl.STATIC_DRAW)

	return vbo
}

// NewEBO creates a new Element Buffer Object (Index Buffer)
func NewEBO(indices []uint32) *BufferObject {
	var eboID uint32
	gl.GenBuffers(1, &eboID)

	ebo := &BufferObject{
		ID:   eboID,
		Type: gl.ELEMENT_ARRAY_BUFFER,
	}

	ebo.Bind()
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, 4*len(indices), gl.Ptr(indices), gl.STATIC_DRAW)

	return ebo
}

// Bind binds the buffer object
func (bo *BufferObject) Bind() {
	gl.BindBuffer(bo.Type, bo.ID)
}

// Unbind unbinds the buffer object
func (bo *BufferObject) Unbind() {
	gl.BindBuffer(bo.Type, 0)
}

// Delete releases the buffer object
func (bo *BufferObject) Delete() {
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
