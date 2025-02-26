package render

import (
	"fmt"
	"openglhelper"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Renderer handles rendering logic and game loop
type Renderer struct {
	window *openglhelper.Window
	camera *Camera

	cubeShader *openglhelper.Shader
	cube       *openglhelper.Mesh
	// Timing
	lastFrameTime float64
	deltaTime     float32
	totalTime     float32

	// FPS tracking
	frameCount int
	totalFPS   float32
}

// NewRenderer creates a new renderer with the specified dimensions and title
func NewRenderer(width, height int, title string) (*Renderer, error) {
	// Create window
	window, err := openglhelper.NewWindow(width, height, title, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	// Create initial camera position
	cameraPos := mgl32.Vec3{0, 0, 3} // Start slightly back from the origin
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

	// Print control instructions
	fmt.Println("Control instructions:")
	fmt.Println("- WASD: Camera movement")
	fmt.Println("- Mouse: Look around")
	fmt.Println("- Space/Shift: Move up/down")
	fmt.Println("- C: Toggle mouse capture")
	fmt.Println("- Escape: Close the application")

	fmt.Println("\nLoading shaders...")
	shader, err := openglhelper.LoadShaderFromFiles("pkg/render/shaders/vert.glsl", "pkg/render/shaders/frag.glsl")
	if err != nil {
		return nil, fmt.Errorf("failed to load shader: %w", err)
	}
	fmt.Println("Shader loaded successfully!")
	fmt.Println("Shader ID:", shader.ID)

	// Position the light to the upper right front
	lightPos := mgl32.Vec3{10.0, 10.0, 10.0}
	// Bright white light
	lightColor := mgl32.Vec3{1.0, 1.0, 1.0}
	// Vibrant red color for the cube (high contrast to make lighting obvious)
	objectColor := mgl32.Vec3{1.0, 0.2, 0.2}

	shader.SetVec3("lightPos", lightPos)
	shader.SetVec3("lightColor", lightColor)
	shader.SetVec3("objectColor", objectColor)
	// Set initial camera position
	shader.SetVec3("viewPos", camera.Position())

	renderer.cubeShader = shader

	// cube mesh stuff (vertex buffer, index buffer, vertex array object)
	cube := openglhelper.NewCube(shader)
	renderer.cube = cube

	return renderer, nil
}

// render is where you implement your rendering logic
func (r *Renderer) render(totalTime float32, deltaTime float32) {
	// Get view and projection matrices from camera
	view := r.camera.ViewMatrix()
	projection := r.camera.ProjectionMatrix()

	// Set up the shader with the matrices
	r.cubeShader.Use()
	r.cubeShader.SetMat4("view", view)
	r.cubeShader.SetMat4("projection", projection)

	// Always update camera position uniform for correct specular highlights
	r.cubeShader.SetVec3("viewPos", r.camera.Position())

	// Debugging - Print visual confirmation once that the shader is running
	if r.totalTime < 0.1 { // Only print once at the start
		fmt.Println("\nStarting rendering...")
		fmt.Println("Using shader ID:", r.cubeShader.ID)
		fmt.Println("Using simple Phong lighting with ambient and diffuse")
		fmt.Println("Cube should appear red with lighting variation across sides")
	}

	// Position the light at a fixed position for consistent lighting
	// This makes it easier to debug lighting issues
	lightPos := mgl32.Vec3{2.0, 2.0, 2.0}
	r.cubeShader.SetVec3("lightPos", lightPos)

	// Create model matrix with reasonable transformation
	model := mgl32.Ident4()
	// Position in front of camera
	model = model.Mul4(mgl32.Translate3D(0, 0, -1.5))
	// Rotate slowly around Y axis to show all sides
	model = model.Mul4(mgl32.HomogRotate3D(totalTime*0.3, mgl32.Vec3{0, 1, 0}))

	r.cubeShader.SetMat4("model", model)

	// Draw the cube
	r.cube.Draw()

	// Print frame completion message once
	if r.totalTime < 0.05 {
		fmt.Println("First frame rendered")
	}
}

// Run starts the main rendering loop
func (r *Renderer) Run() {
	// Main loop
	for !r.window.ShouldClose() {
		// Calculate delta time
		currentTime := glfw.GetTime()
		r.deltaTime = float32(currentTime - r.lastFrameTime)
		r.lastFrameTime = currentTime
		r.totalTime += r.deltaTime

		// Process camera input
		r.camera.ProcessKeyboardInput(r.deltaTime, r.window)

		// Clear the screen
		r.window.Clear()

		// Render frame
		r.render(r.totalTime, r.deltaTime)

		// Swap buffers and poll events
		r.window.SwapBuffers()
		r.window.PollEvents()

		// Track FPS for average calculation
		if r.deltaTime > 0 {
			currentFPS := 1.0 / r.deltaTime
			r.totalFPS += currentFPS
			r.frameCount++
		}
	}

	// Calculate and display average FPS before cleanup
	if r.frameCount > 0 {
		avgFPS := r.totalFPS / float32(r.frameCount)
		fmt.Printf("\nApplication closing. Average FPS: %.1f over %d frames\n", avgFPS, r.frameCount)
	}

	// Cleanup
	r.Cleanup()
}

func (r *Renderer) Cleanup() {
	r.window.Close()
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
