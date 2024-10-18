package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/KaiqueGovani/venom/internal/api"
	"github.com/KaiqueGovani/venom/internal/db"
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

type model struct {
	table table.Model
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m, tea.Batch(
				tea.Printf("Let's go to %s!", m.table.SelectedRow()[1]),
			)
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

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


	columns := []table.Column{
		{Title: "ID", Width: 4},
		{Title: "Project", Width: 10},
		{Title: "Folder", Width: 10},
		{Title: "File", Width: 10},
		{Title: "Variables", Width: 4},
	}

	var listedProjects []table.Row
	for id, project := range projects {
		listedProjects = append(listedProjects, table.Row{id, project.Name, project.TargetFolder, project.FileName, fmt.Sprintf("%d", len(project.Variables))})
	}

	rows := listedProjects

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{t}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}