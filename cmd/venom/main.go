package main

import (
	"fmt"
	"log"
	"time"

	"github.com/KaiqueGovani/venom/internal/model"
	"github.com/couchbase/gocb/v2"
	"github.com/google/uuid"
)

func main() {

	options := gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: username,
			Password: password,
		},
	}

	// Sets a pre-configured profile called "wan-development" to help avoid latency issues
	// when accessing Capella from a different Wide Area Network
	// or Availability Zone (e.g. your laptop).
	if err := options.ApplyProfile(gocb.ClusterConfigProfileWanDevelopment); err != nil {
		log.Fatal(err)
	}

	// Initialize the Connection
	cluster, err := gocb.Connect("couchbases://"+connectionString, options)
	if err != nil {
		log.Fatal(err)
	}

	bucket := cluster.Bucket(bucketName)

	err = bucket.WaitUntilReady(5*time.Second, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Get a reference to the default collection, required for older Couchbase server versions
	// col := bucket.DefaultCollection()

	projects := bucket.Scope("mindsnap").Collection("projects")

    insertId := uuid.New().String()

	_, err = projects.Upsert(insertId,
		model.Project{
			Name:         "MindSnap Frontend",
			FileName:     ".env.production.local",
			TargetFolder: "/frontend",
			Variables:    map[string]string{},
		}, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Get the document back
	getResult, err := projects.Get(insertId, nil)
	if err != nil {
		log.Fatal(err)
	}

	var project model.Project
	err = getResult.Content(&project)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Project: %v\n", project)
}
