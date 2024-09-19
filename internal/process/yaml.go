package process

import (
	"encoding/json"
	"errors"
	"regexp"
)

type Yaml struct {
	Env     string            `json:"env" yaml:"env"`
	App     string            `json:"app" yaml:"app"`
	Start   string            `json:"start" yaml:"start"`
	With    map[string]string `json:"with" yaml:"with"`
	Probe   *Probe            `json:"probe" yaml:"probe"`
	Workdir string            `json:"workdir" yaml:"workdir"`
}

func (f *Yaml) IsValid() error {
	noSpacePattern := regexp.MustCompile(`^\S+$`)
	if !noSpacePattern.MatchString(f.Env) {
		return errors.New("invalid env")
	}
	if !noSpacePattern.MatchString(f.App) {
		return errors.New("invalid app")
	}
	if f.Start == "" {
		return errors.New("invalid service")
	}
	if f.Workdir == "" {
		return errors.New("invalid workdir")
	}
	return nil
}

func (f *Yaml) FromDB(content []byte) error {
	return json.Unmarshal(content, f)
}

func (f *Yaml) ToDB() ([]byte, error) {
	return json.Marshal(f)
}
