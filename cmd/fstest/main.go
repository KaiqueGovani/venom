package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
	"github.com/KaiqueGovani/venom/internal/fs"
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

	// Create the file system manager
	fs := fs.New()

	// Get each project from the projects map into a slice
	var projectValues []model.Project
	for _, project := range projects {
		projectValues = append(projectValues, project)
	}

	// Save the variables to the file system
	err = fs.SaveVariables(projectValues)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Projects saved to the file system")
}
