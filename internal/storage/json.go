package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cadmium228/GoToBedAlready/internal/domain"
)

type JSON struct {
	Path string
}

func (s JSON) Load() (domain.Document, error) {
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return domain.Document{Preferences: domain.DefaultPreferences()}, nil
	}
	if err != nil {
		return domain.Document{}, err
	}
	document := domain.Document{Preferences: domain.DefaultPreferences()}
	if err := json.Unmarshal(data, &document); err != nil {
		return domain.Document{}, fmt.Errorf("decode settings: %w", err)
	}
	return document, nil
}

func (s JSON) Save(document domain.Document) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(s.Path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), s.Path)
}
