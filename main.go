package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const topStoriesURL = "https://hacker-news.firebaseio.com/v0/topstories.json?print=pretty&limitToFirst=30&orderBy=%22$key%22"

var (
	headerHeight = 3
	footerHeight = 3

	headerStyle    = lipgloss.NewStyle().Bold(true).PaddingLeft(2).PaddingTop(2)
	loadingStyle   = lipgloss.NewStyle().PaddingLeft(2).PaddingTop(2).Foreground(lipgloss.Color("5"))
	pagerHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#5C5C5C"))
)

type Model struct {
	stories   []story
	selected  string
	storyText string

	loading       bool
	viewportReady bool
	err           error

	list     list.Model
	spinner  spinner.Model
	viewport viewport.Model
}

type errMsg struct {
	err error
}

type storiesMsg []story

type storyBytesMsg []byte

type story struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Score       int    `json:"score"`
	Url         string `json:"url"`
	By          string `json:"by"`
	Descendants int    `json:"descendants"`
	Kids        []int  `json:"kids"`
	Time        int    `json:"time"`
	StoryType   string `json:"type"`
}

func (s story) FilterValue() string {
	return s.Title
}

func (e errMsg) Error() string {
	return e.err.Error()
}

func getTopStories() tea.Msg {
	c := http.Client{Timeout: time.Second * 10}
	r, err := c.Get(topStoriesURL)

	if err != nil {
		return errMsg{err}
	}
	defer r.Body.Close()

	var stories []story
	var storyIds []int

	json.NewDecoder(r.Body).Decode(&storyIds)

	for _, id := range storyIds {
		var s story

		storyURL := strings.Join([]string{"https://hacker-news.firebaseio.com/v0/item/", strconv.Itoa(id), ".json"}, "")

		r, err := c.Get(storyURL)
		if err != nil {
			return errMsg{err}
		}

		json.NewDecoder(r.Body).Decode(&s)

		stories = append(stories, s)
	}

	return storiesMsg(stories)
}

func (m Model) fetchStory() tea.Cmd {
	return func() tea.Msg {
		res, _ := http.Get(m.selected)
		bytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			m.err = err
		}

		return storyBytesMsg(bytes)
	}
}

func (m Model) Init() tea.Cmd {
	return getTopStories
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		verticalMargins := headerHeight + footerHeight

		if !m.viewportReady {
			m.viewport = viewport.Model{Width: msg.Width, Height: msg.Height - verticalMargins}
			m.viewportReady = true
			m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargins
		}

	case storiesMsg:
		m.stories = msg
		m.loading = false

		for i, story := range m.stories {
			m.list.InsertItem(i, story)
		}

		return m, nil

	case storyBytesMsg:
		m.selected = ""

		converter := md.NewConverter("", true, nil)

		markdown, err := converter.ConvertString(string(msg))
		if err != nil {
			m.err = err
			return m, tea.Quit
		}

		m.storyText = markdown
		if err != nil {
			m.err = err
			return m, tea.Quit
		}

		md, _ := glamour.Render(m.storyText, "dark")
		m.viewport.GotoTop()
		m.viewport.SetContent(md)

		return m, cmd

	case errMsg:
		m.err = msg
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			// Ignore keypress if actively filtering
			if m.list.FilterState() == list.Filtering {
				break
			}

			if m.storyText != "" {
				m.storyText = ""
				m.list.ResetSelected()
				return m, nil
			}

			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(story)
			if ok {
				m.selected = string(i.Url)
			}

			cmd = m.fetchStory()
			return m, cmd
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	cmds = append(cmds, spinner.Tick)

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.err != nil {
		return m.err.Error()
	}

	if m.selected != "" {
		return listStyle.Render(m.list.View())
	}

	if m.storyText != "" {
		footerTop := "╭──────╮"
		footerMid := fmt.Sprintf("┤ %3.f%% │", m.viewport.ScrollPercent()*100)
		footerBot := "╰──────╯"
		footerHelp := pagerHelpStyle.Render("Press q to return to list")
		gapSize := m.viewport.Width - runewidth.StringWidth(footerMid)
		footerTop = strings.Repeat(" ", gapSize) + footerTop
		footerMid = strings.Repeat("─", gapSize) + footerMid
		footerBot = strings.Repeat(" ", gapSize) + footerBot

		footer := fmt.Sprintf("%s\n%s\n%s\n%s", footerTop, footerMid, footerBot, footerHelp)

		return fmt.Sprintf("%s\n%s", m.viewport.View(), footer)
	}

	if m.loading {
		header := headerStyle.Render("Welcome to Gazette!")
		loadingPrompt := fmt.Sprintf(loadingStyle.Render("%s %s"), m.spinner.View(), "Fetching stories...")

		return fmt.Sprintf("%s \n %s", header, loadingPrompt)
	}

	if len(m.stories) > 0 {
		return listStyle.Render(m.list.View())
	}

	// If there's an error, print it out and don't do anything else.
	if m.err != nil {
		return fmt.Sprintf("\nWe had some trouble: %v\n\n", m.err)
	}

	return ""
}

func main() {
	stories := []story{}

	listItems := []list.Item{}
	l := list.NewModel(listItems, itemDelegate{}, listWidth, listHeight)
	l.Title = "Top Stories"

	initialModel := Model{
		stories:   stories,
		selected:  "",
		storyText: "",

		loading:       true,
		viewportReady: false,
		err:           nil,

		spinner: spinner.NewModel(),
		list:    l,
	}

	initialModel.spinner.Spinner = spinner.Dot

	p := tea.NewProgram(initialModel, tea.WithAltScreen())

	err := p.Start()

	if err != nil {
		fmt.Printf("An unexpected error occurred: %v\n", err)
		os.Exit(1)
	}
}
