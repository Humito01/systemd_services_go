package main

import (
	"fmt"
	"log"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coreos/go-systemd/v22/dbus"
)

type Model struct {
	Services         []dbus.UnitStatus
	FilteredServices []dbus.UnitStatus
	CurrentPage      int
	CurrentSelected  int
	SearchQuery      string
	ServiceAction    string
	ErrorMessage     string
	IsLoading        bool
	DropdownOpen     bool
	HighlightedItem  int
	ServicesPerPage  int
}

func main() {
	initialModel := NewModel()
	p := tea.NewProgram(initialModel)
	if err := p.Start(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}

func NewModel() Model {
	return Model{
		ServicesPerPage: 10,
	}
}

func (m Model) Init() tea.Cmd {
	conn, err := dbus.New()
	if err != nil {
		m.ErrorMessage = err.Error()
		return nil
	}
	defer conn.Close()

	units, err := conn.ListUnits()
	if err != nil {
		m.ErrorMessage = err.Error()
		return nil
	}

	m.Services = units
	m.FilteredServices = units
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up":
			if m.CurrentSelected > 0 {
				m.CurrentSelected--
			}
		case "down":
			if m.CurrentSelected < len(m.FilteredServices)-1 {
				m.CurrentSelected++
			}
		case "enter":
			if m.CurrentSelected >= 0 && m.CurrentSelected < len(m.FilteredServices) {
				m.IsLoading = true
				serviceName := m.FilteredServices[m.CurrentSelected].Name
				switch m.ServiceAction {
				case "start":
					go func() {
						err := startService(serviceName)
						if err != nil {
							m.ErrorMessage = err.Error()
						} else {
							m.ErrorMessage = ""
						}
						m.IsLoading = false
					}()
				case "stop":
					go func() {
						err := stopService(serviceName)
						if err != nil {
							m.ErrorMessage = err.Error()
						} else {
							m.ErrorMessage = ""
						}
						m.IsLoading = false
					}()
				case "restart":
					go func() {
						err := restartService(serviceName)
						if err != nil {
							m.ErrorMessage = err.Error()
						} else {
							m.ErrorMessage = ""
						}
						m.IsLoading = false
					}()
				}
			}
		case "s":
			m.SearchQuery = ""
			m.DropdownOpen = true
		}
	}

	// Additional code for using restartAction
	restartAction := ""
	if m.ServiceAction == "restart" {
		restartAction = " [ Restart]"
	}

	m.FilteredServices = filterServices(m.Services, m.SearchQuery)

	return m, nil
}

func (m Model) View() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[Service Monitor]     [Actions]\n┌───────────────────┐  ┌────────────┐\n"))

	startAction := ""
	stopAction := ""
	restartAction := ""
	if m.ServiceAction == "start" {
		startAction = " [ Start ]"
	} else if m.ServiceAction == "stop" {
		stopAction = " [ Stop  ]"
	} else if m.ServiceAction == "restart" {
		restartAction = " [ Restart]"
	}

	for i := 0; i < m.ServicesPerPage; i++ {
		index := i + m.CurrentPage*m.ServicesPerPage
		if index >= len(m.FilteredServices) {
			break
		}
		service := m.FilteredServices[index]
		selected := ""
		if index == m.CurrentSelected {
			selected = ">"
		}
		sb.WriteString(fmt.Sprintf("│ %s %-30s │  │  %s%s%s │\n", selected, service.Name, getServiceState(service), startAction, stopAction, restartAction))

	}

	sb.WriteString(fmt.Sprintf("│                   │  └────────────┘\n[Search]: %s [ Page %d / %d ]\n[Error]: %s\n", m.SearchQuery, m.CurrentPage+1, len(m.FilteredServices)/m.ServicesPerPage+1, m.ErrorMessage))
	return sb.String()
}

func startService(serviceName string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.StartUnit(serviceName, "replace", nil)
	return err
}

func stopService(serviceName string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.StopUnit(serviceName, "replace", nil)
	return err
}

func restartService(serviceName string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.RestartUnit(serviceName, "replace", nil)
	return err
}

func filterServices(services []dbus.UnitStatus, query string) []dbus.UnitStatus {
	if query == "" {
		return services
	}

	var filtered []dbus.UnitStatus
	for _, service := range services {
		if strings.Contains(service.Name, query) {
			filtered = append(filtered, service)
		}
	}
	return filtered
}

func getServiceState(service dbus.UnitStatus) string {
	state := "Unknown"
	switch service.ActiveState {
	case "active":
		state = "Running"
	case "reloading":
		state = "Reloading"
	case "inactive":
		state = "Stopped"
	}
	return state
}
