package render

import (
	"math"

	"openglhelper"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

// Camera implements a 3D camera for navigation
type Camera struct {
	// Position and orientation
	position mgl32.Vec3
	worldUp  mgl32.Vec3
	front    mgl32.Vec3
	up       mgl32.Vec3
	right    mgl32.Vec3

	// Euler angles
	yaw   float32
	pitch float32

	// Camera options
	fov         float32
	moveSpeed   float32
	rotateSpeed float32

	// Mouse state
	lastX      float64
	lastY      float64
	firstMouse bool

	// Projection
	projection mgl32.Mat4
	width      int
	height     int
}

// NewCamera creates a new camera with sensible defaults
func NewCamera(position mgl32.Vec3) *Camera {
	camera := &Camera{
		position:    position,
		worldUp:     mgl32.Vec3{0, 1, 0},  // Y-up coordinate system
		front:       mgl32.Vec3{0, 0, -1}, // Looking along negative Z
		yaw:         DefaultYaw,
		pitch:       DefaultPitch,
		fov:         DefaultFOV,
		moveSpeed:   DefaultMoveSpeed,
		rotateSpeed: DefaultRotateSpeed,
		firstMouse:  true,
		width:       800, // Default size
		height:      600,
	}

	// Initialize vectors
	camera.updateCameraVectors()

	// Initialize projection matrix
	camera.updateProjectionMatrix()

	return camera
}

// updateCameraVectors recalculates camera vectors based on Euler angles
func (c *Camera) updateCameraVectors() {
	// Calculate new front vector
	front := mgl32.Vec3{
		float32(math.Cos(float64(mgl32.DegToRad(c.yaw))) * math.Cos(float64(mgl32.DegToRad(c.pitch)))),
		float32(math.Sin(float64(mgl32.DegToRad(c.pitch)))),
		float32(math.Sin(float64(mgl32.DegToRad(c.yaw))) * math.Cos(float64(mgl32.DegToRad(c.pitch)))),
	}
	c.front = front.Normalize()

	// Re-calculate right and up vectors
	c.right = c.front.Cross(c.worldUp).Normalize()
	c.up = c.right.Cross(c.front).Normalize()
}

// updateProjectionMatrix recalculates the projection matrix
func (c *Camera) updateProjectionMatrix() {
	aspect := float32(c.width) / float32(c.height)
	c.projection = mgl32.Perspective(mgl32.DegToRad(c.fov), aspect, 0.1, 1000.0)
}

// UpdateProjectionMatrix updates the projection matrix with new dimensions
func (c *Camera) UpdateProjectionMatrix(width, height int) {
	c.width = width
	c.height = height
	c.updateProjectionMatrix()
}

// ViewMatrix returns the current view matrix
func (c *Camera) ViewMatrix() mgl32.Mat4 {
	return mgl32.LookAtV(c.position, c.position.Add(c.front), c.up)
}

// ProjectionMatrix returns the current projection matrix
func (c *Camera) ProjectionMatrix() mgl32.Mat4 {
	return c.projection
}

// Position returns the current camera position
func (c *Camera) Position() mgl32.Vec3 {
	return c.position
}

// SetPosition sets the camera position
func (c *Camera) SetPosition(pos mgl32.Vec3) {
	c.position = pos
}

// Orientation returns the current camera orientation (yaw, pitch)
func (c *Camera) Orientation() (yaw, pitch float32) {
	return c.yaw, c.pitch
}

// SetRotation sets the camera rotation angles
func (c *Camera) SetRotation(yaw, pitch float32) {
	c.yaw = yaw

	// Constrain pitch to avoid gimbal lock
	if pitch > MaxPitch {
		pitch = MaxPitch
	}
	if pitch < MinPitch {
		pitch = MinPitch
	}
	c.pitch = pitch

	c.updateCameraVectors()
}

// LookAt makes the camera look at a specific point
func (c *Camera) LookAt(target mgl32.Vec3) {
	direction := target.Sub(c.position).Normalize()

	// Calculate yaw and pitch from direction vector
	c.yaw = mgl32.RadToDeg(float32(math.Atan2(float64(direction.Z()), float64(direction.X()))))
	c.pitch = mgl32.RadToDeg(float32(math.Asin(float64(direction.Y()))))

	c.updateCameraVectors()
}

// FrontVector returns the camera's front direction vector
func (c *Camera) FrontVector() mgl32.Vec3 {
	return c.front
}

// RightVector returns the camera's right direction vector
func (c *Camera) RightVector() mgl32.Vec3 {
	return c.right
}

// UpVector returns the camera's up direction vector
func (c *Camera) UpVector() mgl32.Vec3 {
	return c.up
}

// ProcessKeyboardInput processes keyboard input for camera movement
func (c *Camera) ProcessKeyboardInput(deltaTime float32, window *openglhelper.Window) {
	speed := c.moveSpeed * deltaTime

	// Forward/Backward
	if window.GetKeyState(KeyW) == Press {
		c.position = c.position.Add(c.front.Mul(speed))
	}
	if window.GetKeyState(KeyS) == Press {
		c.position = c.position.Sub(c.front.Mul(speed))
	}

	// Left/Right
	if window.GetKeyState(KeyA) == Press {
		c.position = c.position.Sub(c.right.Mul(speed))
	}
	if window.GetKeyState(KeyD) == Press {
		c.position = c.position.Add(c.right.Mul(speed))
	}

	// Up/Down (Space/Shift)
	if window.GetKeyState(KeySpace) == Press {
		c.position = c.position.Add(c.worldUp.Mul(speed))
	}
	if window.GetKeyState(glfw.KeyLeftShift) == Press {
		c.position = c.position.Sub(c.worldUp.Mul(speed))
	}
}

// HandleMouseMovement updates camera orientation based on mouse movement
func (c *Camera) HandleMouseMovement(xpos, ypos float64) {
	if c.firstMouse {
		c.lastX = xpos
		c.lastY = ypos
		c.firstMouse = false
		return
	}

	// Calculate offset
	xoffset := float32(xpos - c.lastX)
	yoffset := float32(c.lastY - ypos) // Reversed: y ranges bottom to top

	c.lastX = xpos
	c.lastY = ypos

	// Apply sensitivity
	xoffset *= c.rotateSpeed
	yoffset *= c.rotateSpeed

	// Update camera angles
	c.yaw += xoffset
	c.pitch += yoffset

	// Constrain pitch
	if c.pitch > MaxPitch {
		c.pitch = MaxPitch
	}
	if c.pitch < MinPitch {
		c.pitch = MinPitch
	}

	// Update camera vectors
	c.updateCameraVectors()
}

// HandleMouseScroll handles mouse scroll for zoom
func (c *Camera) HandleMouseScroll(yoffset float64) {
	// Update FOV based on scroll (zoom)
	c.fov -= float32(yoffset)

	// Constrain FOV
	if c.fov < MinFOV {
		c.fov = MinFOV
	}
	if c.fov > MaxFOV {
		c.fov = MaxFOV
	}

	// Update projection matrix
	c.updateProjectionMatrix()
}

// ResetMouseState resets the first-mouse flag for smooth camera control
func (c *Camera) ResetMouseState() {
	c.firstMouse = true
}
