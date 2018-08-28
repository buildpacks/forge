package httpsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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

func (c *Client) Post(path string, data interface{}, out interface{}, ctx context.Context) error {
	bData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "json marshal")
	}

	statusCode, body, _, err := c.Do("POST", path, strings.NewReader(string(bData)), ctx)
	if err != nil {
		return errors.Wrap(err, "http post")
	}
	defer body.Close()

	if statusCode == 204 {
		// FIXME how to actually clear it
		out = nil
		return nil
	}

	if statusCode >= 500 {
		txt, err := ioutil.ReadAll(body)
		if err != nil {
			return errors.Wrap(err, "read http body")
		}
		var message struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(txt, &message); err == nil && message.Message != "" {
			return errors.New(message.Message)
		}
		return fmt.Errorf("HTTP(%d) %s", statusCode, txt)
	}

	return json.NewDecoder(body).Decode(out)
}

func (c *Client) Delete(path string, out interface{}) error {
	statusCode, body, _, err := c.Do("DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	defer body.Close()
	if statusCode == 204 {
		// FIXME how to actually clear it
		out = nil
		return nil
	}

	return json.NewDecoder(body).Decode(out)
}

func (c *Client) Do(method, path string, body io.Reader, ctx context.Context) (int, io.ReadCloser, int64, error) {
	req, err := http.NewRequest(method, "http://unix"+path, body)
	if err != nil {
		return 0, nil, -1, err
	}
	req.Header.Set("Content-Type", "application/json")

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return 0, nil, -1, err
	}

	var contentLength int64 = -1
	if res.Header.Get("Content-Length") != "" {
		// fmt.Println("HEADER:", path, res.Header.Get("Content-Length"))
		if i, err := strconv.Atoi(res.Header.Get("Content-Length")); err == nil {
			contentLength = int64(i)
		}
	}

	return res.StatusCode, res.Body, contentLength, nil
}
