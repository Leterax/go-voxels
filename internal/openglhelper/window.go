package openglhelper

import (
	"fmt"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Window handles GLFW window creation and management
type Window struct {
	glfwWindow    *glfw.Window
	width         int
	height        int
	title         string
	mouseCaptured bool
	vsync         bool
}

// NewWindow creates a new GLFW window with OpenGL context
func NewWindow(width, height int, title string, vsync bool) (*Window, error) {
	// Initialize GLFW
	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize GLFW: %w", err)
	}

	// Configure GLFW
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 6)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.True)

	// Create window
	glfwWindow, err := glfw.CreateWindow(width, height, title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("failed to create GLFW window: %w", err)
	}

	glfwWindow.MakeContextCurrent()
	if vsync {
		glfw.SwapInterval(1) // Enable vsync
	} else {
		glfw.SwapInterval(0) // Disable vsync
	}

	// Initialize OpenGL
	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize OpenGL: %w", err)
	}

	// Print OpenGL version
	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Printf("OpenGL version: %s\n", version)

	// Configure global OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)

	return &Window{
		glfwWindow:    glfwWindow,
		width:         width,
		height:        height,
		title:         title,
		mouseCaptured: false,
	}, nil
}

// Clear clears the screen
func (w *Window) Clear(color mgl32.Vec4) {
	// Set clear color to a dark purple to distinguish from black objects
	gl.ClearColor(color.X(), color.Y(), color.Z(), color.W())
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

// SwapBuffers swaps the front and back buffers
func (w *Window) SwapBuffers() {
	w.glfwWindow.SwapBuffers()
}

// PollEvents processes pending events
func (w *Window) PollEvents() {
	glfw.PollEvents()
}

// ShouldClose returns whether the window should close
func (w *Window) ShouldClose() bool {
	return w.glfwWindow.ShouldClose()
}

// Close releases all resources
func (w *Window) Close() {
	glfw.Terminate()
}

// Size returns the window dimensions
func (w *Window) Size() (width, height int) {
	return w.width, w.height
}

// SetSize sets the window dimensions
func (w *Window) SetSize(width, height int) {
	w.width = width
	w.height = height
	w.glfwWindow.SetSize(width, height)
}

// SetTitle sets the window title
func (w *Window) SetTitle(title string) {
	w.title = title
	w.glfwWindow.SetTitle(title)
}

// GetKeyState returns the state of the given key
func (w *Window) GetKeyState(key glfw.Key) glfw.Action {
	return w.glfwWindow.GetKey(key)
}

// OnResize is called when the window is resized
func (w *Window) OnResize(width, height int) {
	w.width = width
	w.height = height
	gl.Viewport(0, 0, int32(width), int32(height))
}

// GLFWWindow returns the underlying GLFW window
func (w *Window) GLFWWindow() *glfw.Window {
	return w.glfwWindow
}

// SetMouseCaptured captures or releases the mouse cursor
func (w *Window) SetMouseCaptured(captured bool) {
	w.mouseCaptured = captured

	if captured {
		w.glfwWindow.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	} else {
		w.glfwWindow.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	}
}

// ToggleMouseCaptured toggles the mouse capture state
func (w *Window) ToggleMouseCaptured() {
	w.SetMouseCaptured(!w.mouseCaptured)
}

// IsMouseCaptured returns whether the mouse is currently captured
func (w *Window) IsMouseCaptured() bool {
	return w.mouseCaptured
}
