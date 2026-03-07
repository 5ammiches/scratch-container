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

	// create container state data json as "created"
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

	// create container data json
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

	containerData, err := json.MarshalIndent(c, " ", " ")
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

func ListContainers() ([]ContainerListItem, error) {
	dirs, err := os.ReadDir(BASE_DIR)
	if err != nil {
		return nil, fmt.Errorf("error reading directory: %w\n", err)
	}

	var res []ContainerListItem
	for _, item := range dirs {
		if !item.IsDir() {
			continue
		}

		containerFile := filepath.Join(BASE_DIR, item.Name(), "container.json")
		stateFile := filepath.Join(BASE_DIR, item.Name(), "state.json")

		c, err := readContainerData(containerFile)
		if err != nil {
			return nil, fmt.Errorf("error reading container data in %s: %w\n", item.Name(), err)
		}

		s, err := readStateData(stateFile)
		if err != nil {
			return nil, fmt.Errorf("error reading state data in %s: %w\n", item.Name(), err)
		}

		cli := ContainerListItem{
			ID:        c.ID,
			Name:      c.Name,
			Image:     c.Image,
			Status:    s.Status,
			RootFS:    c.RootFS,
			CreatedAt: c.CreatedAt,
		}
		res = append(res, cli)
	}

	return res, nil
}

func readContainerData(dir string) (*Container, error) {
	containerData, err := os.ReadFile(dir)
	if err != nil {
		return nil, fmt.Errorf("Error reading container data in %s: %w\n", dir, err)
	}

	c := &Container{}
	json.Unmarshal(containerData, &c)
	return c, nil
}

func readStateData(dir string) (*State, error) {
	stateData, err := os.ReadFile(dir)
	if err != nil {
		return nil, fmt.Errorf("Error reading state data in %s: %w\n", dir, err)
	}

	s := &State{}
	json.Unmarshal(stateData, &s)
	return s, nil
}

// TODO return all fields of container + status for container by ID
func GetContainer(ID string) (*Container, *State, error) {
	dir := filepath.Join(BASE_DIR, ID)

	c, err := readContainerData(dir)
	if err != nil {

	}

	return nil, nil, nil
}

// TODO in future add name or ID option to CLI get container command then resolve name->ID handle

// TODO implement container update
func UpdateContainer(c *Container) error {
	return nil
}

// TODO implement container deletion
func DeleteContainer(c *Container) error {
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
