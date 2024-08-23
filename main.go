package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/onurhanak/libgenapi"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type Book struct {
	ID, Title, Author, Link, Filetype, MD5 string
}

type model struct {
	textInput textinput.Model
	table     table.Model
	rows      []table.Row
}

type downloadFinishedMsg struct {
	index  int
	status string
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.textInput, cmd = m.textInput.Update(msg)

		switch msg.String() {
		case "enter":
			query := libgenapi.NewQuery("default", m.textInput.Value())
			err := query.Search()
			if err != nil {
				fmt.Println("Error during search:", err)
				return m, nil
			}

			// Populate the table with the search results
			m.rows = []table.Row{}
			for _, book := range query.Results {
				m.rows = append(m.rows, table.Row{book.Author, book.Title, book.Extension, book.DownloadLink, ""})
			}
			m.table.SetRows(m.rows)

			return m, nil

		case "ctrl+d":
			selectedRow := m.table.SelectedRow()
			if len(selectedRow) >= 4 {
				// update the row to show "Downloading..." status
				rowIndex := m.table.Cursor()
				m.rows = updateRowStatus(m.rows, rowIndex, "Downloading...")
				m.table.SetRows(m.rows)

				return m, downloadFileCmd(selectedRow[1], selectedRow[2], selectedRow[3], rowIndex)
			} else {
				fmt.Println("You need to make a search first.")
			}
			return m, nil

		case "ctrl+c", "esc":
			return m, tea.Quit
		}

	case downloadFinishedMsg:
		// change the status msg
		m.rows = updateRowStatus(m.rows, msg.index, msg.status)
		m.table.SetRows(m.rows)
	}

	m.table, cmd = m.table.Update(msg)

	return m, cmd
}

// downloadFileCmd starts the file download and returns a trigger
func downloadFileCmd(title, filetype, link string, index int) tea.Cmd {
	return func() tea.Msg {
		err := DownloadFile(title, filetype, link)
		status := "Downloaded"
		if err != nil {
			status = "Failed"
		}
		return downloadFinishedMsg{index: index, status: status}
	}
}

func updateRowStatus(rows []table.Row, index int, status string) []table.Row {
	if index >= 0 && index < len(rows) {
		row := rows[index]
		row[4] = status
		rows[index] = row
	}
	return rows
}

func (m model) View() string {
	return baseStyle.Render(m.textInput.View()) + "\n" + baseStyle.Render(m.table.View()) + "\n"
}

func CleanFileName(filename string) string {
	filename = strings.ReplaceAll(filename, " ", "_")

	filename = strings.ReplaceAll(filename, "/", "_")

	reg := regexp.MustCompile(`[^a-zA-Z0-9_\.-]`)
	filename = reg.ReplaceAllString(filename, "")

	return filename
}

func DownloadFile(title string, filetype string, link string) error {
	fileName := fmt.Sprintf("%s.%s", title, filetype)
	fileName = CleanFileName(fileName)
	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	fmt.Print("\033[H\033[2J")
	columns := []table.Column{
		{Title: "Authors", Width: 30},
		{Title: "Title", Width: 60},
		{Title: "Filetype", Width: 10},
		{Title: "Link", Width: 30},
		{Title: "Status", Width: 15},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15),
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

	ti := textinput.New()
	ti.Placeholder = "Query"
	ti.Focus()
	ti.CharLimit = 250
	ti.Width = 152
	fmt.Println("Enter to search. ESC to quit. Ctrl+D to download.")
	m := model{ti, t, rows}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
