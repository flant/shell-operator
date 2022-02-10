package debug

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/flant/shell-operator/pkg/app"
	utils "github.com/flant/shell-operator/pkg/utils/file"
)

type Client struct {
	SocketPath string
}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) WithSocketPath(path string) {
	c.SocketPath = path
}

func (c *Client) newHttpClient() (http.Client, error) {
	exists, err := utils.FileExists(c.SocketPath)
	if err != nil {
		return http.Client{}, fmt.Errorf("check debug socket '%s': %s", c.SocketPath, err)
	}
	if !exists {
		return http.Client{}, fmt.Errorf("debug socket '%s' is not exists", c.SocketPath)
	}

	return http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", c.SocketPath)
			},
		},
	}, nil
}

func DefaultClient() *Client {
	cl := NewClient()
	cl.WithSocketPath(app.DebugUnixSocket)
	return cl
}

func (c *Client) Get(url string) ([]byte, error) {
	httpc, err := c.newHttpClient()
	if err != nil {
		return nil, err
	}

	resp, err := httpc.Get(url)
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
	httpc, err := c.newHttpClient()
	if err != nil {
		return nil, err
	}

	resp, err := httpc.PostForm(targetUrl, data)
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
