package v2

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

func (y *AppYAML) Load(path string) error {
	yamlBytes, err := ioutil.ReadFile(path)
	if pathErr, ok := err.(*os.PathError); ok && pathErr.Op == "open" {
		return nil
	}
	if err != nil {
		return err
	}
	return yaml.Unmarshal(yamlBytes, y)
}

func (y *AppYAML) Save(path string) error {
	yamlBytes, err := yaml.Marshal(y)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, yamlBytes, 0666)
}

type AppYAML struct {
	Applications []*AppConfig `yaml:"applications,omitempty"`
}
