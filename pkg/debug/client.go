package debug

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/file"
)

type Client struct {
	SocketPath string
	httpClient *http.Client
}

func NewClient(socketPath string) (*Client, error) {
	exists, err := utils.FileExists(socketPath)
	if err != nil {
		return nil, fmt.Errorf("check debug socket '%s': %s", socketPath, err)
	}
	if !exists {
		return nil, fmt.Errorf("debug socket '%s' is not exists", socketPath)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := &net.Dialer{
					Timeout: 10 * time.Second,
				}
				return dialer.DialContext(ctx, "unix", socketPath)
			},
			DisableKeepAlives: true,
		},
	}

	return &Client{
		SocketPath: socketPath,
		httpClient: client,
	}, nil
}

func (c *Client) Close() {
	if c.httpClient != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}

func DefaultClient() (*Client, error) {
	return NewClient(app.DebugUnixSocket)
}

func (c *Client) Get(url string) ([]byte, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBuf := new(bytes.Buffer)
	_, err = io.Copy(bodyBuf, resp.Body)
	if err != nil {
		return nil, err
	}
	return bodyBuf.Bytes(), nil
}

func (c *Client) Post(targetUrl string, data map[string][]string) ([]byte, error) {
	resp, err := c.httpClient.PostForm(targetUrl, data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBuf := new(bytes.Buffer)
	_, err = io.Copy(bodyBuf, resp.Body)
	if err != nil {
		return nil, err
	}
	return bodyBuf.Bytes(), nil
}
