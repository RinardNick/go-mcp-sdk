package stdio

import (
	"io"

	"github.com/RinardNick/go-mcp-sdk/pkg/client"
)

// NewSession creates a new stdio session
func NewSession(reader io.Reader, writer io.Writer, c *StdioClient) (*client.Session, error) {
	return client.NewSession(reader, writer, c)
}
