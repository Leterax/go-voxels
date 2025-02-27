package game

import (
	"log"
	"sync"

	"github.com/leterax/go-voxels/pkg/network"
	"github.com/leterax/go-voxels/pkg/voxel"
)

// ChunkManager handles the management of chunks received from the network
// It ensures thread-safe access and processing of chunks
type ChunkManager struct {
	chunks         map[voxel.ChunkCoord]*voxel.Chunk
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

// chunkJob represents a job to process a chunk
type chunkJob struct {
	coord     voxel.ChunkCoord
	blocks    []voxel.BlockType
	monoType  bool
	blockType voxel.BlockType
}

// NewChunkManager creates a new chunk manager
func NewChunkManager(client *network.Client, renderDistance uint8) *ChunkManager {
	cm := &ChunkManager{
		chunks:         make(map[voxel.ChunkCoord]*voxel.Chunk),
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
	// x,y,z are in world coordinates, so we need to convert them to chunk coordinates
	positionInChunkCoords := voxel.WorldToChunkCoord(x, y, z, network.ChunkSize)
	cm.queueChunkJob(voxel.ChunkCoord{X: positionInChunkCoords.X, Y: positionInChunkCoords.Y, Z: positionInChunkCoords.Z}, blocks, false, voxel.Air)
}

// handleMonoChunk is called when a mono-type chunk is received from the network
func (cm *ChunkManager) handleMonoChunk(x, y, z int32, blockType voxel.BlockType) {
	// x,y,z are in world coordinates, so we need to convert them to chunk coordinates
	positionInChunkCoords := voxel.WorldToChunkCoord(x, y, z, network.ChunkSize)
	cm.queueChunkJob(voxel.ChunkCoord{X: positionInChunkCoords.X, Y: positionInChunkCoords.Y, Z: positionInChunkCoords.Z}, nil, true, blockType)
}

// queueChunkJob adds a chunk processing job to the queue
func (cm *ChunkManager) queueChunkJob(coord voxel.ChunkCoord, blocks []voxel.BlockType, monoType bool, blockType voxel.BlockType) {
	cm.chunkQueue <- chunkJob{
		coord:     coord,
		blocks:    blocks,
		monoType:  monoType,
		blockType: blockType,
	}
}

// markChunksChanged sets the flag indicating chunks have changed
func (cm *ChunkManager) markChunksChanged() {
	cm.chunksChangedMutex.Lock()
	cm.chunksChanged = true
	cm.chunksChangedMutex.Unlock()
}

// UpdatePlayerPosition sends player position updates to the server
func (cm *ChunkManager) UpdatePlayerPosition(x, y, z, yaw, pitch float32) error {
	if cm.client != nil {
		// Log the position update
		// log.Printf("ChunkManager: Sending position update to server: (%v, %v, %v), yaw=%v, pitch=%v",
		// 	x, y, z, yaw, pitch)
		return cm.client.SendUpdateEntity(x, y, z, yaw, pitch)
	}
	log.Printf("ChunkManager: Client not initialized, cannot send position update")
	return nil
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
func (cm *ChunkManager) processMonoChunk(coord voxel.ChunkCoord, blockType voxel.BlockType) {
	// Create a new chunk using the world coordinates directly
	chunk := voxel.NewChunk(coord.X, coord.Y, coord.Z, network.ChunkSize)

	// Fill the chunk with the mono block type
	chunk.FillWithBlockType(blockType)

	// For non-air mono chunks, we can use a simplified mesh generation
	chunk.Mesh = voxel.MonoChunkMesh(chunk, blockType)

	// Store the chunk
	cm.storeChunk(coord, chunk)
}

// processFullChunk processes a full chunk with mixed block types
func (cm *ChunkManager) processFullChunk(coord voxel.ChunkCoord, blocks []voxel.BlockType) {
	// Create a new chunk from the received blocks using the world coordinates directly
	chunk := voxel.NewChunkFromBlocks(coord.X, coord.Y, coord.Z, network.ChunkSize, blocks)

	// Generate mesh using greedy meshing
	chunk.GenerateMesh()

	// Store the chunk
	cm.storeChunk(coord, chunk)
}

// storeChunk stores a chunk in the chunks map with proper locking
func (cm *ChunkManager) storeChunk(coord voxel.ChunkCoord, chunk *voxel.Chunk) {
	cm.chunksMutex.Lock()
	cm.chunks[coord] = chunk
	cm.chunksMutex.Unlock()
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

// GetNewChunks returns chunks that have been added since the last call
// and clears the new chunks list
func (cm *ChunkManager) GetNewChunks() []*voxel.Chunk {
	cm.chunksMutex.RLock()
	defer cm.chunksMutex.RUnlock()

	// Get chunks that have been marked as changed
	if !cm.resetChunksChanged() {
		return nil
	}

	// Return all available chunks - the renderer will filter as needed
	chunks := make([]*voxel.Chunk, 0, len(cm.chunks))
	for _, chunk := range cm.chunks {
		chunks = append(chunks, chunk)
	}

	return chunks
}

// RemoveDistantChunks removes chunks that are too far from the given position
func (cm *ChunkManager) RemoveDistantChunks(playerX, playerY, playerZ int32) {
	// Convert player position to chunk coordinates
	playerChunkPos := voxel.WorldToChunkCoord(playerX, playerY, playerZ, network.ChunkSize)

	// Maximum distance squared (render distance in chunks)
	maxDistSquared := int32(cm.renderDistance * cm.renderDistance)

	chunksToRemove := []voxel.ChunkCoord{}

	// Find chunks that are too far away
	cm.chunksMutex.RLock()
	for coord, _ := range cm.chunks {
		// Calculate distance squared between chunk and player
		dx := coord.X - playerChunkPos.X
		dy := coord.Y - playerChunkPos.Y
		dz := coord.Z - playerChunkPos.Z
		distSquared := dx*dx + dy*dy + dz*dz

		if distSquared > maxDistSquared {
			chunksToRemove = append(chunksToRemove, coord)
		}
	}
	cm.chunksMutex.RUnlock()

	// Remove the distant chunks
	if len(chunksToRemove) > 0 {
		cm.chunksMutex.Lock()
		for _, coord := range chunksToRemove {
			delete(cm.chunks, coord)
		}
		cm.chunksMutex.Unlock()

		// Mark that chunks have changed
		cm.markChunksChanged()
	}
}
