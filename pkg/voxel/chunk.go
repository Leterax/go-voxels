package voxel

import (
	"github.com/go-gl/mathgl/mgl32"
)

// BlockType represents the different types of blocks in the game
type BlockType uint8

const (
	Air BlockType = iota
	Grass
	Dirt
	Stone
	OakLog
	OakLeaves
	Glass
	Water
	Sand
	Snow
	OakPlanks
	StoneBricks
	Netherrack
	GoldBlock
	PackedIce
	Lava
	Barrel
	Bookshelf
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
	return x*c.Size*c.Size + y*c.Size + z
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
	return mgl32.Vec3{
		float32(c.X),
		float32(c.Y),
		float32(c.Z),
	}
}

// GenerateMesh creates a mesh for this chunk using greedy meshing
func (c *Chunk) GenerateMesh() *Mesh {
	// Convert to our expected 3D format
	blocks3D := convertTo3DArray(c.Blocks, c.Size)

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

// convertTo3DArray converts a flat 1D array of BlockType to a 3D array
// This function swaps X and Z coordinates to fix the coordinate system mismatch with the server
func convertTo3DArray(flatBlocks []BlockType, size int) [][][]BlockType {
	// We need to initialize the array with dimensions that match how we'll access it
	// Since we're swapping X and Z, we need to initialize as [size][size][size]
	blocks := make([][][]BlockType, size)
	for i := 0; i < size; i++ {
		blocks[i] = make([][]BlockType, size)
		for j := 0; j < size; j++ {
			blocks[i][j] = make([]BlockType, size)
		}
	}

	// Now we fill the array with swapped coordinates
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			for z := 0; z < size; z++ {
				// Use the original index calculation to access the flat array
				index := x*size*size + y*size + z

				// Here we swap X and Z when storing in the 3D array
				// Original coordinates (x,y,z) become (z,y,x) in the 3D array
				if index < len(flatBlocks) {
					// Put the voxel at the swapped coordinates
					blocks[z][y][x] = flatBlocks[index]
				} else {
					blocks[z][y][x] = Air
				}
			}
		}
	}
	return blocks
}
