package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/leterax/go-voxels/pkg/render"
	"github.com/leterax/go-voxels/pkg/voxel"
)

func init() {
	// This is needed to ensure that OpenGL functions are called from the same thread
	runtime.LockOSThread()

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())
}

func main() {
	fmt.Println("Starting Go-Voxels...")

	// Initialize the renderer
	renderer, err := render.NewRenderer(800, 600, "Go-Voxels")
	if err != nil {
		log.Fatalf("Failed to initialize renderer: %v", err)
	}

	// Position camera for a better view of the chunks
	renderer.SetCameraPosition(mgl32.Vec3{0, 25, 35})
	renderer.SetCameraLookAt(mgl32.Vec3{0, 0, 0})

	// generate some chunks
	chunks := generateWorld()

	// Run the main loop
	renderer.Run(chunks)
}

func generateWorld() []*voxel.Chunk {
	chunks := make([]*voxel.Chunk, 10)

	// Create chunks at different positions in a 3x3 grid plus one above the center
	positions := [][3]int32{
		{-1, 0, -1}, {0, 0, -1}, {1, 0, -1},
		{-1, 0, 0}, {0, 0, 0}, {1, 0, 0},
		{-1, 0, 1}, {0, 0, 1}, {1, 0, 1},
		{0, 1, 0}, // One chunk above the center
	}

	for i := 0; i < 10; i++ {
		// Create chunk at the specified position
		chunks[i] = voxel.NewChunk(positions[i][0], positions[i][1], positions[i][2], 16)
		fillChunk(chunks[i])

		// Generate mesh for the chunk - important for rendering!
		chunks[i].GeneratePackedMesh()
	}
	return chunks
}

// fillChunk fills a chunk with blocks according to a heightmap
func fillChunk(chunk *voxel.Chunk) {
	for x := 0; x < chunk.Size; x++ {
		for z := 0; z < chunk.Size; z++ {
			// Create a more interesting heightmap using sine waves
			height := int(math.Sin(float64(x)/5.0)*3.0 + math.Cos(float64(z)/5.0)*3.0 + 8)

			// Clamp height to ensure it stays within chunk
			if height < 0 {
				height = 0
			}
			if height >= chunk.Size {
				height = chunk.Size - 1
			}

			// Fill the column from bottom to height
			for y := 0; y < height; y++ {
				// Determine block type based on height
				var blockType voxel.BlockType

				if y == height-1 {
					// Top layer is grass
					blockType = voxel.Grass
				} else if y > height-4 {
					// A few layers below top are dirt
					blockType = voxel.Dirt
				} else {
					// Bottom layers are stone
					blockType = voxel.Stone
				}

				// Add some random features
				if y == height && rand.Float64() < 0.05 {
					// 5% chance to place a gold block on top
					blockType = voxel.GoldBlock
				}

				chunk.SetBlock(x, y, z, blockType)
			}

			// Water in the lower areas
			if height < 5 {
				for y := height; y < 5; y++ {
					chunk.SetBlock(x, y, z, voxel.Water)
				}
			}
		}
	}
}
