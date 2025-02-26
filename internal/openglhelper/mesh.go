package openglhelper

import (
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Vertex represents a 3D vertex with position, normal, and texture coordinates
type Vertex struct {
	Position  mgl32.Vec3
	Normal    mgl32.Vec3
	TexCoords mgl32.Vec2
}

// Mesh represents a 3D mesh with vertices and indices
type Mesh struct {
	vao      *VertexArrayObject
	vbo      *BufferObject
	ebo      *BufferObject
	indices  []uint32
	vertices []float32
	shader   *Shader
}

// NewMesh creates a new mesh from vertices and indices
func NewMesh(vertices []float32, indices []uint32, shader *Shader) *Mesh {
	// Create VAO, VBO, and EBO
	vao := NewVAO()
	vao.Bind()

	vbo := NewVBO(vertices)
	ebo := NewEBO(indices)

	// Position attribute (3 floats)
	vao.SetVertexAttribPointer(0, 3, gl.FLOAT, false, 8*4, 0)
	// Normal attribute (3 floats)
	vao.SetVertexAttribPointer(1, 3, gl.FLOAT, false, 8*4, 3*4)
	// Texture coordinates attribute (2 floats)
	vao.SetVertexAttribPointer(2, 2, gl.FLOAT, false, 8*4, 6*4)

	// Unbind VAO
	vao.Unbind()

	return &Mesh{
		vao:      vao,
		vbo:      vbo,
		ebo:      ebo,
		indices:  indices,
		vertices: vertices,
		shader:   shader,
	}
}

// Draw renders the mesh
func (m *Mesh) Draw() {
	m.shader.Use()
	m.vao.Bind()
	gl.DrawElements(gl.TRIANGLES, int32(len(m.indices)), gl.UNSIGNED_INT, nil)
	m.vao.Unbind()
}

// Delete releases all resources
func (m *Mesh) Delete() {
	m.vao.Delete()
	m.vbo.Delete()
	m.ebo.Delete()
}

// SetShader sets the shader for the mesh
func (m *Mesh) SetShader(shader *Shader) {
	m.shader = shader
}

// NewCube creates a new cube mesh
func NewCube(shader *Shader) *Mesh {
	// Cube vertices: position (3), normal (3), texture coordinates (2)
	vertices := []float32{
		// Front face
		-0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 0.0, 0.0, // Bottom-left
		0.5, -0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 0.0, // Bottom-right
		0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 1.0, 1.0, // Top-right
		-0.5, 0.5, 0.5, 0.0, 0.0, 1.0, 0.0, 1.0, // Top-left

		// Back face
		-0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 0.0, // Bottom-left
		-0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 1.0, 1.0, // Top-left
		0.5, 0.5, -0.5, 0.0, 0.0, -1.0, 0.0, 1.0, // Top-right
		0.5, -0.5, -0.5, 0.0, 0.0, -1.0, 0.0, 0.0, // Bottom-right

		// Top face
		-0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 0.0, 1.0, // Back-left
		-0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 0.0, 0.0, // Front-left
		0.5, 0.5, 0.5, 0.0, 1.0, 0.0, 1.0, 0.0, // Front-right
		0.5, 0.5, -0.5, 0.0, 1.0, 0.0, 1.0, 1.0, // Back-right

		// Bottom face
		-0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 0.0, 0.0, // Back-left
		0.5, -0.5, -0.5, 0.0, -1.0, 0.0, 1.0, 0.0, // Back-right
		0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 1.0, 1.0, // Front-right
		-0.5, -0.5, 0.5, 0.0, -1.0, 0.0, 0.0, 1.0, // Front-left

		// Right face
		0.5, -0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 0.0, // Bottom-back
		0.5, 0.5, -0.5, 1.0, 0.0, 0.0, 1.0, 1.0, // Top-back
		0.5, 0.5, 0.5, 1.0, 0.0, 0.0, 0.0, 1.0, // Top-front
		0.5, -0.5, 0.5, 1.0, 0.0, 0.0, 0.0, 0.0, // Bottom-front

		// Left face
		-0.5, -0.5, -0.5, -1.0, 0.0, 0.0, 0.0, 0.0, // Bottom-back
		-0.5, -0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 0.0, // Bottom-front
		-0.5, 0.5, 0.5, -1.0, 0.0, 0.0, 1.0, 1.0, // Top-front
		-0.5, 0.5, -0.5, -1.0, 0.0, 0.0, 0.0, 1.0, // Top-back
	}

	// Cube indices
	indices := []uint32{
		0, 1, 2, 2, 3, 0, // Front face
		4, 5, 6, 6, 7, 4, // Back face
		8, 9, 10, 10, 11, 8, // Top face
		12, 13, 14, 14, 15, 12, // Bottom face
		16, 17, 18, 18, 19, 16, // Right face
		20, 21, 22, 22, 23, 20, // Left face
	}

	return NewMesh(vertices, indices, shader)
}
