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

// GetBlock returns the block type at the specified local coordinates
func (c *Chunk) GetBlock(x, y, z int) BlockType {
	if x < 0 || y < 0 || z < 0 || x >= c.Size || y >= c.Size || z >= c.Size {
		return Air // Return air for out-of-bounds coordinates
	}
	index := x*c.Size*c.Size + y*c.Size + z
	return c.Blocks[index]
}

// SetBlock sets the block type at the specified local coordinates
func (c *Chunk) SetBlock(x, y, z int, blockType BlockType) {
	if x < 0 || y < 0 || z < 0 || x >= c.Size || y >= c.Size || z >= c.Size {
		return // Ignore out-of-bounds coordinates
	}
	index := x*c.Size*c.Size + y*c.Size + z
	c.Blocks[index] = blockType
}

// WorldPosition returns the world position of this chunk (corner)
func (c *Chunk) WorldPosition() mgl32.Vec3 {
	return mgl32.Vec3{
		float32(c.X * int32(c.Size)),
		float32(c.Y * int32(c.Size)),
		float32(c.Z * int32(c.Size)),
	}
}

// GenerateMesh creates a mesh for this chunk using greedy meshing
func (c *Chunk) GenerateMesh() *Mesh {
	// Convert to our expected 3D format
	blocks3D := convertNetworkBlocksTo3DArray(c.Blocks, c.Size, c.Size, c.Size)

	// Create a mesh using greedy meshing
	c.Mesh = GreedyMeshChunk(blocks3D, c.WorldPosition())
	return c.Mesh
}

// GeneratePackedMesh creates a mesh with packed vertices for this chunk
func (c *Chunk) GeneratePackedMesh() *Mesh {
	// Convert to our expected 3D format
	blocks3D := convertNetworkBlocksTo3DArray(c.Blocks, c.Size, c.Size, c.Size)

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

// convertNetworkBlocksTo3DArray converts a flat 1D array of network.BlockType to a 3D array of BlockType
func convertNetworkBlocksTo3DArray(flatBlocks []BlockType, sizeX, sizeY, sizeZ int) [][][]BlockType {
	blocks := make([][][]BlockType, sizeX)
	for x := 0; x < sizeX; x++ {
		blocks[x] = make([][]BlockType, sizeY)
		for y := 0; y < sizeY; y++ {
			blocks[x][y] = make([]BlockType, sizeZ)
			for z := 0; z < sizeZ; z++ {
				index := x*sizeY*sizeZ + y*sizeZ + z
				if index < len(flatBlocks) {
					blocks[x][y][z] = BlockType(flatBlocks[index])
				} else {
					blocks[x][y][z] = Air
				}
			}
		}
	}
	return blocks
}
