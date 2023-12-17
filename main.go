package main

import (
	"fmt"
	"log"
	"os/exec"
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
	InSearchMode     bool
	ActionMenuOpen   bool
}

func main() {
	initialModel := NewModel()
	p := tea.NewProgram(&initialModel, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}

func NewModel() Model {
	return Model{
		ServicesPerPage: 15,
		IsLoading:       true,
	}
}

func (m *Model) Init() tea.Cmd {
	return loadServices
}

func loadServices() tea.Msg {
	conn, err := dbus.New()
	if err != nil {
		return errMsg{err}
	}
	defer conn.Close()

	units, err := conn.ListUnits()
	if err != nil {
		return errMsg{err}
	}

	return initMsg{units}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case initMsg:
		m.Services = msg.units
		m.FilteredServices = filterServices(msg.units, m.SearchQuery)
		m.IsLoading = false
	case errMsg:
		m.ErrorMessage = msg.Error()
		m.IsLoading = false
	case tea.KeyMsg:
		if m.InSearchMode {
			switch msg.Type {
			case tea.KeyEnter:
				m.InSearchMode = false
				m.FilteredServices = filterServices(m.Services, m.SearchQuery)
			case tea.KeyBackspace:
				if len(m.SearchQuery) > 0 {
					m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
				}
			default:
				m.SearchQuery += string(msg.Runes)
			}
			return m, nil
		}

		if m.ActionMenuOpen {
			switch msg.Type {
			case tea.KeyEnter:
				m.executeServiceAction(m.ServiceAction)
				m.ActionMenuOpen = false
				m.ServiceAction = ""
			case tea.KeyBackspace:
				if len(m.ServiceAction) > 0 {
					m.ServiceAction = m.ServiceAction[:len(m.ServiceAction)-1]
				}
			default:
				m.ServiceAction += string(msg.Runes)
			}
			return m, nil
		}

		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "up", "k":
			if m.CurrentSelected > 0 {
				m.CurrentSelected--
			}
		case "down", "j":
			if m.CurrentSelected < len(m.FilteredServices)-1 {
				m.CurrentSelected++
			}
		case "right", "l":
			if m.CurrentPage < (len(m.FilteredServices) / m.ServicesPerPage) {
				m.CurrentPage++
				m.CurrentSelected = 0
			}
		case "left", "h":
			if m.CurrentPage > 0 {
				m.CurrentPage--
				m.CurrentSelected = 0
			}
		case "enter":
			m.ActionMenuOpen = !m.ActionMenuOpen
		case "/":
			m.InSearchMode = true
			m.SearchQuery = ""
		}
	}

	return m, nil
}

func (m *Model) executeServiceAction(action string) {
	serviceName := m.FilteredServices[m.CurrentSelected].Name
	var cmd *exec.Cmd
	switch action {
	case "1", "start":
		cmd = exec.Command("systemctl", "start", serviceName)
	case "2", "stop":
		cmd = exec.Command("systemctl", "stop", serviceName)
	case "3", "restart":
		cmd = exec.Command("systemctl", "restart", serviceName)
		// Add other cases as needed
	}

	if cmd != nil {
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error performing %s on %s: %v\n", action, serviceName, err)
		} else {
			fmt.Printf("Successfully performed %s on %s\n", action, serviceName)
		}
	}
}

func (m *Model) View() string {
	var sb strings.Builder
	sb.WriteString("[Service Monitor]     [Actions]\n┌────┬─────────────────────────────────┬────────────┬──────────────────────────┐\n")

	maxNameLength := 25
	maxDescLength := 30
	for i := 0; i < m.ServicesPerPage; i++ {
		index := i + m.CurrentPage*m.ServicesPerPage
		if index >= len(m.FilteredServices) {
			break
		}
		service := m.FilteredServices[index]

		selected := " "
		if index == m.CurrentSelected {
			selected = ">"
		}

		serviceName := service.Name
		if len(serviceName) > maxNameLength {
			serviceName = serviceName[:maxNameLength-3] + "..."
		}

		serviceDesc := service.Description
		if len(serviceDesc) > maxDescLength {
			serviceDesc = serviceDesc[:maxDescLength-3] + "..."
		}

		sb.WriteString(fmt.Sprintf("│%s %2d │ %-25s │ %-8s │ %-25s │\n", selected, index+1, serviceName, getServiceState(service), serviceDesc))
	}

	sb.WriteString("└────┴─────────────────────────────────┴────────────┴──────────────────────────┘\n")
	sb.WriteString(m.renderSearchAndError())
	sb.WriteString(m.renderActionMenu())

	return sb.String()
}

func (m *Model) renderSearchAndError() string {
	searchLabel := "[Search]: "
	if m.InSearchMode {
		searchLabel = "[Typing]: "
	}
	return fmt.Sprintf("%s%s [ Page %d / %d ]\n[Error]: %s\n", searchLabel, m.SearchQuery, m.CurrentPage+1, (len(m.FilteredServices)/m.ServicesPerPage)+1, m.ErrorMessage)
}

func (m *Model) renderActionMenu() string {
	if m.ActionMenuOpen {
		return "\n[Action Menu]\n1: Start\n2: Stop\n3: Restart\n4: Reload\n5: Enable\n6: Disable\nEnter index or type action name:"
	}
	return ""
}

type initMsg struct {
	units []dbus.UnitStatus
}

type errMsg struct {
	error
}

func filterServices(services []dbus.UnitStatus, query string) []dbus.UnitStatus {
	if query == "" {
		return services
	}
	var filtered []dbus.UnitStatus
	for _, service := range services {
		if strings.Contains(strings.ToLower(service.Name), strings.ToLower(query)) {
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
