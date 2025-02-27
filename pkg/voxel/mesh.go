package voxel

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Direction represents a cardinal direction
type Direction int

const (
	North Direction = iota // -Z  (now -X after swap)
	South                  // +Z  (now +X after swap)
	East                   // +X  (now +Z after swap)
	West                   // -X  (now -Z after swap)
	Up                     // +Y  (unchanged)
	Down                   // -Y  (unchanged)
)

// DirectionVector returns the unit vector for a direction
// Updated to account for swapped X and Z coordinates
func (d Direction) DirectionVector() mgl32.Vec3 {
	switch d {
	case North: // Originally -Z, now -X
		return mgl32.Vec3{-1, 0, 0}
	case South: // Originally +Z, now +X
		return mgl32.Vec3{1, 0, 0}
	case East: // Originally +X, now +Z
		return mgl32.Vec3{0, 0, 1}
	case West: // Originally -X, now -Z
		return mgl32.Vec3{0, 0, -1}
	case Up: // +Y (unchanged)
		return mgl32.Vec3{0, 1, 0}
	case Down: // -Y (unchanged)
		return mgl32.Vec3{0, -1, 0}
	default:
		return mgl32.Vec3{0, 0, 0}
	}
}

// PackedVertex represents a vertex with all data packed into a single uint32
type PackedVertex struct {
	Packed uint32
}

// PackVertex packs vertex data into a single uint32
// x, y, z: Local vertex position (5 bits each, 0-31)
// u, v: Texture coordinates (1 bit each, 0 or 1)
// o: Orientation/face direction (3 bits, 0-7)
// t: Texture ID from block type (8 bits, 0-255)
// ao: Ambient occlusion (3 bits, 0-7)
// This function swaps X and Z coordinates to match the server's coordinate system
func PackVertex(x, y, z, u, v, o, t, ao int) uint32 {
	// 4 bytes, 32 bits
	// 00000000000000000000000000000000
	//  aaattttttttooouvzzzzzyyyyyxxxxx

	// Swap x and z coordinates to correct the orientation
	// This makes the coordinates match the server's coordinate system
	return uint32(
		((z & 31) << 0) | // Use z for the x position bits
			((y & 31) << 5) | // y remains the same
			((x & 31) << 10) | // Use x for the z position bits
			((u & 1) << 15) |
			((v & 1) << 16) |
			((o & 7) << 17) |
			((t & 255) << 20) |
			((ao & 7) << 28))
}

// Vertex represents a vertex in a mesh
type Vertex struct {
	Position  mgl32.Vec3
	Normal    mgl32.Vec3
	TexCoords mgl32.Vec2
}

// Face represents a face consisting of two triangles
type Face struct {
	Vertices  [4]Vertex // Counter-clockwise winding order
	BlockType BlockType
}

// Mesh represents a mesh of triangles
type Mesh struct {
	Faces    []Face
	Vertices []Vertex
	Indices  []uint32

	// Packed data for efficient rendering
	PackedVertices []uint32
}

// NewMesh creates a new empty mesh
func NewMesh() *Mesh {
	return &Mesh{
		Faces:          make([]Face, 0),
		Vertices:       make([]Vertex, 0),
		Indices:        make([]uint32, 0),
		PackedVertices: make([]uint32, 0),
	}
}

// AddFace adds a face to the mesh
func (m *Mesh) AddFace(face Face) {
	m.Faces = append(m.Faces, face)

	// Adding quad as two triangles
	baseIndex := uint32(len(m.Vertices))

	// Add four vertices
	for _, v := range face.Vertices {
		m.Vertices = append(m.Vertices, v)
	}

	// Add indices for two triangles (CCW winding)
	m.Indices = append(m.Indices, baseIndex, baseIndex+1, baseIndex+2)
	m.Indices = append(m.Indices, baseIndex, baseIndex+2, baseIndex+3)
}

// AddPackedFace adds a face with packed vertex data
func (m *Mesh) AddPackedFace(packedVertices [4]uint32) {
	// Add the four packed vertices for the quad
	for _, pv := range packedVertices {
		m.PackedVertices = append(m.PackedVertices, pv)
	}
}

// GreedyMeshChunk performs greedy meshing on a chunk of voxels
// It takes a 3D array of voxel types and generates an optimized mesh
func GreedyMeshChunk(voxels [][][]BlockType, chunkPos mgl32.Vec3) *Mesh {
	mesh := NewMesh()

	// Get dimensions
	sizeX := len(voxels)
	if sizeX == 0 {
		return mesh
	}
	sizeY := len(voxels[0])
	if sizeY == 0 {
		return mesh
	}
	sizeZ := len(voxels[0][0])
	if sizeZ == 0 {
		return mesh
	}

	// Process each axis direction separately
	for axis := 0; axis < 3; axis++ {
		// Define the dimensions and axes based on the current main axis
		var uAxis, vAxis int
		var maskSize [3]int

		if axis == 0 { // X-axis faces (YZ plane)
			uAxis, vAxis = 1, 2 // Y, Z
			maskSize = [3]int{sizeX, sizeY, sizeZ}
		} else if axis == 1 { // Y-axis faces (XZ plane)
			uAxis, vAxis = 0, 2 // X, Z
			maskSize = [3]int{sizeX, sizeY, sizeZ}
		} else { // Z-axis faces (XY plane)
			uAxis, vAxis = 0, 1 // X, Y
			maskSize = [3]int{sizeX, sizeY, sizeZ}
		}

		// For each face coordinate along axis d
		for x := 0; x <= maskSize[axis]; x++ {
			// Create masks for this slice
			maskPos := make([][]bool, maskSize[uAxis])
			maskNeg := make([][]bool, maskSize[uAxis])
			idsPos := make([][]BlockType, maskSize[uAxis])
			idsNeg := make([][]BlockType, maskSize[uAxis])

			for i := 0; i < maskSize[uAxis]; i++ {
				maskPos[i] = make([]bool, maskSize[vAxis])
				maskNeg[i] = make([]bool, maskSize[vAxis])
				idsPos[i] = make([]BlockType, maskSize[vAxis])
				idsNeg[i] = make([]BlockType, maskSize[vAxis])
			}

			// Fill the masks based on voxel visibility
			if 0 < x && x < maskSize[axis] {
				// Interior faces
				for u := 0; u < maskSize[uAxis]; u++ {
					for v := 0; v < maskSize[vAxis]; v++ {
						// Convert 2D mask coordinates to 3D voxel coordinates
						var pos, neg [3]int
						pos[axis] = x
						neg[axis] = x - 1
						pos[uAxis] = u
						neg[uAxis] = u
						pos[vAxis] = v
						neg[vAxis] = v

						// Get voxel types
						var posFilled, negFilled bool
						var posID, negID BlockType

						// Safely get the voxel types
						if pos[0] >= 0 && pos[0] < sizeX && pos[1] >= 0 && pos[1] < sizeY && pos[2] >= 0 && pos[2] < sizeZ {
							posID = voxels[pos[0]][pos[1]][pos[2]]
							posFilled = posID != Air
						}

						if neg[0] >= 0 && neg[0] < sizeX && neg[1] >= 0 && neg[1] < sizeY && neg[2] >= 0 && neg[2] < sizeZ {
							negID = voxels[neg[0]][neg[1]][neg[2]]
							negFilled = negID != Air
						}

						// Set masks and ids
						if negFilled && !posFilled {
							maskPos[u][v] = true
							idsPos[u][v] = negID
						}

						if posFilled && !negFilled {
							maskNeg[u][v] = true
							idsNeg[u][v] = posID
						}
					}
				}
			} else if x == 0 {
				// Negative boundary
				for u := 0; u < maskSize[uAxis]; u++ {
					for v := 0; v < maskSize[vAxis]; v++ {
						var pos [3]int
						pos[axis] = 0
						pos[uAxis] = u
						pos[vAxis] = v

						if pos[0] >= 0 && pos[0] < sizeX && pos[1] >= 0 && pos[1] < sizeY && pos[2] >= 0 && pos[2] < sizeZ {
							posID := voxels[pos[0]][pos[1]][pos[2]]
							if posID != Air {
								maskNeg[u][v] = true
								idsNeg[u][v] = posID
							}
						}
					}
				}
			} else if x == maskSize[axis] {
				// Positive boundary
				for u := 0; u < maskSize[uAxis]; u++ {
					for v := 0; v < maskSize[vAxis]; v++ {
						var neg [3]int
						neg[axis] = x - 1
						neg[uAxis] = u
						neg[vAxis] = v

						if neg[0] >= 0 && neg[0] < sizeX && neg[1] >= 0 && neg[1] < sizeY && neg[2] >= 0 && neg[2] < sizeZ {
							negID := voxels[neg[0]][neg[1]][neg[2]]
							if negID != Air {
								maskPos[u][v] = true
								idsPos[u][v] = negID
							}
						}
					}
				}
			}

			// Process both face directions
			for maskDir := 0; maskDir < 2; maskDir++ {
				var mask [][]bool
				var ids [][]BlockType
				var normalSign int

				if maskDir == 0 {
					mask = maskPos
					ids = idsPos
					normalSign = 1
				} else {
					mask = maskNeg
					ids = idsNeg
					normalSign = -1
				}

				// Check if there are any faces to process
				hasFaces := false
				for u := 0; u < maskSize[uAxis]; u++ {
					for v := 0; v < maskSize[vAxis]; v++ {
						if mask[u][v] {
							hasFaces = true
							break
						}
					}
					if hasFaces {
						break
					}
				}

				if !hasFaces {
					continue
				}

				// Track visited cells
				visited := make([][]bool, maskSize[uAxis])
				for i := range visited {
					visited[i] = make([]bool, maskSize[vAxis])
				}

				// Extract rectangles (similar to Python's extract_rectangles)
				for u := 0; u < maskSize[uAxis]; u++ {
					for v := 0; v < maskSize[vAxis]; v++ {
						if !mask[u][v] || visited[u][v] {
							continue
						}

						// Get the block type
						blockType := ids[u][v]

						// Find width (along v-axis)
						width := 1
						for width+v < maskSize[vAxis] && mask[u][v+width] && !visited[u][v+width] && ids[u][v+width] == blockType {
							width++
						}

						// Find height (along u-axis)
						height := 1
						canExtend := true

						for height+u < maskSize[uAxis] && canExtend {
							// Check if we can extend the entire row
							for i := 0; i < width; i++ {
								if !mask[u+height][v+i] || visited[u+height][v+i] || ids[u+height][v+i] != blockType {
									canExtend = false
									break
								}
							}

							if canExtend {
								height++
							}
						}

						// Mark as visited
						for i := 0; i < height; i++ {
							for j := 0; j < width; j++ {
								visited[u+i][v+j] = true
							}
						}

						// Create a face
						// Base position
						pos := [3]int{0, 0, 0}
						pos[axis] = x
						pos[uAxis] = u
						pos[vAxis] = v

						// Create vertices
						var v0, v1, v2, v3 [3]int
						v0[0] = pos[0]
						v0[1] = pos[1]
						v0[2] = pos[2]
						v1[0] = pos[0]
						v1[1] = pos[1]
						v1[2] = pos[2]
						v2[0] = pos[0]
						v2[1] = pos[1]
						v2[2] = pos[2]
						v3[0] = pos[0]
						v3[1] = pos[1]
						v3[2] = pos[2]

						// Adjust vertices based on direction and normal sign
						// Match the Python logic for correct winding order
						if axis == 1 { // Y-axis faces need special handling
							if normalSign > 0 { // Top face (normal points up)
								// Counter-clockwise when looking down from above
								v1[vAxis] += width  // Forward
								v2[uAxis] += height // Right + Forward
								v2[vAxis] += width
								v3[uAxis] += height // Right
							} else { // Bottom face (normal points down)
								// Counter-clockwise when looking up from below
								v1[uAxis] += height // Right
								v2[uAxis] += height // Right + Forward
								v2[vAxis] += width
								v3[vAxis] += width // Forward
							}
						} else { // X and Z axis faces
							if normalSign > 0 { // Normal points positive
								v1[uAxis] += height // Up
								v2[uAxis] += height // Up + Right
								v2[vAxis] += width
								v3[vAxis] += width // Right
							} else { // Normal points negative
								v1[vAxis] += width  // Right
								v2[uAxis] += height // Up + Right
								v2[vAxis] += width
								v3[uAxis] += height // Up
							}
						}

						// Calculate normal
						normal := [3]int{0, 0, 0}
						normal[axis] = normalSign

						// Convert to world positions
						worldPos := func(localPos [3]int) mgl32.Vec3 {
							return mgl32.Vec3{
								float32(localPos[0]) + chunkPos[0],
								float32(localPos[1]) + chunkPos[1],
								float32(localPos[2]) + chunkPos[2],
							}
						}

						// Convert to Vec3
						p0 := worldPos(v0)
						p1 := worldPos(v1)
						p2 := worldPos(v2)
						p3 := worldPos(v3)

						// Determine orientation (0-5 for the 6 cardinal directions)
						var orientation int
						if axis == 0 {
							if normalSign > 0 {
								orientation = int(East)
							} else {
								orientation = int(West)
							}
						} else if axis == 1 {
							if normalSign > 0 {
								orientation = int(Up)
							} else {
								orientation = int(Down)
							}
						} else { // axis == 2
							if normalSign > 0 {
								orientation = int(South)
							} else {
								orientation = int(North)
							}
						}

						// Get texture ID from block type (limit to 8 bits)
						textureID := int(blockType)
						if textureID > 255 {
							textureID = 255
						}

						// Default ambient occlusion
						ambientOcclusion := 7

						// Create local packed vertices for efficient rendering
						// We need to ensure they are within the 5-bit range (0-31)
						packedVertices := [4]uint32{
							PackVertex(v0[0]%32, v0[1]%32, v0[2]%32, 0, 0, orientation, textureID, ambientOcclusion),
							PackVertex(v1[0]%32, v1[1]%32, v1[2]%32, 1, 0, orientation, textureID, ambientOcclusion),
							PackVertex(v2[0]%32, v2[1]%32, v2[2]%32, 1, 1, orientation, textureID, ambientOcclusion),
							PackVertex(v3[0]%32, v3[1]%32, v3[2]%32, 0, 1, orientation, textureID, ambientOcclusion),
						}

						// Add packed vertices to mesh
						mesh.AddPackedFace(packedVertices)

						// Create normal vector for traditional face
						faceNormal := mgl32.Vec3{float32(normal[0]), float32(normal[1]), float32(normal[2])}

						// Calculate texture coordinates
						t0 := mgl32.Vec2{0, 0}
						t1 := mgl32.Vec2{float32(width), 0}
						t2 := mgl32.Vec2{float32(width), float32(height)}
						t3 := mgl32.Vec2{0, float32(height)}

						// Create the traditional face for compatibility
						face := Face{
							BlockType: blockType,
							Vertices: [4]Vertex{
								{Position: p0, Normal: faceNormal, TexCoords: t0},
								{Position: p1, Normal: faceNormal, TexCoords: t1},
								{Position: p2, Normal: faceNormal, TexCoords: t2},
								{Position: p3, Normal: faceNormal, TexCoords: t3},
							},
						}

						// Add face to mesh
						mesh.AddFace(face)
					}
				}
			}
		}
	}

	return mesh
}

// ConvertTo3DArray converts a flat 1D array of BlockTypes to a 3D array
// Assumes the flat array is arranged in X -> Y -> Z order
func ConvertTo3DArray(flatBlocks []BlockType, sizeX, sizeY, sizeZ int) [][][]BlockType {
	blocks := make([][][]BlockType, sizeX)
	for x := 0; x < sizeX; x++ {
		blocks[x] = make([][]BlockType, sizeY)
		for y := 0; y < sizeY; y++ {
			blocks[x][y] = make([]BlockType, sizeZ)
			for z := 0; z < sizeZ; z++ {
				index := x*sizeY*sizeZ + y*sizeZ + z
				if index < len(flatBlocks) {
					blocks[x][y][z] = flatBlocks[index]
				} else {
					blocks[x][y][z] = Air
				}
			}
		}
	}
	return blocks
}

// GreedyMesh processes a flat array of block types and returns a mesh
func GreedyMesh(flatBlocks []BlockType, chunkX, chunkY, chunkZ int32, chunkSize int) *Mesh {
	// Convert chunk position to world coordinates
	chunkPos := mgl32.Vec3{float32(chunkX), float32(chunkY), float32(chunkZ)}

	// Convert flat array to 3D array
	blocks := ConvertTo3DArray(flatBlocks, chunkSize, chunkSize, chunkSize)

	// Generate mesh using greedy meshing
	return GreedyMeshChunk(blocks, chunkPos)
}

// MonoChunkMesh generates a mesh for a chunk filled with a single block type
// This is an optimization for chunks that contain only one type of block
func MonoChunkMesh(chunk *Chunk, blockType BlockType) *Mesh {
	// Create a new mesh
	mesh := NewMesh()
	size := chunk.Size

	// Define the face orientations - one quad per side of the chunk
	orientations := []struct {
		vertices [4]uint32
	}{
		{ // +X face (right)
			vertices: [4]uint32{
				PackVertex(size, 0, 0, 0, 0, 0, int(blockType), 7),
				PackVertex(size, size, 0, 0, 1, 0, int(blockType), 7),
				PackVertex(size, size, size, 1, 1, 0, int(blockType), 7),
				PackVertex(size, 0, size, 1, 0, 0, int(blockType), 7),
			},
		},
		{ // -X face (left)
			vertices: [4]uint32{
				PackVertex(0, 0, size, 0, 0, 1, int(blockType), 7),
				PackVertex(0, size, size, 0, 1, 1, int(blockType), 7),
				PackVertex(0, size, 0, 1, 1, 1, int(blockType), 7),
				PackVertex(0, 0, 0, 1, 0, 1, int(blockType), 7),
			},
		},
		{ // +Y face (top)
			vertices: [4]uint32{
				PackVertex(0, size, 0, 0, 0, 2, int(blockType), 7),
				PackVertex(0, size, size, 0, 1, 2, int(blockType), 7),
				PackVertex(size, size, size, 1, 1, 2, int(blockType), 7),
				PackVertex(size, size, 0, 1, 0, 2, int(blockType), 7),
			},
		},
		{ // -Y face (bottom)
			vertices: [4]uint32{
				PackVertex(0, 0, size, 0, 0, 3, int(blockType), 7),
				PackVertex(0, 0, 0, 0, 1, 3, int(blockType), 7),
				PackVertex(size, 0, 0, 1, 1, 3, int(blockType), 7),
				PackVertex(size, 0, size, 1, 0, 3, int(blockType), 7),
			},
		},
		{ // +Z face (front)
			vertices: [4]uint32{
				PackVertex(0, 0, size, 0, 0, 4, int(blockType), 7),
				PackVertex(size, 0, size, 0, 1, 4, int(blockType), 7),
				PackVertex(size, size, size, 1, 1, 4, int(blockType), 7),
				PackVertex(0, size, size, 1, 0, 4, int(blockType), 7),
			},
		},
		{ // -Z face (back)
			vertices: [4]uint32{
				PackVertex(0, size, 0, 0, 0, 5, int(blockType), 7),
				PackVertex(size, size, 0, 0, 1, 5, int(blockType), 7),
				PackVertex(size, 0, 0, 1, 1, 5, int(blockType), 7),
				PackVertex(0, 0, 0, 1, 0, 5, int(blockType), 7),
			},
		},
	}

	// Add all six faces to the mesh
	for _, orientation := range orientations {
		mesh.AddPackedFace(orientation.vertices)
	}

	return mesh
}
