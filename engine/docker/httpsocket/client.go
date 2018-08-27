package httpsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
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

func (c *Client) Post(path string, data interface{}, out interface{}) error {
	bData, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "json marshal")
	}
	res, err := c.client.Post("http://unix"+path, "application/json", strings.NewReader(string(bData)))
	if err != nil {
		return errors.Wrap(err, "http post")
	}
	defer res.Body.Close()

	if res.StatusCode == 204 {
		// FIXME how to actually clear it
		out = nil
		return nil
	}

	if res.StatusCode >= 500 {
		txt, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return errors.Wrap(err, "read http body")
		}
		var message struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(txt, &message); err == nil && message.Message != "" {
			return errors.New(message.Message)
		}
		return fmt.Errorf("HTTP(%d) %s", res.StatusCode, txt)
	}

	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) Delete(path string, out interface{}) error {
	statusCode, body, err := c.Do("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer body.Close()
	if statusCode == 204 {
		// FIXME how to actually clear it
		out = nil
		return nil
	}

	// txt, err := ioutil.ReadAll(body)
	// if err != nil {
	// 	return errors.Wrap(err, "read http body")
	// }
	// fmt.Println("BODY", err, string(txt))
	// body = ioutil.NopCloser(bytes.NewReader(txt))

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
