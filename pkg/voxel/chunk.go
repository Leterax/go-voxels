package voxel

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Chunk represents a 3D cube of voxels
type Chunk struct {
	// Position in chunk coordinates (not world coordinates)
	X, Y, Z int32
	// Size of the chunk in each dimension
	Size int
	// Voxel data
	Blocks []BlockType
	// Mesh of the chunk for rendering
	Mesh *Mesh
}

// NewChunk creates a new chunk at the specified coordinates
func NewChunk(x, y, z int32, size int) *Chunk {
	blockCount := size * size * size
	return &Chunk{
		X:      x,
		Y:      y,
		Z:      z,
		Size:   size,
		Blocks: make([]BlockType, blockCount),
	}
}

// NewChunkFromBlocks creates a new chunk from existing block data
func NewChunkFromBlocks(x, y, z int32, size int, blocks []BlockType) *Chunk {
	return &Chunk{
		X:      x,
		Y:      y,
		Z:      z,
		Size:   size,
		Blocks: blocks,
	}
}

// FillWithBlockType fills the entire chunk with a single block type
func (c *Chunk) FillWithBlockType(blockType BlockType) {
	for i := range c.Blocks {
		c.Blocks[i] = blockType
	}
}

// isValidCoordinate checks if the given coordinates are within the chunk boundaries
func (c *Chunk) isValidCoordinate(x, y, z int) bool {
	return x >= 0 && y >= 0 && z >= 0 && x < c.Size && y < c.Size && z < c.Size
}

// getBlockIndex converts 3D coordinates to a 1D array index
func (c *Chunk) getBlockIndex(x, y, z int) int {
	return LocalToIndex(x, y, z, c.Size)
}

// GetBlock returns the block type at the specified local coordinates
func (c *Chunk) GetBlock(x, y, z int) BlockType {
	if !c.isValidCoordinate(x, y, z) {
		return Air // Return air for out-of-bounds coordinates
	}
	return c.Blocks[c.getBlockIndex(x, y, z)]
}

// SetBlock sets the block type at the specified local coordinates
func (c *Chunk) SetBlock(x, y, z int, blockType BlockType) {
	if !c.isValidCoordinate(x, y, z) {
		return // Ignore out-of-bounds coordinates
	}
	c.Blocks[c.getBlockIndex(x, y, z)] = blockType
}

// WorldPosition returns the world position of this chunk (corner)
func (c *Chunk) WorldPosition() mgl32.Vec3 {
	return ChunkToWorldPos(c.X, c.Y, c.Z, c.Size)
}

// GenerateMesh creates a mesh for this chunk using greedy meshing
func (c *Chunk) GenerateMesh() *Mesh {
	// Convert to our expected 3D format, with coordinate swap
	blocks3D := ConvertTo3DArray(c.Blocks, c.Size, c.Size, c.Size, true)

	// Create a mesh using greedy meshing
	c.Mesh = GreedyMeshChunk(blocks3D, c.WorldPosition())
	return c.Mesh
}

// GetPackedVertexCount returns the number of packed vertices in the mesh
func (c *Chunk) GetPackedVertexCount() int {
	if c.Mesh == nil {
		return 0
	}
	return len(c.Mesh.PackedVertices)
}

// GetPackedVertices returns the packed vertices for rendering
func (c *Chunk) GetPackedVertices() []uint32 {
	if c.Mesh == nil {
		return nil
	}
	return c.Mesh.PackedVertices
}

// ForEachNeighbor calls the given function for each neighboring chunk position
func (c *Chunk) ForEachNeighbor(fn func(x, y, z int32)) {
	// Check the 26 neighboring chunks (all 3x3x3 grid around this chunk except this chunk itself)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for dz := -1; dz <= 1; dz++ {
				// Skip the center chunk
				if dx == 0 && dy == 0 && dz == 0 {
					continue
				}
				fn(c.X+int32(dx), c.Y+int32(dy), c.Z+int32(dz))
			}
		}
	}
}

// IsMono checks if the chunk contains only a single block type
// Returns true and the block type if mono, false and Air otherwise
func (c *Chunk) IsMono() (bool, BlockType) {
	if len(c.Blocks) == 0 {
		return true, Air
	}

	firstBlock := c.Blocks[0]

	// If first block is Air, quickly check if any block is not Air
	if firstBlock == Air {
		for _, block := range c.Blocks {
			if block != Air {
				return false, Air
			}
		}
		return true, Air
	}

	// Otherwise check if all blocks match the first one
	for _, block := range c.Blocks {
		if block != firstBlock {
			return false, Air
		}
	}

	return true, firstBlock
}
