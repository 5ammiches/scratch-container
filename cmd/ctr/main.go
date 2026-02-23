package main

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/crane"
)

func CreateContainer(image string) error {
	img, err := crane.Pull(image)
	if err != nil {
		fmt.Println("Error pulling image:", err)
		return err
	}

	manifest, _ := img.Digest()
	config, _ := img.ConfigName()

	m, _ := img.Manifest()

	fmt.Println("Manifest digest:", manifest)
	fmt.Println("Config digest:  ", config)
	fmt.Println("Layer count:    ", len(m.Layers))
	fmt.Println("Layer digests:")
	for _, l := range m.Layers {
		fmt.Println(" ", l.Digest)
	}

	return nil

}

func main() {

	if err := CreateContainer("openjdk:27-ea-trixie"); err != nil {
		fmt.Println("Failed:", err)
	}
}
