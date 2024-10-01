package api

import "github.com/KaiqueGovani/venom/internal/model"

type API interface {
	GetProjects() ([]model.Project, error)
	GetProject(id string) (model.Project, error)
	CreateProject(project model.Project) (model.Project, error)
	UpdateProject(id string, project model.Project) (model.Project, error)
	DeleteProject(id string) error
}

