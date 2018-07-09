package config

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

// Config which would be read from the config.yaml
type Config struct {
	Controllers []Controller
}

// Controller which contains all the information for a specific controller
type Controller struct {
	Type           string
	WatchCriterion Criterion
	Actions        []Action
}

// Criterion as to what fields should the controller be looking at
type Criterion struct {
	Operator    string
	Identifiers []string
}

// Action that the controller will be taking based on the Parameters
type Action struct {
	Name   string
	Params map[interface{}]interface{}
}

// ReadConfig function that reads the yaml file
func ReadConfig(filePath string) (Config, error) {
	var config Config
	// Read YML
	source, err := ioutil.ReadFile(filePath)
	if err != nil {
		return config, err
	}

	// Unmarshall
	err = yaml.Unmarshal(source, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

// WriteConfig function that can write to the yaml file
func WriteConfig(config Config, path string) error {
	b, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path, b, 0644)
	if err != nil {
		return err
	}
	return nil
}
