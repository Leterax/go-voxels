package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/leterax/go-voxels/pkg/game"
	"github.com/leterax/go-voxels/pkg/network"
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

	var chunkManager *game.ChunkManager

	// Check if we should connect to a server
	if *serverAddr != "" {
		// Multiplayer mode - connect to the server
		chunkManager = setupMultiplayerMode(renderer, *serverAddr, *playerName, uint8(*renderDist))

		// Run the main loop with network chunk updates
		runNetworkMode(renderer, chunkManager)
	} else {
		// Singleplayer mode - generate local world
		chunks := generateWorld()

		// Run the main loop with static chunks
		renderer.Run(chunks)
	}
}

// setupMultiplayerMode sets up the network client and chunk manager
func setupMultiplayerMode(renderer *render.Renderer, serverAddr, playerName string, renderDist uint8) *game.ChunkManager {
	fmt.Println("Connecting to server:", serverAddr)

	// Connect to the server
	client, err := network.NewClient(serverAddr)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	fmt.Println("Connected to server")
	// Set player name and render distance
	client.SetEntityName(playerName)
	client.SetRenderDistance(renderDist)

	// Send initial metadata to the server
	if err := client.SendClientMetadata(); err != nil {
		log.Fatalf("Failed to send client metadata: %v", err)
	}

	// Create chunk manager to handle received chunks
	chunkManager := game.NewChunkManager(client, renderDist)

	// Start a goroutine to process network packets
	go func() {
		if err := client.ProcessPackets(); err != nil {
			log.Printf("Network error: %v", err)
		}
	}()

	return chunkManager
}

// runNetworkMode runs the game loop with network-received chunks
func runNetworkMode(renderer *render.Renderer, chunkManager *game.ChunkManager) {
	// Set up initial OpenGL state (similar to renderer.Run)
	renderer.SetupOpenGL()

	// Debug stats
	var frameCount int
	lastStatsTime := time.Now()

	// Track if we need to update draw commands
	haveChunksChanged := true

	// Initial chunks and positions
	var chunks []*voxel.Chunk

	// Run the main loop, continuously getting the latest chunks from the ChunkManager
	for !renderer.ShouldClose() {
		// Get the latest chunks from the chunk manager only if needed
		newChunksAvailable := chunkManager.HaveChunksChanged()

		if newChunksAvailable {
			chunks = chunkManager.GetChunks()
			haveChunksChanged = true
			fmt.Println("Chunks have changed, updating renderer...")
		}

		// Update debug stats once per second
		frameCount++
		if time.Since(lastStatsTime) >= time.Second {
			fps := frameCount
			chunkCount := len(chunks)

			fmt.Printf("FPS: %d, Chunks: %d\n", fps, chunkCount)

			lastStatsTime = time.Now()
			frameCount = 0
		}

		// Only update draw commands when chunks have changed
		if haveChunksChanged && len(chunks) > 0 {
			renderer.UpdateDrawCommands(chunks)
			haveChunksChanged = false
		}

		// Process one frame with the current chunks
		renderer.RenderFrame(chunks)
	}

	// Clean up resources
	chunkManager.Cleanup()
	renderer.Cleanup()
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
		chunks[i].GenerateMesh()
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
