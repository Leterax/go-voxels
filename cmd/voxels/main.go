package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/leterax/go-voxels/pkg/render"
)

func init() {
	// This is needed to ensure that OpenGL functions are called from the same thread
	runtime.LockOSThread()
}

func main() {
	fmt.Println("Starting Go-Voxels...")

	// Initialize the renderer
	renderer, err := render.NewRenderer(800, 600, "Go-Voxels")
	if err != nil {
		log.Fatalf("Failed to initialize renderer: %v", err)
	}
	

	// Run the main loop
	renderer.Run()
}  