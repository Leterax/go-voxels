package openglhelper

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Shader represents an OpenGL shader program
type Shader struct {
	ID uint32
}

// compileShader compiles a single shader
func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source + "\x00")
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile shader: %v", log)
	}

	return shader, nil
}

// NewShader creates a new shader program from vertex and fragment shader source
func NewShader(vertexShaderSource, fragmentShaderSource string) (*Shader, error) {
	program, err := newProgram(vertexShaderSource, fragmentShaderSource)
	if err != nil {
		return nil, err
	}

	return &Shader{ID: program}, nil
}

// newProgram creates a shader program from vertex and fragment shader sources
func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, fmt.Errorf("vertex shader compilation failed: %w", err)
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, fmt.Errorf("fragment shader compilation failed: %w", err)
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

// Use activates the shader program
func (s *Shader) Use() {
	gl.UseProgram(s.ID)
}

// Delete releases the shader program
func (s *Shader) Delete() {
	gl.DeleteProgram(s.ID)
}

// SetBool sets a boolean uniform
func (s *Shader) SetBool(name string, value bool) {
	var intValue int32
	if value {
		intValue = 1
	}
	gl.Uniform1i(gl.GetUniformLocation(s.ID, gl.Str(name+"\x00")), intValue)
}

// SetInt sets an integer uniform
func (s *Shader) SetInt(name string, value int32) {
	gl.Uniform1i(gl.GetUniformLocation(s.ID, gl.Str(name+"\x00")), value)
}

// SetFloat sets a float uniform
func (s *Shader) SetFloat(name string, value float32) {
	gl.Uniform1f(gl.GetUniformLocation(s.ID, gl.Str(name+"\x00")), value)
}

// SetVec3 sets a vec3 uniform
func (s *Shader) SetVec3(name string, vec mgl32.Vec3) {
	gl.Uniform3f(gl.GetUniformLocation(s.ID, gl.Str(name+"\x00")), vec[0], vec[1], vec[2])
}

// SetMat4 sets a mat4 uniform
func (s *Shader) SetMat4(name string, mat mgl32.Mat4) {
	gl.UniformMatrix4fv(gl.GetUniformLocation(s.ID, gl.Str(name+"\x00")), 1, false, &mat[0])
}

// LoadShaderFromFiles loads a shader program from vertex and fragment shader files
func LoadShaderFromFiles(vertexPath, fragmentPath string) (*Shader, error) {
	// Read vertex shader
	vertexSource, err := os.ReadFile(vertexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vertex shader file: %w", err)
	}

	// Read fragment shader
	fragmentSource, err := os.ReadFile(fragmentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read fragment shader file: %w", err)
	}

	// Create shader program
	return NewShader(string(vertexSource), string(fragmentSource))
}
