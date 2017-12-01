package app

import (
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"

	"github.com/sclevine/forge"
)

type Config struct {
	Path string
}

func (c *Config) Load() (*LocalYML, error) {
	localYML := &LocalYML{}
	yamlBytes, err := ioutil.ReadFile(c.Path)
	if pathErr, ok := err.(*os.PathError); ok && pathErr.Op == "open" {
		return localYML, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(yamlBytes, localYML); err != nil {
		return nil, err
	}
	return localYML, nil
}

func (c *Config) Save(localYML *LocalYML) error {
	yamlBytes, err := yaml.Marshal(localYML)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(c.Path, yamlBytes, 0666)
}

type LocalYML struct {
	Applications []*forge.AppConfig `yaml:"applications,omitempty"`
}
