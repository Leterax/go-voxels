package game

import (
	"sync"

	"github.com/leterax/go-voxels/pkg/network"
	"github.com/leterax/go-voxels/pkg/voxel"
)

// ChunkManager handles the management of chunks received from the network
// It ensures thread-safe access and processing of chunks
type ChunkManager struct {
	chunks         map[ChunkCoord]*voxel.Chunk
	chunksMutex    sync.RWMutex
	chunkQueue     chan chunkJob
	client         *network.Client
	stopWorker     chan struct{}
	workerStopped  chan struct{}
	renderDistance uint8

	// Flag to track when chunks have changed
	chunksChanged      bool
	chunksChangedMutex sync.RWMutex
}

// ChunkCoord represents the x,y,z coordinates of a chunk
type ChunkCoord struct {
	X, Y, Z int32
}

// chunkJob represents a job to process a chunk
type chunkJob struct {
	coord     ChunkCoord
	blocks    []voxel.BlockType
	monoType  bool
	blockType voxel.BlockType
}

// NewChunkManager creates a new chunk manager
func NewChunkManager(client *network.Client, renderDistance uint8) *ChunkManager {
	cm := &ChunkManager{
		chunks:         make(map[ChunkCoord]*voxel.Chunk),
		chunkQueue:     make(chan chunkJob, 100), // Buffer for 100 chunk jobs
		client:         client,
		stopWorker:     make(chan struct{}),
		workerStopped:  make(chan struct{}),
		renderDistance: renderDistance,
		chunksChanged:  true, // Initial state is changed to build first draw commands
	}

	// Set up network callbacks
	client.OnChunkReceive = cm.handleChunkReceive
	client.OnMonoChunk = cm.handleMonoChunk

	// Start the worker goroutine
	go cm.chunkWorker()

	return cm
}

// handleChunkReceive is called when a full chunk is received from the network
func (cm *ChunkManager) handleChunkReceive(x, y, z int32, blocks []voxel.BlockType) {
	// Queue the chunk for processing
	cm.chunkQueue <- chunkJob{
		coord:    ChunkCoord{X: x, Y: y, Z: z},
		blocks:   blocks,
		monoType: false,
	}
}

// handleMonoChunk is called when a mono-type chunk is received from the network
func (cm *ChunkManager) handleMonoChunk(x, y, z int32, blockType voxel.BlockType) {
	// Queue the mono-type chunk for processing
	cm.chunkQueue <- chunkJob{
		coord:     ChunkCoord{X: x, Y: y, Z: z},
		monoType:  true,
		blockType: blockType,
	}
}

// markChunksChanged sets the flag indicating chunks have changed
func (cm *ChunkManager) markChunksChanged() {
	cm.chunksChangedMutex.Lock()
	cm.chunksChanged = true
	cm.chunksChangedMutex.Unlock()
}

// resetChunksChanged resets the changed flag and returns previous state
func (cm *ChunkManager) resetChunksChanged() bool {
	cm.chunksChangedMutex.Lock()
	defer cm.chunksChangedMutex.Unlock()

	prevState := cm.chunksChanged
	cm.chunksChanged = false
	return prevState
}

// chunkWorker processes chunks in the background
func (cm *ChunkManager) chunkWorker() {
	defer close(cm.workerStopped)

	for {
		select {
		case <-cm.stopWorker:
			return
		case job := <-cm.chunkQueue:
			// Process the chunk job
			if job.monoType {
				cm.processMonoChunk(job.coord, job.blockType)
			} else {
				cm.processFullChunk(job.coord, job.blocks)
			}

			// Mark that chunks have changed
			cm.markChunksChanged()
		}
	}
}

// processMonoChunk generates a chunk with a single block type
func (cm *ChunkManager) processMonoChunk(coord ChunkCoord, blockType voxel.BlockType) {
	// Create a new chunk using the world coordinates directly
	// The WorldPosition() method in the Chunk will handle conversion to world space
	chunk := voxel.NewChunk(coord.X, coord.Y, coord.Z, network.ChunkSize)

	// Fill the chunk with the mono block type
	chunk.FillWithBlockType(blockType)

	// Generate mesh
	if blockType == voxel.Air {
		// For air chunks, we don't need a mesh
		chunk.Mesh = voxel.NewMesh()
	} else {
		// For non-air mono chunks, we can use a simplified mesh generation
		chunk.Mesh = cm.generateMonoChunkMesh(chunk, blockType)
	}

	// Store the chunk
	cm.chunksMutex.Lock()
	cm.chunks[coord] = chunk
	cm.chunksMutex.Unlock()
}

// processFullChunk processes a full chunk with mixed block types
func (cm *ChunkManager) processFullChunk(coord ChunkCoord, blocks []voxel.BlockType) {
	// Create a new chunk from the received blocks using the world coordinates directly
	chunk := voxel.NewChunkFromBlocks(coord.X, coord.Y, coord.Z, network.ChunkSize, blocks)

	// Generate mesh using greedy meshing
	chunk.GeneratePackedMesh()

	// Store the chunk
	cm.chunksMutex.Lock()
	cm.chunks[coord] = chunk
	cm.chunksMutex.Unlock()
}

// generateMonoChunkMesh generates a mesh for a mono-type chunk without using greedy meshing
func (cm *ChunkManager) generateMonoChunkMesh(chunk *voxel.Chunk, blockType voxel.BlockType) *voxel.Mesh {
	// Create a new mesh
	mesh := voxel.NewMesh()

	// For a mono-type chunk, we only need faces on the six sides of the chunk
	// This is much faster than running the full greedy meshing algorithm

	size := chunk.Size

	// Only create faces on the surfaces of the chunk
	// We create 6 faces, one for each side of the chunk

	// Define offsets for different orientations
	// Orientation values (0-5): +X, -X, +Y, -Y, +Z, -Z

	// Helper function to add a face quad
	addFaceQuad := func(orientation int, x, y, z int) {
		// Get vertices for this face based on orientation
		var vertices [4]uint32

		switch orientation {
		case 0: // +X face
			vertices[0] = voxel.PackVertex(size, y, z, 0, 0, 0, int(blockType), 7)
			vertices[1] = voxel.PackVertex(size, y+1, z, 0, 1, 0, int(blockType), 7)
			vertices[2] = voxel.PackVertex(size, y+1, z+1, 1, 1, 0, int(blockType), 7)
			vertices[3] = voxel.PackVertex(size, y, z+1, 1, 0, 0, int(blockType), 7)
		case 1: // -X face
			vertices[0] = voxel.PackVertex(0, y, z+1, 0, 0, 1, int(blockType), 7)
			vertices[1] = voxel.PackVertex(0, y+1, z+1, 0, 1, 1, int(blockType), 7)
			vertices[2] = voxel.PackVertex(0, y+1, z, 1, 1, 1, int(blockType), 7)
			vertices[3] = voxel.PackVertex(0, y, z, 1, 0, 1, int(blockType), 7)
		case 2: // +Y face
			vertices[0] = voxel.PackVertex(x, size, z, 0, 0, 2, int(blockType), 7)
			vertices[1] = voxel.PackVertex(x, size, z+1, 0, 1, 2, int(blockType), 7)
			vertices[2] = voxel.PackVertex(x+1, size, z+1, 1, 1, 2, int(blockType), 7)
			vertices[3] = voxel.PackVertex(x+1, size, z, 1, 0, 2, int(blockType), 7)
		case 3: // -Y face
			vertices[0] = voxel.PackVertex(x, 0, z+1, 0, 0, 3, int(blockType), 7)
			vertices[1] = voxel.PackVertex(x, 0, z, 0, 1, 3, int(blockType), 7)
			vertices[2] = voxel.PackVertex(x+1, 0, z, 1, 1, 3, int(blockType), 7)
			vertices[3] = voxel.PackVertex(x+1, 0, z+1, 1, 0, 3, int(blockType), 7)
		case 4: // +Z face
			vertices[0] = voxel.PackVertex(x, y, size, 0, 0, 4, int(blockType), 7)
			vertices[1] = voxel.PackVertex(x+1, y, size, 0, 1, 4, int(blockType), 7)
			vertices[2] = voxel.PackVertex(x+1, y+1, size, 1, 1, 4, int(blockType), 7)
			vertices[3] = voxel.PackVertex(x, y+1, size, 1, 0, 4, int(blockType), 7)
		case 5: // -Z face
			vertices[0] = voxel.PackVertex(x, y+1, 0, 0, 0, 5, int(blockType), 7)
			vertices[1] = voxel.PackVertex(x+1, y+1, 0, 0, 1, 5, int(blockType), 7)
			vertices[2] = voxel.PackVertex(x+1, y, 0, 1, 1, 5, int(blockType), 7)
			vertices[3] = voxel.PackVertex(x, y, 0, 1, 0, 5, int(blockType), 7)
		}

		mesh.AddPackedFace(vertices)
	}

	// For each orientation add face quads
	// +X face (right)
	for y := 0; y < size; y++ {
		for z := 0; z < size; z++ {
			addFaceQuad(0, 0, y, z)
		}
	}

	// -X face (left)
	for y := 0; y < size; y++ {
		for z := 0; z < size; z++ {
			addFaceQuad(1, 0, y, z)
		}
	}

	// +Y face (top)
	for x := 0; x < size; x++ {
		for z := 0; z < size; z++ {
			addFaceQuad(2, x, 0, z)
		}
	}

	// -Y face (bottom)
	for x := 0; x < size; x++ {
		for z := 0; z < size; z++ {
			addFaceQuad(3, x, 0, z)
		}
	}

	// +Z face (front)
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			addFaceQuad(4, x, y, 0)
		}
	}

	// -Z face (back)
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			addFaceQuad(5, x, y, 0)
		}
	}

	return mesh
}

// GetChunks returns a slice of all chunks for rendering
func (cm *ChunkManager) GetChunks() []*voxel.Chunk {
	cm.chunksMutex.RLock()
	defer cm.chunksMutex.RUnlock()

	chunks := make([]*voxel.Chunk, 0, len(cm.chunks))
	for _, chunk := range cm.chunks {
		chunks = append(chunks, chunk)
	}

	return chunks
}

// HaveChunksChanged returns true if chunks have been added or removed since
// the last time this method was called
func (cm *ChunkManager) HaveChunksChanged() bool {
	return cm.resetChunksChanged()
}

// Cleanup stops the worker goroutine
func (cm *ChunkManager) Cleanup() {
	close(cm.stopWorker)
	<-cm.workerStopped
}
