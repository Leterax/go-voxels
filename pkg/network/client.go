package network

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strings"

	"github.com/leterax/go-voxels/pkg/voxel"
)

const (
	ServerPort = 20000
	ChunkSize  = 16
)

// ClientBound packet IDs
const (
	PacketIDIdentification       uint8 = 0x00
	PacketIDAddEntity            uint8 = 0x01
	PacketIDRemoveEntity         uint8 = 0x02
	PacketIDUpdateEntityPosition uint8 = 0x03
	PacketIDSendChunk            uint8 = 0x04
	PacketIDSendMonoTypeChunk    uint8 = 0x05
	PacketIDChat                 uint8 = 0x06
	PacketIDUpdateEntityMetadata uint8 = 0x07
)

// ServerBound packet IDs
const (
	PacketIDUpdateEntity   uint8 = 0x00
	PacketIDUpdateBlock    uint8 = 0x01
	PacketIDBlockBulkEdit  uint8 = 0x02
	PacketIDChatMessage    uint8 = 0x03
	PacketIDClientMetadata uint8 = 0x04
)

// Client represents a connection to the voxel game server
type Client struct {
	conn             net.Conn
	entityID         uint32
	entityName       string
	renderDist       uint8
	OnEntityAdd      func(entityID uint32, x, y, z, yaw, pitch float32, name string)
	OnEntityRemove   func(entityID uint32)
	OnEntityUpdate   func(entityID uint32, x, y, z, yaw, pitch float32)
	OnChunkReceive   func(x, y, z int32, blocks []voxel.BlockType)
	OnMonoChunk      func(x, y, z int32, blockType voxel.BlockType)
	OnChat           func(message string)
	OnEntityMetadata func(entityID uint32, name string)
}

// NewClient creates a new client connected to the server at the given address
func NewClient(address string) (*Client, error) {
	if !strings.Contains(address, ":") {
		address = fmt.Sprintf("%s:%d", address, ServerPort)
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn:       conn,
		renderDist: 8, // Default render distance
	}, nil
}

// Close closes the connection to the server
func (c *Client) Close() error {
	return c.conn.Close()
}

// SetEntityName sets the name of the client's entity
func (c *Client) SetEntityName(name string) {
	c.entityName = name
}

// SetRenderDistance sets the render distance for the client
func (c *Client) SetRenderDistance(distance uint8) {
	c.renderDist = distance
}

// SendClientMetadata sends the client metadata to the server
func (c *Client) SendClientMetadata() error {
	// Packet structure: id(U8) + renderDistance(U8) + name(U8[64])
	packet := make([]byte, 1+1+64)
	packet[0] = PacketIDClientMetadata
	packet[1] = c.renderDist

	// Copy name, truncating or padding with zeros as needed
	nameBytes := []byte(c.entityName)
	if len(nameBytes) > 64 {
		nameBytes = nameBytes[:64]
	}
	copy(packet[2:], nameBytes)

	_, err := c.conn.Write(packet)
	return err
}

// SendUpdateEntity sends the client's entity position to the server
func (c *Client) SendUpdateEntity(x, y, z, yaw, pitch float32) error {
	// Packet structure: id(U8) + x(F32) + y(F32) + z(F32) + yaw(F32) + pitch(F32)
	packet := make([]byte, 1+4*5)
	packet[0] = PacketIDUpdateEntity

	// Write floats in big endian
	binary.BigEndian.PutUint32(packet[1:], float32ToUint32(x))
	binary.BigEndian.PutUint32(packet[5:], float32ToUint32(y))
	binary.BigEndian.PutUint32(packet[9:], float32ToUint32(z))
	binary.BigEndian.PutUint32(packet[13:], float32ToUint32(yaw))
	binary.BigEndian.PutUint32(packet[17:], float32ToUint32(pitch))

	_, err := c.conn.Write(packet)
	return err
}

// SendUpdateBlock sends a block update to the server
func (c *Client) SendUpdateBlock(blockType voxel.BlockType, x, y, z int32) error {
	// Packet structure: id(U8) + blockType(U8) + x(I32) + y(I32) + z(I32)
	packet := make([]byte, 1+1+4*3)
	packet[0] = PacketIDUpdateBlock
	packet[1] = uint8(blockType)

	// Write coordinates in big endian
	binary.BigEndian.PutUint32(packet[2:], uint32(x))
	binary.BigEndian.PutUint32(packet[6:], uint32(y))
	binary.BigEndian.PutUint32(packet[10:], uint32(z))

	_, err := c.conn.Write(packet)
	return err
}

// SendBlockBulkEdit sends multiple block updates to the server
func (c *Client) SendBlockBulkEdit(updates []BlockUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	// Packet structure: id(U8) + blockCount(U32) + [blockType(U8) + x(I32) + y(I32) + z(I32)...]
	packetSize := 1 + 4 + (1+4*3)*len(updates)
	packet := make([]byte, packetSize)

	packet[0] = PacketIDBlockBulkEdit
	binary.BigEndian.PutUint32(packet[1:], uint32(len(updates)))

	offset := 5
	for _, update := range updates {
		packet[offset] = uint8(update.BlockType)
		binary.BigEndian.PutUint32(packet[offset+1:], uint32(update.X))
		binary.BigEndian.PutUint32(packet[offset+5:], uint32(update.Y))
		binary.BigEndian.PutUint32(packet[offset+9:], uint32(update.Z))
		offset += 13
	}

	_, err := c.conn.Write(packet)
	return err
}

// SendChat sends a chat message to the server
func (c *Client) SendChat(message string) error {
	// Packet structure: id(U8) + message(U8[4096])
	packet := make([]byte, 1+4096)
	packet[0] = PacketIDChatMessage

	// Copy message, truncating or padding with zeros as needed
	msgBytes := []byte(message)
	if len(msgBytes) > 4096 {
		msgBytes = msgBytes[:4096]
	}
	copy(packet[1:], msgBytes)

	_, err := c.conn.Write(packet)
	return err
}

// BlockUpdate represents a single block update
type BlockUpdate struct {
	BlockType voxel.BlockType
	X, Y, Z   int32
}

// ProcessPackets starts processing incoming packets from the server
func (c *Client) ProcessPackets() error {
	for {
		// Read packet ID
		var packetID uint8
		if err := binary.Read(c.conn, binary.BigEndian, &packetID); err != nil {
			if err == io.EOF {
				return fmt.Errorf("connection closed by server")
			}
			return fmt.Errorf("failed to read packet ID: %w", err)
		}

		// Process packet based on ID
		switch packetID {
		case PacketIDIdentification:
			if err := c.handleIdentification(); err != nil {
				return err
			}
		case PacketIDAddEntity:
			if err := c.handleAddEntity(); err != nil {
				return err
			}
		case PacketIDRemoveEntity:
			if err := c.handleRemoveEntity(); err != nil {
				return err
			}
		case PacketIDUpdateEntityPosition:
			if err := c.handleUpdateEntityPosition(); err != nil {
				return err
			}
		case PacketIDSendChunk:
			if err := c.handleSendChunk(); err != nil {
				return err
			}
		case PacketIDSendMonoTypeChunk:
			if err := c.handleSendMonoTypeChunk(); err != nil {
				return err
			}
		case PacketIDChat:
			if err := c.handleChat(); err != nil {
				return err
			}
		case PacketIDUpdateEntityMetadata:
			if err := c.handleUpdateEntityMetadata(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown packet ID: %d", packetID)
		}
	}
}

func (c *Client) handleIdentification() error {
	var entityID uint32
	if err := binary.Read(c.conn, binary.BigEndian, &entityID); err != nil {
		return fmt.Errorf("failed to read entity ID: %w", err)
	}

	c.entityID = entityID
	return nil
}

func (c *Client) handleAddEntity() error {
	var entityID uint32
	var x, y, z, yaw, pitch float32

	if err := binary.Read(c.conn, binary.BigEndian, &entityID); err != nil {
		return fmt.Errorf("failed to read entity ID: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &x); err != nil {
		return fmt.Errorf("failed to read x: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &y); err != nil {
		return fmt.Errorf("failed to read y: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &z); err != nil {
		return fmt.Errorf("failed to read z: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &yaw); err != nil {
		return fmt.Errorf("failed to read yaw: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &pitch); err != nil {
		return fmt.Errorf("failed to read pitch: %w", err)
	}

	// Read name (64 bytes)
	nameBytes := make([]byte, 64)
	if _, err := io.ReadFull(c.conn, nameBytes); err != nil {
		return fmt.Errorf("failed to read name: %w", err)
	}

	// Extract null-terminated name
	name := string(nameBytes)
	if idx := strings.IndexByte(name, 0); idx >= 0 {
		name = name[:idx]
	}

	if c.OnEntityAdd != nil {
		c.OnEntityAdd(entityID, x, y, z, yaw, pitch, name)
	}

	return nil
}

func (c *Client) handleRemoveEntity() error {
	var entityID uint32
	if err := binary.Read(c.conn, binary.BigEndian, &entityID); err != nil {
		return fmt.Errorf("failed to read entity ID: %w", err)
	}

	if c.OnEntityRemove != nil {
		c.OnEntityRemove(entityID)
	}

	return nil
}

func (c *Client) handleUpdateEntityPosition() error {
	var entityID uint32
	var x, y, z, yaw, pitch float32

	if err := binary.Read(c.conn, binary.BigEndian, &entityID); err != nil {
		return fmt.Errorf("failed to read entity ID: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &x); err != nil {
		return fmt.Errorf("failed to read x: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &y); err != nil {
		return fmt.Errorf("failed to read y: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &z); err != nil {
		return fmt.Errorf("failed to read z: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &yaw); err != nil {
		return fmt.Errorf("failed to read yaw: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &pitch); err != nil {
		return fmt.Errorf("failed to read pitch: %w", err)
	}

	if c.OnEntityUpdate != nil {
		c.OnEntityUpdate(entityID, x, y, z, yaw, pitch)
	}

	return nil
}

func (c *Client) handleSendChunk() error {
	var x, y, z int32

	if err := binary.Read(c.conn, binary.BigEndian, &x); err != nil {
		return fmt.Errorf("failed to read x: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &y); err != nil {
		return fmt.Errorf("failed to read y: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &z); err != nil {
		return fmt.Errorf("failed to read z: %w", err)
	}

	// Read chunk data (ChunkSize^3 bytes)
	chunkDataSize := ChunkSize * ChunkSize * ChunkSize
	chunkData := make([]byte, chunkDataSize)
	if _, err := io.ReadFull(c.conn, chunkData); err != nil {
		return fmt.Errorf("failed to read chunk data: %w", err)
	}

	// Convert to BlockType slice
	blocks := make([]voxel.BlockType, chunkDataSize)
	for i := range chunkDataSize {
		blocks[i] = voxel.BlockType(chunkData[i])
	}

	if c.OnChunkReceive != nil {
		c.OnChunkReceive(x, y, z, blocks)
	}

	return nil
}

func (c *Client) handleSendMonoTypeChunk() error {
	var x, y, z int32
	var blockType voxel.BlockType

	if err := binary.Read(c.conn, binary.BigEndian, &x); err != nil {
		return fmt.Errorf("failed to read x: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &y); err != nil {
		return fmt.Errorf("failed to read y: %w", err)
	}

	if err := binary.Read(c.conn, binary.BigEndian, &z); err != nil {
		return fmt.Errorf("failed to read z: %w", err)
	}

	// Read single block type
	var blockTypeByte uint8
	if err := binary.Read(c.conn, binary.BigEndian, &blockTypeByte); err != nil {
		return fmt.Errorf("failed to read block type: %w", err)
	}
	blockType = voxel.BlockType(blockTypeByte)

	if c.OnMonoChunk != nil {
		c.OnMonoChunk(x, y, z, blockType)
	}

	return nil
}

func (c *Client) handleChat() error {
	// Read message (4096 bytes)
	messageBytes := make([]byte, 4096)
	if _, err := io.ReadFull(c.conn, messageBytes); err != nil {
		return fmt.Errorf("failed to read message: %w", err)
	}

	// Extract null-terminated message
	message := string(messageBytes)
	if idx := strings.IndexByte(message, 0); idx >= 0 {
		message = message[:idx]
	}

	if c.OnChat != nil {
		c.OnChat(message)
	}

	return nil
}

func (c *Client) handleUpdateEntityMetadata() error {
	var entityID uint32

	if err := binary.Read(c.conn, binary.BigEndian, &entityID); err != nil {
		return fmt.Errorf("failed to read entity ID: %w", err)
	}

	// Read name (64 bytes)
	nameBytes := make([]byte, 64)
	if _, err := io.ReadFull(c.conn, nameBytes); err != nil {
		return fmt.Errorf("failed to read name: %w", err)
	}

	// Extract null-terminated name
	name := string(nameBytes)
	if idx := strings.IndexByte(name, 0); idx >= 0 {
		name = name[:idx]
	}

	if c.OnEntityMetadata != nil {
		c.OnEntityMetadata(entityID, name)
	}

	return nil
}

// Helper function to convert float32 to uint32 for binary encoding
func float32ToUint32(f float32) uint32 {
	bits := math.Float32bits(f)
	return bits
}
