package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/leterax/go-voxels/pkg/render"
)

func init() {
	// This is needed to ensure that OpenGL functions are called from the same thread
	runtime.LockOSThread()
}

func main() {
	fmt.Println("Starting Go-Voxels...")

	// Parse command line flags
	serverAddr := flag.String("server", "", "Server address (empty for singleplayer)")
	playerName := flag.String("name", "Player", "Player name")
	renderDist := flag.Int("renderdist", 8, "Render distance (in chunks)")
	flag.Parse()

	// Initialize the renderer
	renderer, err := render.NewRenderer(800, 600, "Go-Voxels")
	if err != nil {
		log.Fatalf("Failed to initialize renderer: %v", err)
	}

	// Position camera for a better view of the chunks
	renderer.SetCameraPosition(mgl32.Vec3{0, 25, 35})
	renderer.SetCameraLookAt(mgl32.Vec3{0, 0, 0})

	renderer.Run()
}
