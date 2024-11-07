package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
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

var a *api.ApiHandler

func main() {
	cluster, err := initializeDatabase()
	if err != nil {
		log.Fatal(err)
	}
	defer cluster.Close(&gocb.ClusterCloseOptions{})

	a = api.NewApiHandler(bucketName, scopeName, collectionName, cluster, getCollection(cluster))

	if len(os.Args) < 2 {
		helpCmd()
		return
	}

	mainCmd := os.Args[1]
	switch mainCmd {
	case "configure":
		configureCmd()
	case "pull":
		pullCmd()
	case "help":
		helpCmd()
	default:
		log.Fatalf("Command '%s' not recognized.\n", mainCmd)
	}
}

// initializeDatabase sets up the database connection and returns the cluster.
func initializeDatabase() (*gocb.Cluster, error) {
	cluster, err := db.Connect()
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

// getCollection retrieves the collection from the specified bucket.
func getCollection(cluster *gocb.Cluster) *gocb.Collection {
	bucket := cluster.Bucket(bucketName)
	bucket.WaitUntilReady(5*time.Second, nil)
	return bucket.Scope(scopeName).Collection(collectionName)
}

// configureCmd handles the configuration commands.
func configureCmd() {
	configureSet := flag.NewFlagSet("configure", flag.ExitOnError)
	defineFlags(configureSet)

	if err := configureSet.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	executeConfigureCommand(configureSet)
}

// defineFlags defines the flags used in the configure command.
func defineFlags(set *flag.FlagSet) {
	set.Bool("list", false, "List configurations")
	set.Bool("add", false, "Add a new configuration")
	set.String("name", "", "Name of the project to add or modify")
	set.String("set", "", "Set a variable in the format KEY=VALUE")
	set.String("unset", "", "Remove a specified key")
	set.String("filename", "", "Filename associated with the project")
	set.String("target", "", "Target folder path")
}

// executeConfigureCommand executes the logic for the configure command based on the flags.
func executeConfigureCommand(set *flag.FlagSet) {
	name := set.Lookup("name").Value.String()
	if set.Lookup("list").Value.String() == "true" {
		listProjects()
	} else if set.Lookup("add").Value.String() == "true" {
		addProject(name)
	} else if set.Lookup("set").Value.String() != "" {
		setProjectVariable(name, set.Lookup("set").Value.String())
	} else if set.Lookup("unset").Value.String() != "" {
		unsetProjectVariable(name, set.Lookup("unset").Value.String())
	} else if set.Lookup("filename").Value.String() != "" && set.Lookup("target").Value.String() != "" {
		editProject(name, set.Lookup("filename").Value.String(), set.Lookup("target").Value.String())
	} else {
		fmt.Println("No valid command provided for 'configure'.")
	}
}

// listProjects lists all projects.
func listProjects() {
	projects, err := a.GetProjects()
	handleError(err)

	fmt.Print("\nProjects:\n\n")
    
	for _, project := range projects {
		fmt.Printf("Project Name: %s\n", project.Name)
		fmt.Printf("  File: %s\n", project.FileName)
		fmt.Printf("  Target Folder: %s\n", project.TargetFolder)
		fmt.Printf("  Variables (%d):\n", len(project.Variables))

		for key, value := range project.Variables {
			// Mask sensitive data with a placeholder
			if strings.Contains(key, "SECRET") || strings.Contains(key, "KEY") {
				value = "*****"
			}
			fmt.Printf("    - %s: %s\n", key, value)
		}
		fmt.Println()
	}
}

// addProject adds a new project.
func addProject(name string) {
	newProject := model.Project{
		Name:         name,
		FileName:     "",
		TargetFolder: "",
		Variables:    map[string]string{},
	}

	_, err := a.CreateProject(newProject)
	handleError(err)

	fmt.Printf("Added project with name: %s\n", name)
}

// setProjectVariable sets a variable for a project.
func setProjectVariable(name, set string) {
	project, err := a.GetProject(name)
	handleError(err)

	key, value, found := strings.Cut(set, "=")
	if !found {
		log.Fatalf("Invalid set format: %s", set)
	}

	project.Variables[key] = value
	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Set %s = %s for project %s\n", key, value, project.Name)
}

// unsetProjectVariable removes a variable from a project.
func unsetProjectVariable(name, unset string) {
	project, err := a.GetProject(name)
	handleError(err)

	if _, ok := project.Variables[unset]; !ok {
		log.Fatalf("Key %s not found in project %s", unset, project.Name)
	} 

	delete(project.Variables, unset)
	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Removed key %s from project %s\n", unset, project.Name)
}

// editProject edits the project details.
func editProject(name, filename, target string) {
	project, err := a.GetProject(name)
	handleError(err)

	project.FileName = filename
	project.TargetFolder = target

	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Updated project %s with new file: %s and target: %s\n", project.Name, filename, target)
}

// handleError checks for errors and logs appropriately.
func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// pullCmd retrieves project variables and saves them to the file system.
func pullCmd() {
	pullSet := flag.NewFlagSet("pull", flag.ExitOnError)
	projectName := pullSet.String("name", "", "Specify project name to pull")

	if err := pullSet.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
	}

	if *projectName != "" {
			// Pull only the specified project
			project, err := a.GetProject(*projectName)
			handleError(err)

			fs := fs.New()
			err = fs.SaveVariables([]model.Project{project})
			handleError(err)

			log.Printf("Project %s saved successfully.\n", project.Name)
	} else {
			// Pull all projects
			projects, err := a.GetProjects()
			handleError(err)

			fs := fs.New()
			projectValues := convertProjectsToSlice(projects)

			err = fs.SaveVariables(projectValues)
			handleError(err)

			log.Println("All projects saved successfully.")
	}
}

// helpCmd lists all available commands with brief descriptions.
func helpCmd() {
	fmt.Println("\nAvailable commands:\n")
	fmt.Println("  configure  - Manage project configurations. Subcommands:")
	fmt.Println("    --list           - List all project configurations.")
	fmt.Println("    --add            - Add a new project. Requires --name.")
	fmt.Println("    --name           - Specify project name for adding or editing.")
	fmt.Println("    --set KEY=VALUE  - Set a variable for the specified project.")
	fmt.Println("    --unset KEY      - Remove a variable from the specified project.")
	fmt.Println("    --filename       - Set the filename associated with the project.")
	fmt.Println("    --target         - Set the target folder for the project.")
	fmt.Println()
	fmt.Println("  pull       - Retrieve project variables and save them to the file system.")
	fmt.Println("    --name           - (Optional) Specify the project to pull. If omitted, pulls all projects.")
	fmt.Println()
	fmt.Println("  help       - List all available commands with brief descriptions.")
	fmt.Println()
	fmt.Println("Example usage:")
	fmt.Println("  venom configure --add --name MyProject")
	fmt.Println("  venom pull --name MyProject")
}

// convertProjectsToSlice converts the map of projects to a slice.
func convertProjectsToSlice(projects map[string]model.Project) []model.Project {
	var projectValues []model.Project
	for _, project := range projects {
		projectValues = append(projectValues, project)
	}
	return projectValues
}
