package main

import (
	"log"
	"runtime"

	"github.com/leterax/go-voxels/pkg/render"
	"github.com/leterax/go-voxels/pkg/render/examples"
)

func init() {
	// This is needed to ensure that the OpenGL functions are called from the same thread
	runtime.LockOSThread()
}

func main() {
	// Create window
	window, err := render.NewWindow(800, 600, "Go Voxels - Cube Example")
	if err != nil {
		log.Fatalf("Failed to create window: %v", err)
	}
	defer window.Close()

	// Run the cube example
	examples.CubeExample(window)
}
