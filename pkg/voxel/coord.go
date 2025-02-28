package voxel

import (
	"github.com/go-gl/mathgl/mgl32"
)

// ChunkCoord represents the x,y,z coordinates of a chunk
type ChunkCoord struct {
	X, Y, Z int32
}

// WorldToChunkCoord converts a world position to chunk coordinates
func WorldToChunkCoord(worldX, worldY, worldZ int32, chunkSize int) ChunkCoord {
	// Integer division to get chunk coordinate
	return ChunkCoord{
		X: int32(worldX) / int32(chunkSize),
		Y: int32(worldY) / int32(chunkSize),
		Z: int32(worldZ) / int32(chunkSize),
	}
}

// WorldToLocalCoord converts a world position to local coordinates within a chunk
func WorldToLocalCoord(worldX, worldY, worldZ int32, chunkSize int) (int, int, int) {
	// Get the remainder to find position within chunk
	localX := int(worldX) % chunkSize
	localY := int(worldY) % chunkSize
	localZ := int(worldZ) % chunkSize

	// Handle negative coordinates properly
	if localX < 0 {
		localX += chunkSize
	}
	if localY < 0 {
		localY += chunkSize
	}
	if localZ < 0 {
		localZ += chunkSize
	}

	return localX, localY, localZ
}

// ChunkToWorldPos converts chunk coordinates to world position (corner of chunk)
func ChunkToWorldPos(chunkX, chunkY, chunkZ int32, chunkSize int) mgl32.Vec3 {
	return mgl32.Vec3{
		float32(chunkX * int32(chunkSize)),
		float32(chunkY * int32(chunkSize)),
		float32(chunkZ * int32(chunkSize)),
	}
}

// LocalToIndex converts local block coordinates to an index in a flat array
func LocalToIndex(x, y, z, chunkSize int) int {
	return x*chunkSize*chunkSize + y*chunkSize + z
}

// IndexToLocal converts a flat array index to local coordinates within a chunk
func IndexToLocal(index, chunkSize int) (x, y, z int) {
	x = index / (chunkSize * chunkSize)
	remainder := index % (chunkSize * chunkSize)
	y = remainder / chunkSize
	z = remainder % chunkSize
	return
}

// ConvertTo3DArray converts a flat 1D array of BlockType to a 3D array
// swapCoords: if true, swaps X and Z coordinates to fix coordinate system mismatches
func ConvertTo3DArray(flatBlocks []BlockType, sizeX, sizeY, sizeZ int, swapCoords bool) [][][]BlockType {
	// Initialize the 3D array based on dimensions
	blocks := make([][][]BlockType, sizeX)
	for i := range sizeX {
		blocks[i] = make([][]BlockType, sizeY)
		for j := range sizeY {
			blocks[i][j] = make([]BlockType, sizeZ)
		}
	}

	// Fill the array with data from the flat array
	for x := range sizeX {
		for y := range sizeY {
			for z := range sizeZ {
				// Calculate index in flat array
				index := x*sizeY*sizeZ + y*sizeZ + z

				// Determine target coordinates based on swap flag
				var tx, ty, tz int
				if swapCoords {
					// Swap X and Z coordinates
					tx, ty, tz = z, y, x
				} else {
					tx, ty, tz = x, y, z
				}

				// Set the block in the 3D array, with bounds checking
				if index < len(flatBlocks) {
					blocks[tx][ty][tz] = flatBlocks[index]
				} else {
					blocks[tx][ty][tz] = Air
				}
			}
		}
	}
	return blocks
}
