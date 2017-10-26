package internal

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"text/template"
)

type Version struct {
	Client *http.Client
}

func (u *Version) Build(tmpl, versionURL string) (string, error) {
	resp, err := u.Client.Get(versionURL)
	if err != nil {
		return "", ErrNetwork
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", ErrUnavailable
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	urlBuf := &bytes.Buffer{}
	t := template.Must(template.New("").Parse(tmpl))
	if err := t.Execute(urlBuf, string(bytes.TrimSpace(body))); err != nil {
		return "", err
	}
	return urlBuf.String(), nil
}
