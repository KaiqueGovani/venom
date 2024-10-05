package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
	"github.com/KaiqueGovani/venom/internal/model"
	"github.com/couchbase/gocb/v2"
)

const (
	bucketName     = "venom"
	scopeName      = "mindsnap"
	collectionName = "projects"
)

func main() {
	cluster, err := db.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer cluster.Close(&gocb.ClusterCloseOptions{})

	// Get the bucket
	bucket := cluster.Bucket(bucketName)
	bucket.WaitUntilReady(5*time.Second, nil)

	// Get the collection
	col := cluster.Bucket(bucketName).Scope(scopeName).Collection(collectionName)

	a := api.NewApiHandler(bucketName, scopeName, collectionName, cluster, col)

	// Get all projects
	projects, _ := a.GetProjects()
	fmt.Println(projects)

	projectsJSON, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(projectsJSON))

	// Get a single project
	project, _ := a.GetProject("7798d7a7-0d4d-4c38-b637-781e29fb0344")

	projectJSON, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(projectJSON))

	// Create a new project
	newProject := model.Project{
		Name:         "New Project",
		FileName:     "new_project",
		TargetFolder: "new_project",
		Variables:    map[string]string{"key": "value"},
	}

	createdId, err := a.CreateProject(newProject)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Created ID:", createdId)

	// Update the project
	updatedProject := model.Project{
		Name:         "Updated Project",
		FileName:     "updated_project",
		TargetFolder: "updated_project",
		Variables:    map[string]string{"key": "value"},
	}

	updated, err := a.UpdateProject(createdId, updatedProject)
	if err != nil {
		log.Fatal(err)
	}

	updatedJSON, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(updatedJSON))

	// Delete the project
	err = a.DeleteProject(createdId)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Deleted project with ID:", createdId)
}
