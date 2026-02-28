package container

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"scratch-container/pkg/identity"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
)

const BASE_DIR = "/var/lib/con"

type ContainerConfig struct {
	Image   string
	Command []string
	Env     []string
	Labels  map[string]string
}

func CreateContainer(cfg *ContainerConfig) (*Container, error) {
	id := identity.GenerateID()
	name := identity.GenerateName()
	bundlePath := filepath.Join(BASE_DIR, id)
	rootfsPath := filepath.Join(bundlePath, "rootfs")

	if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
		err := os.MkdirAll(rootfsPath, 0700)
		if err != nil {
			return nil, fmt.Errorf("error creating bundle and rootfs dirs for container: %w\n", err)
		}
	}

	img, err := crane.Pull(cfg.Image)
	if err != nil {
		return nil, fmt.Errorf("error pulling image when creating container: %w\n", err)
	}

	extract(img, rootfsPath)

	s := &State{
		Status: StatusCreated,
		PID:    0,
	}

	stateData, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json")
	}

	stateJson := fmt.Sprintf("state.json")
	err = os.WriteFile(stateJson, stateData, 0644)
	if err != nil {
		return nil, fmt.Errorf("error creating state file %s: %w", stateJson, err)
	}

	c := &Container{
		ID:         id,
		Name:       name,
		Image:      cfg.Image,
		Command:    cfg.Command,
		Env:        cfg.Env,
		Labels:     cfg.Labels,
		BundlePath: bundlePath,
		RootFS:     rootfsPath,
		CreatedAt:  time.Now(),
	}

	containerData, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("error marshaling json")
	}

	containerJson := fmt.Sprintf("%s.json", c.ID)
	err = os.WriteFile(containerJson, containerData, 0644)
	if err != nil {
		return nil, fmt.Errorf("error creating container file %s: %w", containerJson, err)
	}

	return c, nil
}

func GetContainer(c Container) (Container, error) {
	return c, nil
}

func UpdateContainer(c Container) (Container, error) {
	return c, nil
}

func DeleteContainer(c Container) error {
	return nil
}

func extract(img v1.Image, rootfsPath string) error {
	rc := mutate.Extract(img)

	defer rc.Close()

	return untar(rc, rootfsPath)
}

func untar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(header.Mode))
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
		}
	}

	return nil
}
