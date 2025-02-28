package render

import (
	"fmt"
	"log"

	"openglhelper"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Renderer handles window and input management for the voxel world
type Renderer struct {
	window *openglhelper.Window
	camera *Camera

	// Timing
	lastFrameTime float64
	deltaTime     float32
	totalTime     float32

	// Debug settings
	debugEnabled bool

	// State management
	running  bool
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
		window:       window,
		camera:       camera,
		debugEnabled: false,
	}

	// Set up callbacks
	window.GLFWWindow().SetKeyCallback(renderer.keyCallback)
	window.GLFWWindow().SetCursorPosCallback(renderer.cursorPosCallback)
	window.GLFWWindow().SetMouseButtonCallback(renderer.mouseButtonCallback)
	window.GLFWWindow().SetScrollCallback(renderer.scrollCallback)
	window.GLFWWindow().SetFramebufferSizeCallback(renderer.framebufferSizeCallback)

	return renderer, nil
}

func (r *Renderer) OnRender(totalTime, frameTime float32) {
	// clear the screen
	r.window.Clear(mgl32.Vec4{51. / 255., 51. / 255., 51. / 255., 1.0})
}

// Debug logs a message if debug mode is enabled
func (r *Renderer) Debug(format string, args ...any) {
	if r.debugEnabled {
		log.Printf(format, args...)
	}
}

// ShouldClose returns whether the window should close
func (r *Renderer) ShouldClose() bool {
	return r.window.ShouldClose()
}

// Rum runs the main loop
func (r *Renderer) Run() {
	// Make sure the window is set as the current context
	r.window.GLFWWindow().MakeContextCurrent()

	r.lastFrameTime = glfw.GetTime()
	r.running = true

	// Main loop
	for r.running && !r.window.ShouldClose() {
		// Calculate delta time
		currentTime := glfw.GetTime()
		r.deltaTime = float32(currentTime - r.lastFrameTime)
		r.lastFrameTime = currentTime
		r.totalTime += r.deltaTime

		// Process input and update camera
		r.processKeyboardInput()

		// Render the scene
		r.OnRender(r.totalTime, r.deltaTime)

		// Swap buffers and poll events
		r.window.SwapBuffers()
		glfw.PollEvents()
	}
}

// Cleanup releases all resources used by the renderer
func (r *Renderer) Cleanup() {
	if r.isClosed {
		return
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

	// Toggle debug mode with D key
	if key == glfw.KeyF && action == glfw.Press {
		r.debugEnabled = !r.debugEnabled
		log.Printf("Debug mode: %v", r.debugEnabled)
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

// GetCamera returns the camera instance
func (r *Renderer) GetCamera() *Camera {
	return r.camera
}

// SetCameraPosition sets the camera position in world space
func (r *Renderer) SetCameraPosition(position mgl32.Vec3) {
	r.camera.SetPosition(position)
}

// SetCameraLookAt makes the camera look at a target position
func (r *Renderer) SetCameraLookAt(target mgl32.Vec3) {
	r.camera.LookAt(target)
}

// processKeyboardInput processes keyboard input and updates the camera
func (r *Renderer) processKeyboardInput() {
	// Process keyboard input for camera movement
	r.camera.ProcessKeyboardInput(r.deltaTime, r.window)
}
