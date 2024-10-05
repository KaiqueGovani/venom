package api

import (
	"fmt"

	"github.com/KaiqueGovani/venom/internal/model"
	"github.com/couchbase/gocb/v2"
	"github.com/google/uuid"
)

type API interface {
	GetProjects() (map[string]model.Project, error)
	GetProject(id string) (model.Project, error)
	CreateProject(project model.Project) (model.Project, error)
	UpdateProject(id string, project model.Project) (model.Project, error)
	DeleteProject(id string) error
}

type ApiHandler struct {
	Bucket             string
	Scope              string
	Collection         string
	Cluster            *gocb.Cluster
	ProjectsCollection *gocb.Collection
}

func NewApiHandler(bucket string, scope string, collection string, cluster *gocb.Cluster, projectsCollection *gocb.Collection) ApiHandler {
	return ApiHandler{
		Bucket:             bucket,
		Scope:              scope,
		Collection:         collection,
		Cluster:            cluster,
		ProjectsCollection: projectsCollection,
	}
}

func (a ApiHandler) GetProjects() (map[string]model.Project, error) {
	results, err := a.Cluster.Query(
		fmt.Sprintf("SELECT META().id, * FROM %s.%s.%s", a.Bucket, a.Scope, a.Collection),
		&gocb.QueryOptions{
			// Note that we set Adhoc to true to prevent this query being run as a prepared statement.
			Adhoc:    true,
			Readonly: true,
		})
	if err != nil {
		return nil, err
	}

	projects := make(map[string]model.Project)
	var result struct {
		ID      string        `json:"id"`
		Project model.Project `json:"projects"`
	}
	for results.Next() {
		err := results.Row(&result)
		if err != nil {
			return nil, err
		}

		// Add the value to the projects map
		projects[result.ID] = result.Project
	}

	// always check for errors after iterating
	err = results.Err()
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (a ApiHandler) GetProject(id string) (model.Project, error) {
	var project model.Project

	result, err := a.ProjectsCollection.Get(id, &gocb.GetOptions{})
	if err != nil {
		return project, err
	}

	err = result.Content(&project)
	if err != nil {
		return project, err
	}
	return project, nil
}

func (a ApiHandler) CreateProject(project model.Project) (string, error) {
	key := uuid.New().String()
	_, err := a.ProjectsCollection.Upsert(key, project, nil)
	if err != nil {
		return "", err
	}
	return key, nil
}

func (a ApiHandler) UpdateProject(id string, project model.Project) (model.Project, error) {
	_, err := a.ProjectsCollection.Upsert(id, project, nil)
	if err != nil {
		return project, err
	}
	return project, nil
}

func (a ApiHandler) DeleteProject(id string) error {
	_, err := a.ProjectsCollection.Remove(id, nil)
	if err != nil {
		return err
	}
	return nil
}
