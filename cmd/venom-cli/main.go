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
		log.Fatal("Comando insuficiente, por favor, use um comando após 'venom'.")
	}

	mainCmd := os.Args[1]
	switch mainCmd {
	case "configure":
		configureCmd()
	case "pull":
		pullCmd()
	default:
		log.Fatalf("Comando '%s' não reconhecido.\n", mainCmd)
	}
}

// initializeDatabase configura a conexão com o banco de dados e retorna o cluster.
func initializeDatabase() (*gocb.Cluster, error) {
	cluster, err := db.Connect()
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

// getCollection obtém a coleção do bucket especificado.
func getCollection(cluster *gocb.Cluster) *gocb.Collection {
	bucket := cluster.Bucket(bucketName)
	bucket.WaitUntilReady(5*time.Second, nil)
	return bucket.Scope(scopeName).Collection(collectionName)
}

// configureCmd gerencia os comandos de configuração.
func configureCmd() {
	configureSet := flag.NewFlagSet("configure", flag.ExitOnError)
	defineFlags(configureSet)

	if err := configureSet.Parse(os.Args[2:]); err != nil {
		log.Fatal(err)
	}

	executeConfigureCommand(configureSet)
}

// defineFlags define as flags usadas no comando configure.
func defineFlags(set *flag.FlagSet) {
	set.Bool("list", false, "Listar configurações")
	set.Bool("add", false, "Adicionar configuração")
	set.String("name", "", "Nome do projeto para adicionar")
	set.String("set", "", "Configurar KEY=VALUE")
	set.String("unset", "", "Remover chave especificada")
	set.String("filename", "", "Nome do arquivo")
	set.String("target", "", "Caminho de destino")
}

// executeConfigureCommand executa a lógica do comando de configuração baseado nas flags.
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
		fmt.Println("Nenhum comando válido foi passado para 'configure'.")
	}
}

// listProjects lista todos os projetos.
func listProjects() {
	projects, err := a.GetProjects()
	handleError(err)

	fmt.Print("\nProjetos:\n\n")
    
	for _, project := range projects {
		fmt.Printf("Nome do Projeto: %s\n", project.Name)
		fmt.Printf("  Arquivo: %s\n", project.FileName)
		fmt.Printf("  Pasta de Destino: %s\n", project.TargetFolder)
		fmt.Printf("  Variáveis (%d):\n", len(project.Variables))

		for key, value := range project.Variables {
			// Para não expor dados sensíveis, você pode substituir por um placeholder
			if strings.Contains(key, "SECRET") || strings.Contains(key, "KEY") {
				value = "*****"
			}
			fmt.Printf("    - %s: %s\n", key, value)
		}
		fmt.Println()
	}
}

// addProject adiciona um novo projeto.
func addProject(name string) {
	newProject := model.Project{
		Name:         name,
		FileName:     "",
		TargetFolder: "",
		Variables:    map[string]string{},
	}

	_, err := a.CreateProject(newProject)
	handleError(err)

	fmt.Printf("Adicionando projeto com o nome: %s\n", name)
}

// setProjectVariable configura uma variável para um projeto.
func setProjectVariable(name, set string) {
	project, err := a.GetProject(name)
	handleError(err)

	key, value, found := strings.Cut(set, "=")
	if !found {
		log.Fatalf("invalid set format: %s", set)
	}

	project.Variables[key] = value
	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Configurando %s = %s para o projeto %s\n", key, value, project.Name)
}

// unsetProjectVariable remove uma variável de um projeto.
func unsetProjectVariable(name, unset string) {
	project, err := a.GetProject(name)
	handleError(err)

	if _, ok := project.Variables[unset]; !ok {
		log.Fatalf("chave %s nao encontrada no projeto %s", unset, project.Name)
	}

	delete(project.Variables, unset)
	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Removendo chave %s do projeto %s\n", unset, project.Name)
}

// editProject edita as informações de um projeto.
func editProject(name, filename, target string) {
	project, err := a.GetProject(name)
	handleError(err)

	project.FileName = filename
	project.TargetFolder = target

	_, err = a.UpdateProject(name, project)
	handleError(err)

	fmt.Printf("Editando projeto %s com novo arquivo: %s e destino: %s\n", project.Name, filename, target)
}

// handleError verifica erros e faz log apropriado.
func handleError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// pullCmd recupera variáveis de projetos e as salva no sistema de arquivos.
func pullCmd() {
	projects, err := a.GetProjects()
	handleError(err)

	fs := fs.New()
	projectValues := convertProjectsToSlice(projects)

	err = fs.SaveVariables(projectValues)
	handleError(err)

	log.Println("Projetos salvos com sucesso.")
}

// convertProjectsToSlice converte o mapa de projetos para um slice.
func convertProjectsToSlice(projects map[string]model.Project) []model.Project {
	var projectValues []model.Project
	for _, project := range projects {
		projectValues = append(projectValues, project)
	}
	return projectValues
}
