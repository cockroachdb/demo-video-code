package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#6933ff"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#d6dbe7"))

	matchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00fced"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ec3f96"))
)

func main() {
	db, err := pgxpool.New(context.Background(), "postgres://root@localhost:26257?sslmode=disable")
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer db.Close()

	p := tea.NewProgram(initialModel(db), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

type animalsLoadedMsg struct {
	animals []string
	err     error
}

type matchesLoadedMsg struct {
	matches []string
	err     error
}

type animalItem struct {
	name string
}

func (i animalItem) FilterValue() string { return i.name }
func (i animalItem) Title() string       { return i.name }
func (i animalItem) Description() string { return "" }

type customDelegate struct {
	list.DefaultDelegate
}

func (d customDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	i, ok := item.(animalItem)
	if !ok {
		return
	}

	str := i.name

	if index == m.Index() {
		str = selectedStyle.Render("| " + str)
	} else {
		str = "  " + str
	}

	fmt.Fprint(w, str)
}

type model struct {
	db             *pgxpool.Pool
	allAnimals     []string
	filteredItems  []list.Item
	list           list.Model
	textInput      textinput.Model
	matches        []string
	selectedAnimal string
	loading        bool
	err            error
	width          int
	height         int
}

func initialModel(db *pgxpool.Pool) model {
	ti := textinput.New()
	ti.Placeholder = "Type to filter animals..."
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40

	delegate := customDelegate{DefaultDelegate: list.NewDefaultDelegate()}
	l := list.New([]list.Item{}, delegate, 0, 0)

	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)

	return model{
		db:        db,
		textInput: ti,
		list:      l,
		loading:   true,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		loadAnimalsCmd(m.db),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "enter":
			if selectedItem := m.list.SelectedItem(); selectedItem != nil {
				item := selectedItem.(animalItem)
				m.selectedAnimal = item.name
				m.loading = true
				m.matches = nil
				return m, fetchMatchesCmd(m.db, item.name)
			}

		case "esc":
			m.selectedAnimal = ""
			m.matches = nil
			m.textInput.Reset()
			m.updateFilteredList()
			m.textInput.Focus()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-10)

	case animalsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.allAnimals = msg.animals
		m.updateFilteredList()
		return m, nil

	case matchesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.matches = msg.matches
		}
		return m, nil
	}

	oldValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	if m.textInput.Value() != oldValue {
		m.updateFilteredList()
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *model) updateFilteredList() {
	filter := strings.ToLower(m.textInput.Value())
	m.filteredItems = make([]list.Item, 0)

	for _, animal := range m.allAnimals {
		if filter == "" || strings.Contains(strings.ToLower(animal), filter) {
			m.filteredItems = append(m.filteredItems, animalItem{name: animal})
		}
	}

	m.list.SetItems(m.filteredItems)
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("\nError: %v\n", m.err)
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Animal Browser"))
	b.WriteString("\n\n")

	b.WriteString("" + m.textInput.View())
	b.WriteString("\n\n")

	if m.loading && len(m.allAnimals) == 0 {
		b.WriteString("Loading animals...\n")
	} else {
		b.WriteString(m.list.View())
	}

	if m.selectedAnimal != "" {
		b.WriteString("\n\n")
		b.WriteString(titleStyle.Render(fmt.Sprintf("Similar to: %s", m.selectedAnimal)))
		b.WriteString("\n")

		if m.loading {
			b.WriteString(matchStyle.Render("Loading matches..."))
		} else if len(m.matches) > 0 {
			for i, match := range m.matches {
				b.WriteString(matchStyle.Render(fmt.Sprintf("  %d. %s", i+1, match)))
				b.WriteString("\n")
			}
		} else {
			b.WriteString(matchStyle.Render("No matches found"))
		}
	}

	content := b.String()
	helpText := helpStyle.Render("enter: select • esc: clear selection • ctrl+c: quit")

	contentLines := strings.Count(content, "\n")

	if m.height > 0 {
		paddingNeeded := m.height - contentLines - 2
		if paddingNeeded > 0 {
			content += strings.Repeat("\n", paddingNeeded)
		}
	}

	return content + "\n" + helpText + "\n"
}

func loadAnimalsCmd(db *pgxpool.Pool) tea.Cmd {
	return func() tea.Msg {
		animals, err := loadAnimals(db)
		return animalsLoadedMsg{animals: animals, err: err}
	}
}

func fetchMatchesCmd(db *pgxpool.Pool, animal string) tea.Cmd {
	return func() tea.Msg {
		matches, err := fetchMatches(db, animal)
		return matchesLoadedMsg{matches: matches, err: err}
	}
}

func loadAnimals(db *pgxpool.Pool) ([]string, error) {
	const stmt = `SELECT name FROM animal ORDER BY name`

	rows, err := db.Query(context.Background(), stmt)
	if err != nil {
		return nil, fmt.Errorf("making query: %w", err)
	}
	defer rows.Close()

	var animals []string
	var a string
	for rows.Next() {
		if err = rows.Scan(&a); err != nil {
			return nil, fmt.Errorf("scanning animal name: %w", err)
		}
		animals = append(animals, a)
	}

	return animals, nil
}

func fetchMatches(db *pgxpool.Pool, animal string) ([]string, error) {
	stmt := `SELECT
		a2.name,
		a2.vec <=> a.vec AS distance
	FROM animal AS a
	JOIN animal AS a2 ON true
	WHERE a.name = $1
	AND a2.name != a.name
	ORDER BY distance ASC
	LIMIT 5`

	rows, err := db.Query(context.Background(), stmt, animal)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []string
	for rows.Next() {
		var name string
		var score float64
		if err := rows.Scan(&name, &score); err != nil {
			return nil, err
		}
		matches = append(matches, fmt.Sprintf("%s (%.2f)", name, score))
	}
	return matches, nil
}
