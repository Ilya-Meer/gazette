package main

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	listStyle         = lipgloss.NewStyle().PaddingTop(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1).Foreground(lipgloss.Color("170"))
)

const (
	listWidth  = 30
	listHeight = 50
)

type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 3 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(story)
	if !ok {
		fmt.Fprintf(w, "An error occurred")
		return
	}

	str := fmt.Sprintf("%s -  (%d points) \n       (%s)", i.Title, i.Score, i.Url)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s string) string {
			return selectedItemStyle.Render(s)
		}
	}

	fmt.Fprint(w, fn(str))
}
