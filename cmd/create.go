/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"scratch-container/pkg/container"

	"github.com/spf13/cobra"
)

var cfg container.ContainerConfig

// TODO later add flags for other fields available in the ContainerConfig

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create --image [image]",
	Short: "Create a new container from an image",
	Long: `Pull an image and create a container bundle from it.

Examples:
  con create mycontainer --image alpine:latest
  con create mycontainer --image ubuntu:22.04
  con create mycontainer --image gcr.io/distroless/base`,
	Args: cobra.NoArgs,
	RunE: runCreate,
}

func init() {

	createCmd.Flags().StringVarP(
		&cfg.Image,
		"image", "i",
		"",
		"image reference to pull (e.g. alpine:latest)",
	)
	createCmd.MarkFlagRequired("image")

	rootCmd.AddCommand(createCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func runCreate(cmd *cobra.Command, args []string) error {
	con, err := container.CreateContainer(&cfg)
	if err != nil {
		return fmt.Errorf("error creating container: %w\n", err)
	}

	data, _ := json.MarshalIndent(con, " ", " ")
	fmt.Printf("\nContainer %q created successfully\n", con.ID)
	fmt.Println(string(data))

	return nil
}
