package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	colly "github.com/gocolly/colly"
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
			getBook(m.textInput.Value(), &m)
			return m, nil

		case "ctrl+d":
			selectedRow := m.table.SelectedRow()
			if len(selectedRow) >= 5 {
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

		case "h", "j", "k", "l":
			// do nothing to prevent table navigation
			return m, nil
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

func strip(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') ||
			b == ' ' {
			result.WriteByte(b)
		}
	}
	return result.String()
}

func getBook(query string, m *model) {
	c := colly.NewCollector(
		colly.AllowedDomains("libgen.is"),
	)

	var books []Book

	c.OnHTML("tr", func(e *colly.HTMLElement) {
		book := Book{}

		e.ForEach("td", func(index int, el *colly.HTMLElement) {
			switch index {
			case 0:
				book.ID = el.Text
			case 1:
				book.Author = el.Text
			case 8:
				book.Filetype = el.Text
			case 2:
				book.Title = el.ChildText("a")
				md5 := strings.Split(el.ChildAttr("a", "href"), "md5=")
				if len(md5) == 2 {
					book.MD5 = strings.Split(el.ChildAttr("a", "href"), "md5=")[1]
				} else {
					book.MD5 = ""
				}

			}
		})

		if book.Title != "" {
			books = append(books, book)
		}
	})

	query = url.QueryEscape(query)
	var targetPage string = "https://libgen.is/search.php?req=" + query + "&lg_topic=libgen&open=0&view=simple&res=25&phrase=1&column=def"

	err := c.Visit(targetPage)
	if err != nil {
		log.Println("Failed to visit target page:", err)
		return
	}

	p := tea.NewProgram(bookSearchModel(books))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func generateDownloadLink(md5 string, bookID string, bookTitle string, bookFiletype string) string {
	var newBookID string
	if len(bookID) == 4 {
		newBookID = string(bookID[:1]) + "000"
	} else if len(bookID) == 5 {
		newBookID = string(bookID[:2]) + "000"
	}

	md5 = strings.ToLower(md5)
	bookTitle = strings.Replace(strip(bookTitle), " ", "_", -1)
	downloadLink := "https://download.library.lol/main/" + newBookID + "/" + md5 + "/" + bookTitle + "." + bookFiletype

	return downloadLink
}

func DownloadFile(title string, filetype string, link string) error {
	fileName := fmt.Sprintf("%s.%s", title, filetype)
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

func bookSearchModel(books []Book) model {
	fmt.Print("\033[H\033[2J")
	columns := []table.Column{
		{Title: "Authors", Width: 30},
		{Title: "Title", Width: 60},
		{Title: "Filetype", Width: 10},
		{Title: "Link", Width: 30},
		{Title: "Status", Width: 15},
	}

	rows := []table.Row{}

	for index, book := range books {
		if index < 3 {
			continue
		} else if index == 50 {
			break
		}

		book.Link = generateDownloadLink(book.MD5, book.ID, book.Title, book.Filetype)
		rows = append(rows, []string{book.Author, book.Title, book.Filetype, book.Link, ""})
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	ti := textinput.New()
	ti.Placeholder = "Query"
	ti.Focus()
	ti.CharLimit = 250
	ti.Width = 152

	m := model{ti, t, rows}

	fmt.Println("Enter to search. ESC to quit. Ctrl+D to download.")
	return m
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
		table.WithHeight(0),
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
