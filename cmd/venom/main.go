package main

import (
	"fmt"
	"log"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
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

	// Delete the project
	err = a.DeleteProject("Testando")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Deleted project with ID:", "Testando")
}
