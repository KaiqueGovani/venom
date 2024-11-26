package app

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
	"github.com/KaiqueGovani/venom/internal/fs"
	mod "github.com/KaiqueGovani/venom/internal/model"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/common-nighthawk/go-figure"
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
	CreateVariableForm
	EditVariableForm
)

// #region Model
type model struct {
	state           State
	table           table.Model
	varTable        table.Model
	oldKey          string
	customKeyMap    CustomKeyMap
	projects        map[string]mod.Project
	selectedProject *mod.Project
	form            *huh.Form
	spinner         spinner.Model
	apiHandler      *api.ApiHandler
	cluster         *gocb.Cluster
	previousState   State
	confirmCallback tea.Cmd
	fs              fs.FileSystem
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
	Save      key.Binding
	Pull      key.Binding
}

func (k CustomKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Edit, k.LineUp},
		{k.LineDown, k.Quit},
		{k.Configure, k.Help},
	}
}

func (k CustomKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.LineUp, k.LineDown, k.Pull, k.Create, k.Edit, k.Configure, k.Delete, k.Quit}
}

var customKeyMap = CustomKeyMap{
	Pull: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("💾 p", "\bull"),
	),
	Edit: key.NewBinding(
		key.WithKeys("e", "enter"),
		key.WithHelp("📝 e", "\bdit"),
	),
	Configure: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("🧩 v", "\bariables"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("❌ d", "\belete"),
	),
	Create: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("➕ a", "\bdd"),
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
		key.WithHelp("🚪 q", "\buit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Save: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("✅ s", "\bave"),
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
	// Create a sorted slice of project names
	var projectNames []string
	for name := range m.projects {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	// Create the table rows using the sorted project names
	var listedProjects []table.Row
	for _, name := range projectNames {
		project := m.projects[name]
		listedProjects = append(listedProjects, table.Row{
			project.Name,
			project.TargetFolder,
			project.FileName,
			fmt.Sprintf("%d", len(project.Variables)),
		})
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
	).WithWidth(60).WithTheme(getBaseTheme())

	return form
}

// #region VariablesTable
// Helper function to display the variables table
func (m *model) showVariablesTable() tea.Cmd {
	// Set the state to VariablesForm
	m.state = VariablesList

	// Disable the help for variable key
	m.customKeyMap.Configure.SetEnabled(false)
	m.customKeyMap.Pull.SetEnabled(false)

	m.updateVariablesTable()

	return nil
}

func createVariablesTable() table.Model {
	// Define the columns for the variables table
	columns := []table.Column{
		{Title: "Key", Width: 50},
		{Title: "Value", Width: 50},
	}

	// Create the table
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(6),
	)
	styleTable(&t)
	return t
}

func (m *model) updateVariablesTable() {
	// Create a sorted slice of keys
	var keys []string
	for key := range m.selectedProject.Variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Create the table rows using the sorted keys
	var variableRows []table.Row
	for _, key := range keys {
		value := m.selectedProject.Variables[key]
		variableRows = append(variableRows, table.Row{key, value})
	}

	m.varTable.SetRows(variableRows)
	m.varTable.GotoTop()
}

// #region VariableForm
func createVariableForm(key, value string) *huh.Form {
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("key").Title("Variable Key").Value(&key),
			huh.NewInput().Key("value").Title("Variable Value").Value(&value),
			huh.NewConfirm().Key("confirm").Title("Add Variable").Affirmative("Yes").Negative("No"),
		),
	).WithWidth(45).WithTheme(getBaseTheme())

	return form
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
type GoToProjectsList struct{}

func (m *model) SetLoading() tea.Cmd {
	m.state = Loading
	return m.spinner.Tick
}

func (m *model) showConfirmForm(callback tea.Cmd, previousState State, message ...string) tea.Cmd {
	m.previousState = previousState
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

// #region ProjectCommands
func (m *model) GetProjects() tea.Cmd {
	return func() tea.Msg {
		// Get all projects
		projects, _ := m.apiHandler.GetProjects()

		m.projects = projects
		m.updateProjectsTable()
		return GoToProjectsList{}
	}
}

func (m *model) CreateProject() tea.Cmd {
	return func() tea.Msg {
		name, err := m.apiHandler.CreateProject(*m.selectedProject)
		if err != nil {
			panic(err)
		}
		m.projects[name] = *m.selectedProject
		return GoToProjectsList{}
	}
}

func (m *model) UpdateProject() tea.Cmd {
	return func() tea.Msg {
		project, err := m.apiHandler.UpdateProject(m.selectedProject.Name, *m.selectedProject)
		if err != nil {
			panic(err)
		}
		m.projects[project.Name] = project
		return GoToProjectsList{}
	}
}

func (m *model) DeleteProject() tea.Cmd {
	return func() tea.Msg {
		projectName := m.selectedProject.Name
		err := m.apiHandler.DeleteProject(projectName)
		if err != nil {
			panic(err)
		}
		delete(m.projects, projectName)
		return GoToProjectsList{}
	}
}

// #region VariablesCommands
func (m *model) PullVariables() tea.Cmd {
	return func() tea.Msg {
		err := m.fs.SaveVariables([]mod.Project{*m.selectedProject})
		if err != nil {
			panic(err)
		}
		return GoToProjectsList{}
	}
}

func (m *model) SaveVariables() tea.Cmd {
	return func() tea.Msg {
		_, err := m.apiHandler.UpdateProject(m.selectedProject.Name, *m.selectedProject)
		if err != nil {
			panic(err)
		}
		m.updateVariablesTable()
		m.state = VariablesList
		return Message{}
	}
}

// Add this new function to handle variable deletion
func (m *model) deleteVariable(key string) tea.Cmd {
	return tea.Sequence(m.SetLoading(), func() tea.Msg {
		delete(m.selectedProject.Variables, key)
		return Message{}
	}, m.SaveVariables())
}

// #region Init
func (m *model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, tea.Sequence(m.GetApiHandler(), m.GetProjects()))
}

// #region Update
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(GoToProjectsList); ok {
		m.state = ProjectsList
		m.updateProjectsTable()
		m.customKeyMap.Configure.SetEnabled(true)
		m.customKeyMap.Pull.SetEnabled(true)
		return m, nil
	}

	switch m.state {
	case ProjectsList:
		return m.updateProjectsList(msg)
	case EditProjectForm, CreateProjectForm:
		return m.updateProjectForm(msg)
	case VariablesList:
		return m.updateVariablesList(msg)
	case CreateVariableForm, EditVariableForm:
		return m.updateVariableForm(msg)
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
		case key.Matches(msg, m.customKeyMap.Pull):
			if len(m.table.Rows()) == 0 {
				return m, nil
			}
			*m.selectedProject = m.projects[m.table.SelectedRow()[0]]
			return m, tea.Sequence(m.SetLoading(), m.PullVariables())
		case key.Matches(msg, m.customKeyMap.Configure):
			if len(m.table.Rows()) == 0 {
				return m, nil
			}
			*m.selectedProject = m.projects[m.table.SelectedRow()[0]]
			return m, m.showVariablesTable()
		case key.Matches(msg, m.customKeyMap.Edit):
			if len(m.table.Rows()) == 0 {
				return m, nil
			}
			*m.selectedProject = m.projects[m.table.SelectedRow()[0]]
			m.state = EditProjectForm
			m.form = createProjectForm(m.selectedProject, false)
			return m, m.form.Init()
		case key.Matches(msg, m.customKeyMap.Create):
			*m.selectedProject = mod.Project{}
			m.state = CreateProjectForm
			m.form = createProjectForm(m.selectedProject, true)
			return m, m.form.Init()
		case key.Matches(msg, m.customKeyMap.Delete):
			if len(m.table.Rows()) == 0 {
				return m, nil
			}
			*m.selectedProject = m.projects[m.table.SelectedRow()[0]]
			return m, m.showConfirmForm(tea.Sequence(m.SetLoading(), m.DeleteProject()), ProjectsList, "Are you sure you want to delete ", fmt.Sprintf("Project '%s'", m.selectedProject.Name))
		}
	}

	m.table, cmd = m.table.Update(msg)

	return m, cmd
}

// #region UpdateProjectForm
func (m *model) updateProjectForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		if !m.form.GetBool("confirm") {
			return m, func() tea.Msg {
				return GoToProjectsList{}
			}
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

// #region UpdateVariablesList
func (m *model) updateVariablesList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.customKeyMap.Quit):
			m.customKeyMap.Configure.SetEnabled(true)
			return m, func() tea.Msg {
				return GoToProjectsList{}
			}
		case key.Matches(msg, m.customKeyMap.Create):
			// Change this to show the new variable form
			m.state = CreateVariableForm
			m.form = createVariableForm("", "")
			return m, m.form.Init()
		case key.Matches(msg, m.customKeyMap.Edit):
			// Like create, but set the form values
			if len(m.varTable.Rows()) == 0 {
				return m, nil
			}
			selectedRow := m.varTable.SelectedRow()
			m.state = EditVariableForm
			m.oldKey = selectedRow[0]
			m.form = createVariableForm(selectedRow[0], selectedRow[1])
			return m, m.form.Init()

		case key.Matches(msg, m.customKeyMap.Delete):
			if len(m.varTable.Rows()) == 0 {
				return m, nil
			}
			// Delete the selected variable
			selectedRow := m.varTable.SelectedRow()
			return m, m.showConfirmForm(
				m.deleteVariable(selectedRow[0]),
				VariablesList,
				"Are you sure you want to delete",
				fmt.Sprintf("Variable: %s", selectedRow[0]),
			)
		}
	}

	m.varTable, _ = m.varTable.Update(msg)
	return m, nil
}

// #region UpdateCreateVariable
// Add this new function to handle the CreateVariableForm state
func (m *model) updateVariableForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
		cmds = append(cmds, cmd)
	}

	if m.form.State == huh.StateCompleted {
		if m.form.GetBool("confirm") {
			key := m.form.GetString("key")
			value := m.form.GetString("value")
			if m.selectedProject.Variables == nil {
				m.selectedProject.Variables = make(map[string]string)
			}

			if m.state == CreateVariableForm {
				m.selectedProject.Variables[key] = value
			}

			if m.state == EditVariableForm {
				m.selectedProject.Variables[key] = value
				if (m.oldKey != key){ 
					delete(m.selectedProject.Variables, m.oldKey)
				}
				m.oldKey = ""
			}

			m.projects[m.selectedProject.Name] = *m.selectedProject
			return m, tea.Sequence(m.SetLoading(), m.SaveVariables())
		}
		m.state = VariablesList
		return m, nil
	}

	return m, tea.Batch(cmds...)
}

// #region UpdateConfirmForm
func (m *model) updateConfirmForm(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	venomTitle := figure.NewFigure("venom", "alligator", true).String()
	s := lipgloss.NewStyle().Align(lipgloss.Center).
		Foreground(purple).Render(venomTitle) + "\n"

	switch m.state {
	case ProjectsList:
		s += baseStyle.Render(m.table.View()) + "\n"
		s += "\n" + m.table.Help.View(m.customKeyMap)
		return s

	case EditProjectForm:
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Editing Project: ")
		s += lipgloss.NewStyle().Foreground(white).Bold(true).Render(m.selectedProject.Name) + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s
	case CreateProjectForm:
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Creating Project: ")
		s += lipgloss.NewStyle().Foreground(white).Bold(true).Render(m.selectedProject.Name) + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s

	case VariablesList:
		s += baseStyle.Render(m.varTable.View()) + "\n"
		s += "\n" + m.table.Help.View(m.customKeyMap)
		return s
	case CreateVariableForm:
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Adding New Variable") + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s
	case EditVariableForm:
		s += "\n" + lipgloss.NewStyle().Bold(true).Foreground(purple).Render("Editing Variable: ") + "\n"
		s += lipgloss.NewStyle().Foreground(white).Bold(true).Render(m.oldKey) + "\n"
		s += baseStyle.Render(m.form.View()) + "\n"
		return s

	case Loading:
		return s + fmt.Sprintf("\n %s%s\n\n", m.spinner.View(), lipgloss.NewStyle().Foreground(lipgloss.Color(white)).Bold(true).Render("Loading..."))

	case Confirm:
		s += baseStyle.Render(m.form.View()) + "\n"
		return s
	}

	return ""
}

// #region Main
func RunApp() {

	// Starts the TUI application
	t := createProjectsTable()

	v := createVariablesTable()

	spinner := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("57"))),
	)
	spinner.Tick()

	fs := fs.New()

	m := model{Loading, t, v, "", customKeyMap, map[string]mod.Project{}, &mod.Project{}, nil, spinner, &api.ApiHandler{}, nil, ProjectsList, nil, fs}

	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	if m.cluster != nil {
		m.cluster.Close(&gocb.ClusterCloseOptions{})
	}

	println(lipgloss.NewStyle().Align(lipgloss.Center).
		Foreground(purple).
		Bold(true).
		Italic(true).
		Underline(true).
		Render("~ WE ARE VENOM ~"))
}
