package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
	mod "github.com/KaiqueGovani/venom/internal/model"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/couchbase/gocb/v2"
)

const (
	bucketName     = "venom"
	scopeName      = "mindsnap"
	collectionName = "projects"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

// #region State
type State int

const (
	ProjectsList State = iota
	ProjectDetails
)

// #region Model
type model struct {
	state           State
	table           table.Model
	customKeyMap    CustomKeyMap
	selectedProject mod.Project
	apiHandler      api.ApiHandler
}

// #region KeyMap
var customKeyMap = CustomKeyMap{
	Select: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("↩", "✅"),
	),
	LineUp: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	LineDown: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

type CustomKeyMap struct {
	LineUp   key.Binding
	LineDown key.Binding
	Quit     key.Binding
	Select   key.Binding
}

func (k CustomKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.LineUp, k.LineDown, k.Quit}
}

func (k CustomKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.LineUp, k.LineDown},
		{k.Quit},
	}
}

type Success struct{}

// #region Commands
func (m *model) SelectProject() tea.Cmd {
	projectName := m.table.SelectedRow()[0]

	project, err := m.apiHandler.GetProject(projectName)
	if err != nil {
		panic(err)
	}

	m.selectedProject = project

	return func() tea.Msg {
		return Success{}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Println("Welcome to Venom! Use the arrow keys to navigate, press enter to select, and press q to quit.")
}

// #region Update
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.state {
	case ProjectsList:
		return m.updateProjectsList(msg)

	case ProjectDetails:
		return m.updateProjectDetails(msg)

	}

	return m, nil
}

func (m *model) updateProjectsList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.customKeyMap.Select):
			return m, m.SelectProject()
		}

	case Success:
		// Re-render view with the selected project
		m.state = ProjectDetails
		return m, tea.Println("Selected project: " + m.selectedProject.Name)
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) updateProjectDetails(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			return m, tea.Quit
		}
	}

	return m, cmd
}

// #region View
func (m model) View() string {
	if m.selectedProject.Name == "" {
		s := baseStyle.Render(m.table.View()) + "\n"

		s += "\n" + m.table.Help.View(m.customKeyMap)

		return s
	} else {

		return fmt.Sprintf("Selected project: %s\n", m.selectedProject.Name)
	}
}

// #region Main
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

	
	var listedProjects []table.Row
	for _, project := range projects {
		listedProjects = append(listedProjects, table.Row{project.Name, project.TargetFolder, project.FileName, fmt.Sprintf("%d", len(project.Variables))})
	}

	rows := listedProjects

	//#region Table
	columns := []table.Column{
		{Title: "Project", Width: 30},
		{Title: "Folder", Width: 10},
		{Title: "File", Width: 30},
		{Title: "Vars", Width: 4},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(4),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	//#region Help
	t.Help.Styles.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#f0f0f0",
	})

	t.Help.ShowAll = false

	m := model{ProjectsList, t, customKeyMap, mod.Project{}, a}
	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	println("Venom says: 'Goodbye, and remember, with great power comes great responsibility!'")
}
