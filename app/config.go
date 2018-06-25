package app

import (
	"io/ioutil"
	"os"

	yaml "gopkg.in/yaml.v2"

	"github.com/buildpack/forge"
)

type Config struct {
	Path string
}

func (c *Config) Load() (*YAML, error) {
	localYML := &YAML{}
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

func (c *Config) Save(yml *YAML) error {
	yamlBytes, err := yaml.Marshal(yml)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(c.Path, yamlBytes, 0666)
}

type YAML struct {
	Applications []*forge.AppConfig `yaml:"applications,omitempty"`
}
