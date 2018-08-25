package httpsocket

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type Client struct {
	client *http.Client
}

func New(socket string) *Client {
	return &Client{&http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
	}}
}

func (c *Client) Get(path string, out interface{}) error {
	res, err := c.client.Get("http://unix" + path)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) Post(path string, data interface{}, out interface{}) error {
	bData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	res, err := c.client.Post("http://unix"+path, "application/json", strings.NewReader(string(bData)))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(out)
}
