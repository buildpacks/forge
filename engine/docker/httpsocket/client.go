package httpsocket

import (
	"context"
	"encoding/json"
	"io"
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
	if res.StatusCode == 204 {
		out = nil
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) Delete(path string, out interface{}) error {
	statusCode, body, err := c.Do("DELETE", "http://unix"+path, nil)
	if err != nil {
		return err
	}
	defer body.Close()
	if statusCode == 204 {
		out = nil
		return nil
	}
	return json.NewDecoder(body).Decode(out)
}

func (c *Client) Do(method, path string, body io.Reader) (int, io.ReadCloser, error) {
	req, err := http.NewRequest(method, "http://unix"+path, body)
	if err != nil {
		return 0, nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	return res.StatusCode, res.Body, nil
}
