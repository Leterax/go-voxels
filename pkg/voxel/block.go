package voxel

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

// BlockProperties contains physical properties of a block
type BlockProperties struct {
	Solid       bool
	Transparent bool
}

// Default block properties
var blockProperties = map[BlockType]BlockProperties{
	Air:         {Solid: false, Transparent: true},
	Glass:       {Solid: true, Transparent: true},
	Water:       {Solid: true, Transparent: true},
	Grass:       {Solid: true, Transparent: false},
	Dirt:        {Solid: true, Transparent: false},
	Stone:       {Solid: true, Transparent: false},
	OakLog:      {Solid: true, Transparent: false},
	OakLeaves:   {Solid: true, Transparent: true},
	Sand:        {Solid: true, Transparent: false},
	Snow:        {Solid: true, Transparent: false},
	OakPlanks:   {Solid: true, Transparent: false},
	StoneBricks: {Solid: true, Transparent: false},
	Netherrack:  {Solid: true, Transparent: false},
	GoldBlock:   {Solid: true, Transparent: false},
	PackedIce:   {Solid: true, Transparent: false},
	Lava:        {Solid: true, Transparent: true},
	Barrel:      {Solid: true, Transparent: false},
	Bookshelf:   {Solid: true, Transparent: false},
}

// GetBlockProperties returns properties for a specific block type
func GetBlockProperties(blockType BlockType) BlockProperties {
	props, exists := blockProperties[blockType]
	if !exists {
		// Return default properties if not found
		return BlockProperties{Solid: true, Transparent: false}
	}
	return props
}

// IsSolid returns whether the block type is solid
func (b BlockType) IsSolid() bool {
	return GetBlockProperties(b).Solid
}

// IsTransparent returns whether the block type is transparent
func (b BlockType) IsTransparent() bool {
	return GetBlockProperties(b).Transparent
}
