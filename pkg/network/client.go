package network

import (
	"fmt"
	"net"
	"time"
)

// Client handles network communication with the game server
type Client struct {
	conn       net.Conn
	serverAddr string
	connected  bool
}

// NewClient creates a new network client and connects to the specified server
func NewClient(serverAddr string) (*Client, error) {
	client := &Client{
		serverAddr: serverAddr,
	}

	err := client.Connect()
	if err != nil {
		return client, err
	}

	return client, nil
}

// Connect establishes a connection to the server
func (c *Client) Connect() error {
	conn, err := net.DialTimeout("tcp", c.serverAddr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn
	c.connected = true
	fmt.Printf("Connected to server at %s\n", c.serverAddr)

	// Start listening for messages in a goroutine
	go c.listenForMessages()

	return nil
}

// Disconnect closes the connection to the server
func (c *Client) Disconnect() {
	if c.connected && c.conn != nil {
		c.conn.Close()
		c.connected = false
		fmt.Println("Disconnected from server")
	}
}

// SendMessage sends a message to the server
func (c *Client) SendMessage(message []byte) error {
	if !c.connected || c.conn == nil {
		return fmt.Errorf("not connected to server")
	}

	_, err := c.conn.Write(message)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// listenForMessages continuously listens for messages from the server
func (c *Client) listenForMessages() {
	buffer := make([]byte, 1024)

	for c.connected && c.conn != nil {
		n, err := c.conn.Read(buffer)
		if err != nil {
			fmt.Printf("Error reading from server: %v\n", err)
			c.connected = false
			break
		}

		if n > 0 {
			// Process the message
			c.handleMessage(buffer[:n])
		}
	}
}

// handleMessage processes a message received from the server
func (c *Client) handleMessage(message []byte) {
	// TODO: Implement message handling logic
	fmt.Printf("Received message from server: %s\n", string(message))
}

// IsConnected returns whether the client is connected to the server
func (c *Client) IsConnected() bool {
	return c.connected
}

// GetServerAddr returns the server address
func (c *Client) GetServerAddr() string {
	return c.serverAddr
}
