package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Store struct{}

type Sources struct {
	GlobalPath  string
	ProjectPath string
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) GlobalPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gix", "config.toml"), nil
}

func (s *Store) ProjectPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".git", "gix.toml")
}

func (s *Store) LoadGlobal() (Values, string, error) {
	path, err := s.GlobalPath()
	if err != nil {
		return nil, "", err
	}
	values, err := s.readFile(path)
	if err != nil {
		return nil, "", err
	}
	return values, path, nil
}

func (s *Store) LoadProject(repoRoot string) (Values, string, error) {
	if repoRoot == "" {
		return make(Values), "", nil
	}
	path := s.ProjectPath(repoRoot)
	values, err := s.readFile(path)
	if err != nil {
		return nil, "", err
	}
	return values, path, nil
}

func (s *Store) LoadResolved(repoRoot string) (Config, Sources, error) {
	values := DefaultValues()
	sources := Sources{}

	globalValues, globalPath, err := s.LoadGlobal()
	if err != nil {
		return Config{}, Sources{}, err
	}
	values = values.Merge(globalValues)
	sources.GlobalPath = globalPath

	projectValues, projectPath, err := s.LoadProject(repoRoot)
	if err != nil {
		return Config{}, Sources{}, err
	}
	values = values.Merge(projectValues)
	sources.ProjectPath = projectPath
	cfg, err := values.ToConfig()
	if err != nil {
		return Config{}, Sources{}, err
	}
	return cfg, sources, nil
}

func (s *Store) SaveGlobal(values Values) (string, error) {
	path, err := s.GlobalPath()
	if err != nil {
		return "", err
	}
	if err := s.writeFile(path, values); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) SaveProject(repoRoot string, values Values) (string, error) {
	if repoRoot == "" {
		return "", fmt.Errorf("project config requires a git repository")
	}
	path := s.ProjectPath(repoRoot)
	if err := s.writeFile(path, values); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) readFile(path string) (Values, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(Values), nil
		}
		return nil, err
	}
	defer file.Close()
	values, err := ParseTOML(file)
	if err != nil {
		return nil, err
	}
	return values, nil
}

func (s *Store) writeFile(path string, values Values) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(EncodeTOML(values)), 0o644)
}
