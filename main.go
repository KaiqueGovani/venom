package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
)

// Estrutura projetos
type Project struct {
	Name         string
	FileName     string
	TargetFolder string
	Variables    map[string]string
}

// Mock projetos
var projects = []Project{
	{
		Name:         "Project1",
		FileName:     "config1.env",
		TargetFolder: "/path/to/project1",
		Variables: map[string]string{
			"API_KEY":    "12345",
			"DB_HOST":    "localhost",
			"DB_PORT":    "5432",
			"DEBUG_MODE": "true",
		},
	},
	{
		Name:         "Project2",
		FileName:     "config2.env",
		TargetFolder: "/path/to/project2",
		Variables: map[string]string{
			"API_KEY":    "abcde",
			"DB_HOST":    "localhost",
			"DB_PORT":    "3306",
			"DEBUG_MODE": "false",
		},
	},
}

// Modelo estado interface
type model struct {
	input   string // Digitado
	output  string // Resultado
}

// Função inicial
func initialModel() model {
	return model{
		input:  "",
		output: "Digite um comando como 'venom help' para ver os comandos disponíveis.\n",
	}
}

// Estilo "venom"
func createStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Padding(1, 0).
		Width(100).
		Foreground(lipgloss.Color("205")).
		Background(lipgloss.Color("232"))
}

// Init
func (m model) Init() tea.Cmd {
	return nil
}

// Update
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			m.output = handleCommand(m.input)
			m.input = ""
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			m.input += msg.String()
		}
	}
	return m, nil
}

// View
func (m model) View() string {
	venomTitle := figure.NewFigure("venom", "alligator", true).String()
	style := createStyle()
	return style.Render(venomTitle) + "\n\nComando: " + m.input + "\n\nResultado:\n" + m.output
}

// Interpreta os comandos
func handleCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	switch {
	case cmd == "venom help":
		return listCommands()
	case cmd == "venom configure --list":
		return listProjects()
	case strings.HasPrefix(cmd, "venom configure --add --name"):
		return addProject(cmd)
	case strings.HasPrefix(cmd, "venom configure --project") && strings.Contains(cmd, "--set"):
		return setEnvVar(cmd)
	case strings.HasPrefix(cmd, "venom configure --project") && strings.Contains(cmd, "--unset"):
		return unsetEnvVar(cmd)
	case strings.HasPrefix(cmd, "venom configure --edit --project"):
		return editProject(cmd)
	case strings.HasPrefix(cmd, "venom pull --project"):
		return pullProject(cmd)
	default:
		return "Comando inválido! Tente 'venom help' para ver a lista de comandos."
	}
}

// Listar os projetos
func listProjects() string {
	var sb strings.Builder
	sb.WriteString("Lista de Projetos:\n\n")
	for i, project := range projects {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, project.Name))
	}
	return sb.String()
}

// Adicionar um novo projeto
func addProject(cmd string) string {
	projectName := strings.TrimPrefix(cmd, "venom configure --add --name ")
	projectName = strings.TrimSpace(projectName)

	newProject := Project{
		Name:         projectName,
		FileName:     "default.env",
		TargetFolder: "/default/path",
		Variables:    map[string]string{},
	}
	projects = append(projects, newProject)

	return fmt.Sprintf("Projeto '%s' adicionado com sucesso.", projectName)
}

// Adicionar uma variável de ambiente
func setEnvVar(cmd string) string {
	parts := strings.Split(cmd, " --set ")
	if len(parts) != 2 {
		return "Comando inválido! Use: venom configure --project NomeDoProjeto --set KEY=VALUE"
	}

	projectName := strings.TrimPrefix(parts[0], "venom configure --project ")
	projectName = strings.TrimSpace(projectName)
	keyValue := parts[1]
	keyValueParts := strings.Split(keyValue, "=")

	if len(keyValueParts) != 2 {
		return "Formato de variável inválido! Use: KEY=VALUE"
	}

	key := keyValueParts[0]
	value := keyValueParts[1]

	for i, project := range projects {
		if project.Name == projectName {
			projects[i].Variables[key] = value
			return fmt.Sprintf("Variável '%s=%s' adicionada ao projeto '%s'.", key, value, projectName)
		}
	}

	return "Projeto não encontrado!"
}

// Remover uma variável de ambiente
func unsetEnvVar(cmd string) string {
	parts := strings.Split(cmd, " --unset ")
	if len(parts) != 2 {
		return "Comando inválido! Use: venom configure --project NomeDoProjeto --unset KEY"
	}

	projectName := strings.TrimPrefix(parts[0], "venom configure --project ")
	projectName = strings.TrimSpace(projectName)
	key := parts[1]

	for i, project := range projects {
		if project.Name == projectName {
			delete(projects[i].Variables, key)
			return fmt.Sprintf("Variável '%s' removida do projeto '%s'.", key, projectName)
		}
	}

	return "Projeto não encontrado!"
}

// Editar informações de um projeto
func editProject(cmd string) string {
	parts := strings.Split(cmd, " --project ")
	if len(parts) != 2 {
		return "Comando inválido! Use: venom configure --edit --project NomeDoProjeto --filename nome --target caminho"
	}

	projectParts := strings.Split(parts[1], " --")
	projectName := projectParts[0]

	var newFileName, newTargetFolder string
	for _, part := range projectParts[1:] {
		if strings.HasPrefix(part, "filename ") {
			newFileName = strings.TrimPrefix(part, "filename ")
		} else if strings.HasPrefix(part, "target ") {
			newTargetFolder = strings.TrimPrefix(part, "target ")
		}
	}

	for i, project := range projects {
		if project.Name == projectName {
			if newFileName != "" {
				projects[i].FileName = newFileName
			}
			if newTargetFolder != "" {
				projects[i].TargetFolder = newTargetFolder
			}
			return fmt.Sprintf("Projeto '%s' atualizado com sucesso.", projectName)
		}
	}

	return "Projeto não encontrado!"
}

// Puxar detalhes de um projeto específico
func pullProject(cmd string) string {
	projectName := strings.TrimPrefix(cmd, "venom pull --project ")
	projectName = strings.TrimSpace(projectName)

	for _, project := range projects {
		if project.Name == projectName {
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("Projeto: %s\n", project.Name))
			sb.WriteString(fmt.Sprintf("Arquivo de Configuração: %s\n", project.FileName))
			sb.WriteString(fmt.Sprintf("Pasta Alvo: %s\n\n", project.TargetFolder))
			sb.WriteString("Variáveis de Ambiente:\n")
			for key, value := range project.Variables {
				sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
			}
			return sb.String()
		}
	}

	return "Projeto não encontrado!"
}

// Listar todos os comandos disponíveis
func listCommands() string {
	return `Comandos Disponíveis:

1. venom configure --list               - Lista todos os projetos.
2. venom configure --add --name Nome    - Adiciona um novo projeto.
3. venom configure --project Nome --set KEY=VALUE    - Adiciona uma variável ao projeto.
4. venom configure --project Nome --unset KEY        - Remove uma variável do projeto.
5. venom configure --edit --project Nome --filename nome --target caminho    - Edita informações do projeto.
6. venom pull --project Nome    - Exibe as variáveis de um projeto específico.
`
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
		os.Exit(1)
	}
}
