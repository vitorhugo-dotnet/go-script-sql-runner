package profile

import (
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

func Decode(r io.Reader) (Profile, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)

	var p Profile
	if err := decoder.Decode(&p); err != nil {
		return Profile{}, fmt.Errorf("decode profile yaml: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Profile{}, fmt.Errorf("decode profile yaml: multiple YAML documents are not supported")
		}
		return Profile{}, fmt.Errorf("decode profile yaml trailing document: %w", err)
	}

	if err := Validate(p); err != nil {
		return Profile{}, fmt.Errorf("validate profile: %w", err)
	}
	return p, nil
}

func Encode(w io.Writer, p Profile) error {
	if err := Validate(p); err != nil {
		return fmt.Errorf("validate profile: %w", err)
	}
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(p); err != nil {
		return fmt.Errorf("encode profile yaml: %w", err)
	}
	return nil
}
