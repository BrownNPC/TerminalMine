package main

import (
	tea "github.com/charmbracelet/bubbletea/v2"
)

type model struct{}

func (m model) Init() tea.Cmd {
	// Enable mouse tracking
	return tea.EnterAltScreen
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m model) View() string {
	return "" // don't use bubbletea's rendering
}
