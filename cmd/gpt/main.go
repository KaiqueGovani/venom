package main

import (
	"fmt"
	"os"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
	mod "github.com/KaiqueGovani/venom/internal/model"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/couchbase/gocb/v2"
)

const (
	bucketName     = "venom"
	scopeName      = "mindsnap"
	collectionName = "projects"
)

// #region Styles
const (
	white          = lipgloss.Color("#fafafa")
	black          = lipgloss.Color("#292a44")
	gray           = lipgloss.Color("#bbbbbb")
	purple         = lipgloss.Color("#908dfb")
	darkenedPurple = lipgloss.Color("#706ddb")
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

var baseTheme = huh.ThemeCharm()

func getBaseTheme() *huh.Theme {

	baseTheme.Focused.TextInput.Prompt = baseTheme.Focused.TextInput.Prompt.Foreground(purple)
	baseTheme.Focused.TextInput.Text = baseTheme.Focused.TextInput.Text.Foreground(white)
	baseTheme.Blurred.TextInput.Prompt = baseTheme.Blurred.TextInput.Prompt.Foreground(gray)

	baseTheme.Focused.Description = baseTheme.Focused.Description.Foreground(white).Bold(true)

	baseTheme.Focused.FocusedButton = baseTheme.Focused.FocusedButton.Foreground(white).Background(purple).Bold(true)
	baseTheme.Blurred.FocusedButton = baseTheme.Blurred.FocusedButton.Foreground(gray).Background(darkenedPurple)

	baseTheme.Blurred.Title = baseTheme.Blurred.Title.Foreground(gray)

	baseTheme.Help.ShortKey = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#909090",
		Dark:  "#fafafa",
	})

	return baseTheme
}

func styleTable(t *table.Model) {
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

	t.Help.Styles = getBaseTheme().Help

	t.Help.ShowAll = false
}

// #region State
type State int

const (
	ProjectsList State = iota
	CreateProjectForm
	EditProjectForm
	VariablesList
	Loading
	Confirm
)

// #region Model
type model struct {
	state           State
	table           table.Model
	varTable        table.Model
	customKeyMap    CustomKeyMap
	projects        map[string]mod.Project
	selectedProject *mod.Project
	form            *huh.Form
	spinner         spinner.Model
	apiHandler      *api.ApiHandler
	cluster         *gocb.Cluster
	previousState   State
	confirmCallback tea.Cmd
}

// #region KeyMap
type CustomKeyMap struct {
	LineUp    key.Binding
	LineDown  key.Binding
	Configure key.Binding
	Edit      key.Binding
	Delete    key.Binding
	Create    key.Binding
	Quit      key.Binding
	Help      key.Binding
}

func (k CustomKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Edit, k.LineUp},
		{k.LineDown, k.Quit},
		{k.Configure, k.Help},
	}
}

func (k CustomKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LineUp, k.LineDown, k.Create, k.Edit, k.Configure, k.Delete, k.Quit}
}

var customKeyMap = CustomKeyMap{
	Edit: key.NewBinding(
		key.WithKeys("e", "enter"),
		key.WithHelp("e", "\bdit 📝"),
	),
	Configure: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "\bariables 🧩"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "\belete ❌"),
	),
	Create: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "\bdd ➕"),
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
		key.WithHelp("q", "\buit 🚪"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}

// #region ProjectsTable
func createProjectsTable() table.Model {
	columns := []table.Column{
		{Title: "Project", Width: 30},
		{Title: "Folder", Width: 20},
		{Title: "File", Width: 30},
		{Title: "Vars", Width: 4},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(6),
	)

	styleTable(&t)

	return t
}

func (m *model) updateProjectsTable() {
	var listedProjects []table.Row
	for _, project := range m.projects {
		listedProjects = append(listedProjects, table.Row{project.Name, project.TargetFolder, project.FileName, fmt.Sprintf("%d", len(project.Variables))})
	}

	m.table.SetRows(listedProjects)
	m.table.GotoTop()
}

// #region ProjectForm
func createProjectForm(project *mod.Project, new bool) *huh.Form {
	fields := []huh.Field{}

	if new {
		fields = append(fields, huh.NewInput().Key("Name").Title("Project Name").Value(&project.Name))
	}

	fields = append(fields, huh.NewInput().Key("Folder").Title("Target Folder").Value(&project.TargetFolder))
	fields = append(fields, huh.NewInput().Key("File").Title("File Name").Value(&project.FileName))
	fields = append(fields, huh.NewConfirm().Key("confirm").Title("Confirm Changes").Affirmative("Yes").Negative("No"))

	form := huh.NewForm(
		huh.NewGroup(fields...),
	).WithWidth(45).WithTheme(getBaseTheme())

	return form
}

// #region VariablesTable
// Helper function to display the variables table
func (m *model) showVariablesTable() tea.Cmd {
	// Set the state to VariablesForm
	m.state = VariablesList

	// Disable the help for variable key
	m.customKeyMap.Configure.SetEnabled(false)

	// Create a table of variables (key-value pairs)
	var variableRows []table.Row
	for key, value := range m.selectedProject.Variables {
		variableRows = append(variableRows, table.Row{key, value})
	}

	// Define the columns for the variables table
	columns := []table.Column{
		{Title: "Key", Width: 40},
		{Title: "Value", Width: 44},
	}

	// Create the table
	m.varTable = table.New(
		table.WithColumns(columns),
		table.WithRows(variableRows),
		table.WithFocused(true),
		table.WithHeight(6),
	)
	styleTable(&m.varTable)

	return nil
}

// #region ConfirmForm
func createConfirmForm(customMessage ...string) *huh.Form {
	message := "Are you sure?"
	description := "This action cannot be undone."
	if len(customMessage) > 0 && customMessage[0] != "" {
		message = customMessage[0]
	}
	if len(customMessage) > 1 && customMessage[1] != "" {
		description = customMessage[1]
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().Key("confirm").Title(message).Description(description).Affirmative("Yes").Negative("No"),
		),
	).WithWidth(45).WithTheme(getBaseTheme())

	return form
}

// #region Commands
type Message struct{}
type ReturnToList struct{}

func (m *model) SetLoading() tea.Cmd {
	return func() tea.Msg {
		m.previousState = m.state
		m.state = Loading
		return Message{}
	}
}

func (m *model) showConfirmForm(callback tea.Cmd, message ...string) tea.Cmd {
	m.previousState = m.state
	m.state = Confirm
	m.confirmCallback = callback
	m.form = createConfirmForm(message...)
	return m.form.Init()
}

func (m *model) GetApiHandler() tea.Cmd {
	return func() tea.Msg {
		cluster, err := db.Connect()
		if err != nil {
			panic(err)
		}
		m.cluster = cluster

		// Get the bucket
		bucket := m.cluster.Bucket(bucketName)
		bucket.WaitUntilReady(5*time.Second, nil)

		// Get the collection
		col := cluster.Bucket(bucketName).Scope(scopeName).Collection(collectionName)

		m.apiHandler = api.NewApiHandler(bucketName, scopeName, collectionName, cluster, col)
		return Message{}
	}

}

type GetProjectsMsg struct{}

func (m *model) GetProjects() tea.Cmd {
	return func() tea.Msg {
		// Get all projects
		projects, _ := m.apiHandler.GetProjects()

		m.projects = projects
		m.updateProjectsTable()
		m.state = ProjectsList
		return GetProjectsMsg{}
	}
}

func (m *model) SelectProject() tea.Cmd {
	projectName := m.table.SelectedRow()[0]

	project, err := m.apiHandler.GetProject(projectName)
	if err != nil {
		panic(err)
	}

	*m.selectedProject = project
	m.state = EditProjectForm

	// Create a form to edit the project details
	m.form = createProjectForm(&project, false)

	return m.form.Init()
}

func (m *model) CreateProject() tea.Cmd {
	return func() tea.Msg {
		name, err := m.apiHandler.CreateProject(*m.selectedProject)
		if err != nil {
			panic(err)
		}
		m.projects[name] = *m.selectedProject
		m.updateProjectsTable()
		return ReturnToList{}
	}
}

func (m *model) UpdateProject() tea.Cmd {
	return func() tea.Msg {
		project, err := m.apiHandler.UpdateProject(m.selectedProject.Name, *m.selectedProject)
		if err != nil {
			panic(err)
		}
		m.projects[project.Name] = project
		m.updateProjectsTable()
		return ReturnToList{}
	}
}

func (m *model) DeleteProject() tea.Cmd {
	return func() tea.Msg {
		projectName := m.table.SelectedRow()[0]
		err := m.apiHandler.DeleteProject(projectName)
		if err != nil {
			panic(err)
		}

		delete(m.projects, projectName)
		m.updateProjectsTable()
		m.state = ProjectsList
		return ReturnToList{}
	}
}

// #region Init
func (m *model) Init() tea.Cmd {
	//TODO: INITIATE WITH A NICE MESSAGE/LOGO
	return tea.Batch(m.spinner.Tick, tea.Sequence(m.GetApiHandler(), m.GetProjects()))
}

// #region Update
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ReturnToList); ok {
		m.state = ProjectsList
		return m, nil
	}

	switch m.state {
	case ProjectsList:
		return m.updateProjectsList(msg)
	case EditProjectForm, CreateProjectForm:
		return m.updateProjectForm(msg)
	case VariablesList:
		return m.updateVariablesList(msg)
	case Loading:
		return m.updateLoading(msg)
	case Confirm:
		return m.updateConfirmForm(msg)
	}

	return m, nil
}

// #region UpdateLoading
func (m *model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// #region UpdateProjectsList
func (m *model) updateProjectsList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.customKeyMap.Edit):
			return m, m.SelectProject()
		case key.Matches(msg, m.customKeyMap.Configure):
			return m, tea.Sequence(m.SetLoading(), m.SelectProject(), m.showVariablesTable())
		case key.Matches(msg, m.customKeyMap.Create):
			m.selectedProject = &mod.Project{}
			m.state = CreateProjectForm
			m.form = createProjectForm(m.selectedProject, true)
			return m, m.form.Init()
		case key.Matches(msg, m.customKeyMap.Delete):
			return m, m.showConfirmForm(tea.Sequence(m.SetLoading(), m.DeleteProject()), "Are you sure you want to delete ", fmt.Sprintf("Project '%s'", m.table.SelectedRow()[0]))
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// #region UpdateProjectForm
func (m *model) updateProjectForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			m.state = m.previousState
			return m, nil
		}
	}

	var cmds []tea.Cmd
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		if !m.form.GetBool("confirm") {
			m.state = m.previousState
			return m, nil
		}

		if m.state == CreateProjectForm {
			m.selectedProject.Name = m.form.GetString("Name")
			m.selectedProject.TargetFolder = m.form.GetString("Folder")
			m.selectedProject.FileName = m.form.GetString("File")

			return m, tea.Sequence(m.SetLoading(), m.CreateProject())
		}

		if m.state == EditProjectForm {
			m.selectedProject.TargetFolder = m.form.GetString("Folder")
			m.selectedProject.FileName = m.form.GetString("File")

			return m, tea.Sequence(m.SetLoading(), m.UpdateProject())
		}
		return m, tea.Batch(cmds...)
	}

	return m, tea.Batch(cmds...)
}

// #region UpdateVariables
func (m *model) updateVariablesList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			m.state = ProjectsList
			m.customKeyMap.Configure.SetEnabled(true)
			return m, nil
		case key.Matches(msg, m.customKeyMap.Create):
			// Add a new key-value pair
			m.selectedProject.Variables["new_key"] = "new_value"

			return nil, m.showVariablesTable()
		case key.Matches(msg, m.customKeyMap.Delete):
			// Delete the selected variable
			selectedRow := m.varTable.SelectedRow()
			delete(m.selectedProject.Variables, selectedRow[0])

			return nil, m.showVariablesTable()
		}
	}

	m.varTable, _ = m.varTable.Update(msg)
	return m, nil
}

// #region UpdateConfirmForm
func (m *model) updateConfirmForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			m.state = m.previousState
			return m, nil
		}
	}

	var cmds []tea.Cmd
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		if m.form.GetBool("confirm") {
			// Execute the callback command if confirmed
			cmd = m.confirmCallback
			m.confirmCallback = nil
			return m, cmd
		}
		// Return to the previous state if not confirmed
		m.state = m.previousState
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// #region View
func (m model) View() string {
	switch m.state {
	case ProjectsList:
		s := baseStyle.Render(m.table.View()) + "\n"
		s += "\n" + m.table.Help.View(m.customKeyMap)
		return s

	case EditProjectForm:
		s := "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Editing Project: ")
		s += lipgloss.NewStyle().Foreground(white).Bold(true).Render(m.selectedProject.Name) + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s
	case CreateProjectForm:
		s := "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Creating Project: ")
		s += lipgloss.NewStyle().Foreground(white).Bold(true).Render(m.selectedProject.Name) + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s

	case VariablesList:
		s := baseStyle.Render(m.varTable.View()) + "\n"
		s += "\n" + m.table.Help.View(m.customKeyMap)
		return s

	case Loading:
		return fmt.Sprintf("\n %s%s\n\n", m.spinner.View(), lipgloss.NewStyle().Foreground(lipgloss.Color(white)).Bold(true).Render("Loading..."))

	case Confirm:
		s := baseStyle.Render(m.form.View()) + "\n"
		return s
	}

	return ""
}

// #region Main
func main() {

	// Starts the TUI application
	t := createProjectsTable()

	spinner := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("57"))),
	)
	spinner.Tick()

	m := model{Loading, t, table.Model{}, customKeyMap, map[string]mod.Project{}, &mod.Project{}, nil, spinner, &api.ApiHandler{}, nil, ProjectsList, nil}

	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	if m.cluster != nil {
		m.cluster.Close(&gocb.ClusterCloseOptions{})
	}

	println("Venom says: 'Goodbye, and remember, with great power comes great responsibility!'")
}
