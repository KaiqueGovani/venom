package fs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/KaiqueGovani/venom/internal/model"
)

type FileSystem interface {
	SaveVariables([]model.Project) error
}

type fs struct{}

func New() FileSystem {
	return &fs{}
}

func (f *fs) SaveVariables(projects []model.Project) error {
	// Get the current working directory
	basePath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	for _, project := range projects {
		// Create directories if they don't exist
		targetFolder := path.Join(basePath, project.TargetFolder)
		if err := os.MkdirAll(targetFolder, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directories: %w", err)
		}

		// Create the file path
		filePath := filepath.Join(targetFolder, project.FileName)

		// Check if the file already exists
		// TODO: Separate this logic so the user can be asked if it should continue
		if _, err := os.Stat(filePath); err == nil {
			fmt.Printf("Warning: File %s already exists and will be overridden\n", filePath)
		}

		// Create or truncate the file
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		// Write variables to the file if they are not empty
		if len(project.Variables) > 0 {
			for key, value := range project.Variables {
				if _, err := file.WriteString(fmt.Sprintf("%s=%s\n", key, value)); err != nil {
					return fmt.Errorf("failed to write to file: %w", err)
				}
			}
		}
	}

	return nil
}
