package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

func main() {
	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}

	// inspectImage(httpc, "dgodd/testlatestpackv3")
	inspectImage(httpc, "dgodd/packsdevv3:detect")

	// createContainer(httpc)
	// listContainers(httpc)
}

func inspectImage(httpc http.Client, name string) {
	res, err := httpc.Get("http://unix/images/" + name + "/json")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	// io.Copy(os.Stdout, res.Body)

	info := struct {
		Id     string
		Config struct {
			Env        []string
			WorkingDir string
		}
	}{}
	json.NewDecoder(res.Body).Decode(&info)

	fmt.Printf("%#v", info)
}

func createContainer(httpc http.Client) {
	data, err := json.Marshal(map[string]interface{}{
		"Image": "dgodd/testlatestpackv3",
	})
	res, err := httpc.Post("http://unix/containers/create", "application/json", strings.NewReader(string(data)))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	obj := struct {
		Id       string
		Warnings string
	}{}
	json.NewDecoder(res.Body).Decode(&obj)
	fmt.Printf("%#v", obj)

	fmt.Println("Post http://unix/containers/" + obj.Id + "/start")
	res, err = httpc.Post("http://unix/containers/"+obj.Id+"/start", "application/octet-stream", strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	io.Copy(os.Stdout, res.Body)
}

func listContainers(httpc http.Client) {
	res, err := httpc.Get("http://unix/containers/json")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	io.Copy(os.Stdout, res.Body)
}

func listImages(httpc http.Client) {
	res, err := httpc.Get("http://unix/images/json")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	images := make([]struct {
		Containers  int
		Created     int
		Id          string
		Labels      []string
		ParentId    string
		RepoDigests []string
		RepoTags    []string
		SharedSize  int
		Size        int
		VirtualSize int
	}, 0)
	json.NewDecoder(res.Body).Decode(&images)

	fmt.Printf("%#v", images)
}
