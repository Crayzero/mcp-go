package client

import (
	"git.woa.com/copilot-chat/copilot_agent/mcp-go/client/transport"
	"git.woa.com/copilot-chat/copilot_agent/mcp-go/server"
)

// NewInProcessClient connect directly to a mcp server object in the same process
func NewInProcessClient(server *server.MCPServer) (*Client, error) {
	inProcessTransport := transport.NewInProcessTransport(server)
	return NewClient(inProcessTransport), nil
}
