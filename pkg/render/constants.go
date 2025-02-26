package render

import (
	"github.com/go-gl/glfw/v3.3/glfw"
)

// Key constants for keyboard input
const (
	KeyW        = glfw.KeyW
	KeyA        = glfw.KeyA
	KeyS        = glfw.KeyS
	KeyD        = glfw.KeyD
	KeySpace    = glfw.KeySpace
	KeyEscape   = glfw.KeyEscape
	KeyLeftCtrl = glfw.KeyLeftControl
	KeyX        = glfw.KeyX
	KeyU        = glfw.KeyU
)

// Action constants for key states
const (
	Press   = glfw.Press
	Release = glfw.Release
	Repeat  = glfw.Repeat
)

// Camera constants
const (
	// Movement speeds
	DefaultMoveSpeed   = 10.0
	DefaultRotateSpeed = 0.1

	// Default orientation
	DefaultYaw   = -90.0 // Facing -Z direction
	DefaultPitch = 0.0

	// Field of view
	DefaultFOV = 45.0
	MinFOV     = 1.0
	MaxFOV     = 45.0

	// Constraints
	MaxPitch = 89.0
	MinPitch = -89.0
)
