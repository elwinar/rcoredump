package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

type Store interface {
	Core(uid string) (*os.File, error)
	StoreCore(uid string, src io.Reader) (int64, error)
	DeleteCore(uid string) error
	Executable(hash string) (*os.File, error)
	StoreExecutable(hash string, src io.Reader) (int64, error)
	DeleteExecutable(hash string) error
	ExecutableExists(hash string) (bool, error)
}

type FileStore struct {
	root string
}

// compile-time check that the FileStore actually implements the Store
// interface.
var _ Store = new(FileStore)

func NewFileStore(root string) (Store, error) {
	s := FileStore{root: root}
	return s, s.init()
}

func (s FileStore) init() error {
	for _, dir := range []string{
		s.root,
		filepath.Join(s.root, "executables/"),
		filepath.Join(s.root, "cores/"),
	} {
		err := os.Mkdir(dir, os.ModeDir|0774)
		if err != nil && !errors.Is(err, os.ErrExist) {
			return wrap(err, `creating data directory`)
		}
	}

	return nil
}

func (s FileStore) Core(uid string) (*os.File, error) {
	return os.Open(filepath.Join(s.root, "cores", uid))
}

func (s FileStore) StoreCore(uid string, src io.Reader) (int64, error) {
	f, err := os.Create(filepath.Join(s.root, "cores", uid))
	if err != nil {
		return 0, wrap(err, "creating core file")
	}
	defer f.Close()

	written, err := io.Copy(f, src)
	if err != nil {
		return 0, wrap(err, "reading core")
	}

	return written, nil
}

func (s FileStore) DeleteCore(uid string) error {
	return os.Remove(filepath.Join(s.root, "cores", uid))
}

func (s FileStore) Executable(hash string) (*os.File, error) {
	return os.Open(filepath.Join(s.root, "executables", hash))
}

func (s FileStore) StoreExecutable(hash string, src io.Reader) (int64, error) {
	f, err := os.Create(filepath.Join(s.root, "executables", hash))
	if err != nil {
		return 0, wrap(err, "creating executable file")
	}
	defer f.Close()

	written, err := io.Copy(f, src)
	if err != nil {
		return 0, wrap(err, "reading executable")
	}

	return written, nil
}

func (s FileStore) DeleteExecutable(hash string) error {
	return os.Remove(filepath.Join(s.root, "executables", hash))
}

func (s FileStore) ExecutableExists(hash string) (exists bool, err error) {
	exists = true
	_, err = os.Stat(filepath.Join(s.root, "executables", hash))
	if errors.Is(err, os.ErrNotExist) {
		exists = false
		err = nil
	}
	return exists, err
}
