package voxel

import (
	"github.com/go-gl/mathgl/mgl32"
)

// Direction represents a cardinal direction
type Direction int

const (
	North Direction = iota // -Z
	South                  // +Z
	East                   // +X
	West                   // -X
	Up                     // +Y
	Down                   // -Y
)

// DirectionVector returns the unit vector for a direction
func (d Direction) DirectionVector() mgl32.Vec3 {
	switch d {
	case North:
		return mgl32.Vec3{0, 0, -1}
	case South:
		return mgl32.Vec3{0, 0, 1}
	case East:
		return mgl32.Vec3{1, 0, 0}
	case West:
		return mgl32.Vec3{-1, 0, 0}
	case Up:
		return mgl32.Vec3{0, 1, 0}
	case Down:
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
func PackVertex(x, y, z, u, v, o, t, ao int) uint32 {
	// 4 bytes, 32 bits
	// 00000000000000000000000000000000
	//  aaattttttttooouvzzzzzyyyyyxxxxx
	return uint32(
		((x & 31) << 0) |
			((y & 31) << 5) |
			((z & 31) << 10) |
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

	// Create a mask for visited voxels during face merging
	visited := make([][][]bool, sizeX)
	for x := 0; x < sizeX; x++ {
		visited[x] = make([][]bool, sizeY)
		for y := 0; y < sizeY; y++ {
			visited[x][y] = make([]bool, sizeZ)
		}
	}

	// Process each direction separately
	for dim := 0; dim < 6; dim++ {
		dir := Direction(dim)

		// Reset the visited mask for this direction
		for x := 0; x < sizeX; x++ {
			for y := 0; y < sizeY; y++ {
				for z := 0; z < sizeZ; z++ {
					visited[x][y][z] = false
				}
			}
		}

		// Determine the axis based on direction
		var u, v, w int
		var maskSize [3]int

		switch dir {
		case North, South: // Z axis
			u, v, w = 0, 1, 2
			maskSize = [3]int{sizeX, sizeY, sizeZ}
		case East, West: // X axis
			u, v, w = 2, 1, 0
			maskSize = [3]int{sizeZ, sizeY, sizeX}
		case Up, Down: // Y axis
			u, v, w = 0, 2, 1
			maskSize = [3]int{sizeX, sizeZ, sizeY}
		}

		// Determine the range for the main axis
		wStart, wEnd, wStep := 0, maskSize[w], 1
		if dir == South || dir == East || dir == Up {
			wStart, wEnd = maskSize[w]-1, -1
			wStep = -1
		}

		// Iterate through the volume along main axis
		for w0 := wStart; w0 != wEnd; w0 += wStep {
			// Create a 2D array for the mask
			mask := make([][]BlockType, maskSize[u])
			for i := 0; i < maskSize[u]; i++ {
				mask[i] = make([]BlockType, maskSize[v])
				for j := 0; j < maskSize[v]; j++ {
					// Initialize with air
					mask[i][j] = Air
				}
			}

			// For each slice, create a mask of visible faces
			for v0 := 0; v0 < maskSize[v]; v0++ {
				for u0 := 0; u0 < maskSize[u]; u0++ {
					// Get voxel coordinates based on direction
					var x, y, z int

					switch dir {
					case North, South:
						x, y, z = u0, v0, w0
					case East, West:
						x, y, z = w0, v0, u0
					case Up, Down:
						x, y, z = u0, w0, v0
					}

					// Skip if already visited
					if visited[x][y][z] {
						continue
					}

					// Check face visibility: if this block isn't air and either the adjacent block is air
					// or it's the edge of the chunk
					blockType := voxels[x][y][z]
					if blockType == Air {
						continue // Skip air blocks
					}

					// Get adjacent block coordinates
					nx, ny, nz := x, y, z
					switch dir {
					case North:
						nz--
					case South:
						nz++
					case East:
						nx++
					case West:
						nx--
					case Up:
						ny++
					case Down:
						ny--
					}

					// Check if adjacent block is outside the chunk or is air
					isVisible := false
					if nx < 0 || nx >= sizeX || ny < 0 || ny >= sizeY || nz < 0 || nz >= sizeZ {
						isVisible = true // Edge of chunk
					} else if voxels[nx][ny][nz] == Air {
						isVisible = true // Adjacent to air
					} else if voxels[nx][ny][nz] != blockType {
						isVisible = true // Adjacent to different block type
					}

					if isVisible {
						mask[u0][v0] = blockType
					}
				}
			}

			// Now perform greedy meshing on the 2D mask
			for v0 := 0; v0 < maskSize[v]; v0++ {
				for u0 := 0; u0 < maskSize[u]; u0++ {
					// Skip if already visited or air
					blockType := mask[u0][v0]
					if blockType == Air {
						continue
					}

					// Get voxel coordinates
					var x, y, z int
					switch dir {
					case North, South:
						x, y, z = u0, v0, w0
					case East, West:
						x, y, z = w0, v0, u0
					case Up, Down:
						x, y, z = u0, w0, v0
					}

					// Skip if already processed
					if visited[x][y][z] {
						continue
					}

					// Find the maximum width of the face (along u)
					width := 1
					for u1 := u0 + 1; u1 < maskSize[u]; u1++ {
						// Stop if the block type changes or we hit a visited block
						var nextX, nextY, nextZ int
						switch dir {
						case North, South:
							nextX, nextY, nextZ = u1, v0, w0
						case East, West:
							nextX, nextY, nextZ = w0, v0, u1
						case Up, Down:
							nextX, nextY, nextZ = u1, w0, v0
						}

						if mask[u1][v0] != blockType || visited[nextX][nextY][nextZ] {
							break
						}
						width++
					}

					// Find the maximum height of the face (along v)
					height := 1
					canExtend := true
					for v1 := v0 + 1; v1 < maskSize[v] && canExtend; v1++ {
						// Check if we can extend the entire row
						for u1 := u0; u1 < u0+width; u1++ {
							var nextX, nextY, nextZ int
							switch dir {
							case North, South:
								nextX, nextY, nextZ = u1, v1, w0
							case East, West:
								nextX, nextY, nextZ = w0, v1, u1
							case Up, Down:
								nextX, nextY, nextZ = u1, w0, v1
							}

							if mask[u1][v1] != blockType || visited[nextX][nextY][nextZ] {
								canExtend = false
								break
							}
						}

						if canExtend {
							height++
						}
					}

					// Mark all covered voxels as visited
					for v1 := v0; v1 < v0+height; v1++ {
						for u1 := u0; u1 < u0+width; u1++ {
							var visitX, visitY, visitZ int
							switch dir {
							case North, South:
								visitX, visitY, visitZ = u1, v1, w0
							case East, West:
								visitX, visitY, visitZ = w0, v1, u1
							case Up, Down:
								visitX, visitY, visitZ = u1, w0, v1
							}

							visited[visitX][visitY][visitZ] = true
						}
					}

					// Create a face using packed vertices
					packedVertices := [4]uint32{}

					// Determine orientation (0-5 for the 6 cardinal directions)
					orientation := int(dir)

					// Get texture ID from block type (limit to 8 bits)
					textureID := int(blockType)
					if textureID > 255 {
						textureID = 255
					}

					// Default ambient occlusion value (can be calculated properly in a more advanced implementation)
					ambientOcclusion := 7 // Max value for now

					// Calculate local positions within chunk (0-31 range)
					// These are calculated from the face corners, ensuring we stay within 5-bit range
					var x0, y0, z0, x1, y1, z1, x2, y2, z2, x3, y3, z3 int

					// Adjust positions based on direction to work within our 5-bit constraints
					switch dir {
					case North: // Facing -Z
						x0 = u0 % 32
						y0 = v0 % 32
						z0 = w0 % 32
						x1 = (u0 + width) % 32
						y1 = v0 % 32
						z1 = w0 % 32
						x2 = (u0 + width) % 32
						y2 = (v0 + height) % 32
						z2 = w0 % 32
						x3 = u0 % 32
						y3 = (v0 + height) % 32
						z3 = w0 % 32
					case South: // Facing +Z
						x0 = (u0 + width) % 32
						y0 = v0 % 32
						z0 = (w0 + 1) % 32
						x1 = u0 % 32
						y1 = v0 % 32
						z1 = (w0 + 1) % 32
						x2 = u0 % 32
						y2 = (v0 + height) % 32
						z2 = (w0 + 1) % 32
						x3 = (u0 + width) % 32
						y3 = (v0 + height) % 32
						z3 = (w0 + 1) % 32
					case East: // Facing +X
						x0 = (w0 + 1) % 32
						y0 = v0 % 32
						z0 = (u0 + width) % 32
						x1 = (w0 + 1) % 32
						y1 = v0 % 32
						z1 = u0 % 32
						x2 = (w0 + 1) % 32
						y2 = (v0 + height) % 32
						z2 = u0 % 32
						x3 = (w0 + 1) % 32
						y3 = (v0 + height) % 32
						z3 = (u0 + width) % 32
					case West: // Facing -X
						x0 = w0 % 32
						y0 = v0 % 32
						z0 = u0 % 32
						x1 = w0 % 32
						y1 = v0 % 32
						z1 = (u0 + width) % 32
						x2 = w0 % 32
						y2 = (v0 + height) % 32
						z2 = (u0 + width) % 32
						x3 = w0 % 32
						y3 = (v0 + height) % 32
						z3 = u0 % 32
					case Up: // Facing +Y
						x0 = u0 % 32
						y0 = (w0 + 1) % 32
						z0 = (v0 + height) % 32
						x1 = (u0 + width) % 32
						y1 = (w0 + 1) % 32
						z1 = (v0 + height) % 32
						x2 = (u0 + width) % 32
						y2 = (w0 + 1) % 32
						z2 = v0 % 32
						x3 = u0 % 32
						y3 = (w0 + 1) % 32
						z3 = v0 % 32
					case Down: // Facing -Y
						x0 = u0 % 32
						y0 = w0 % 32
						z0 = v0 % 32
						x1 = (u0 + width) % 32
						y1 = w0 % 32
						z1 = v0 % 32
						x2 = (u0 + width) % 32
						y2 = w0 % 32
						z2 = (v0 + height) % 32
						x3 = u0 % 32
						y3 = w0 % 32
						z3 = (v0 + height) % 32
					}

					// Pack the vertices using the UV coordinates
					packedVertices[0] = PackVertex(x0, y0, z0, 0, 0, orientation, textureID, ambientOcclusion)
					packedVertices[1] = PackVertex(x1, y1, z1, 1, 0, orientation, textureID, ambientOcclusion)
					packedVertices[2] = PackVertex(x2, y2, z2, 1, 1, orientation, textureID, ambientOcclusion)
					packedVertices[3] = PackVertex(x3, y3, z3, 0, 1, orientation, textureID, ambientOcclusion)

					// Add the packed face to the mesh
					mesh.AddPackedFace(packedVertices)

					// Also add the traditional face for compatibility
					// Generate the face (two triangles)
					face := Face{
						BlockType: blockType,
					}

					// Calculate the face coordinates in 3D space
					// Convert from voxel coordinates to world coordinates
					faceNormal := dir.DirectionVector()

					// World positions of the corners (add chunk position)
					var p0, p1, p2, p3 mgl32.Vec3

					// Adjust the vertex positions based on the direction
					switch dir {
					case North: // Facing -Z
						p0 = mgl32.Vec3{float32(u0), float32(v0), float32(w0)}
						p1 = mgl32.Vec3{float32(u0 + width), float32(v0), float32(w0)}
						p2 = mgl32.Vec3{float32(u0 + width), float32(v0 + height), float32(w0)}
						p3 = mgl32.Vec3{float32(u0), float32(v0 + height), float32(w0)}
					case South: // Facing +Z
						p0 = mgl32.Vec3{float32(u0 + width), float32(v0), float32(w0 + 1)}
						p1 = mgl32.Vec3{float32(u0), float32(v0), float32(w0 + 1)}
						p2 = mgl32.Vec3{float32(u0), float32(v0 + height), float32(w0 + 1)}
						p3 = mgl32.Vec3{float32(u0 + width), float32(v0 + height), float32(w0 + 1)}
					case East: // Facing +X
						p0 = mgl32.Vec3{float32(w0 + 1), float32(v0), float32(u0 + width)}
						p1 = mgl32.Vec3{float32(w0 + 1), float32(v0), float32(u0)}
						p2 = mgl32.Vec3{float32(w0 + 1), float32(v0 + height), float32(u0)}
						p3 = mgl32.Vec3{float32(w0 + 1), float32(v0 + height), float32(u0 + width)}
					case West: // Facing -X
						p0 = mgl32.Vec3{float32(w0), float32(v0), float32(u0)}
						p1 = mgl32.Vec3{float32(w0), float32(v0), float32(u0 + width)}
						p2 = mgl32.Vec3{float32(w0), float32(v0 + height), float32(u0 + width)}
						p3 = mgl32.Vec3{float32(w0), float32(v0 + height), float32(u0)}
					case Up: // Facing +Y
						p0 = mgl32.Vec3{float32(u0), float32(w0 + 1), float32(v0 + height)}
						p1 = mgl32.Vec3{float32(u0 + width), float32(w0 + 1), float32(v0 + height)}
						p2 = mgl32.Vec3{float32(u0 + width), float32(w0 + 1), float32(v0)}
						p3 = mgl32.Vec3{float32(u0), float32(w0 + 1), float32(v0)}
					case Down: // Facing -Y
						p0 = mgl32.Vec3{float32(u0), float32(w0), float32(v0)}
						p1 = mgl32.Vec3{float32(u0 + width), float32(w0), float32(v0)}
						p2 = mgl32.Vec3{float32(u0 + width), float32(w0), float32(v0 + height)}
						p3 = mgl32.Vec3{float32(u0), float32(w0), float32(v0 + height)}
					}

					// Add chunk position
					p0 = p0.Add(chunkPos)
					p1 = p1.Add(chunkPos)
					p2 = p2.Add(chunkPos)
					p3 = p3.Add(chunkPos)

					// Calculate texture coordinates based on block type and face direction
					// For simplicity, we're using a basic mapping for now
					// Texture mapping could be enhanced based on block type and atlas coordinates
					t0 := mgl32.Vec2{0, 0}
					t1 := mgl32.Vec2{float32(width), 0}
					t2 := mgl32.Vec2{float32(width), float32(height)}
					t3 := mgl32.Vec2{0, float32(height)}

					// Set up the vertices in counter-clockwise winding order
					face.Vertices[0] = Vertex{Position: p0, Normal: faceNormal, TexCoords: t0}
					face.Vertices[1] = Vertex{Position: p1, Normal: faceNormal, TexCoords: t1}
					face.Vertices[2] = Vertex{Position: p2, Normal: faceNormal, TexCoords: t2}
					face.Vertices[3] = Vertex{Position: p3, Normal: faceNormal, TexCoords: t3}

					// Add the face to the mesh
					mesh.AddFace(face)
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
	chunkPos := mgl32.Vec3{float32(chunkX * int32(chunkSize)), float32(chunkY * int32(chunkSize)), float32(chunkZ * int32(chunkSize))}

	// Convert flat array to 3D array
	blocks := ConvertTo3DArray(flatBlocks, chunkSize, chunkSize, chunkSize)

	// Generate mesh using greedy meshing
	return GreedyMeshChunk(blocks, chunkPos)
}
