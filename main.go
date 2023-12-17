package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coreos/go-systemd/v22/dbus"
	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
)

var currentSelected int
var currentPage int

const servicesPerPage = 10

var services []dbus.UnitStatus
var filteredServices []dbus.UnitStatus
var searchQuery string

func main() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	var err error
	services, err = listServices()
	if err != nil {
		log.Fatalf("failed to list services: %v", err)
	}
	filteredServices = services

	serviceTable := widgets.NewTable()
	serviceTable.Rows = [][]string{{"Name", "Load", "Active", "Sub", "Description"}}
	updateUI(serviceTable, filteredServices)

	serviceTable.TextStyle = ui.NewStyle(ui.ColorWhite)
	serviceTable.RowSeparator = true
	serviceTable.BorderStyle = ui.NewStyle(ui.ColorGreen)

	grid := ui.NewGrid()
	grid.SetRect(0, 0, 100, 30)
	grid.Set(ui.NewRow(1.0, serviceTable))

	ui.Render(grid)

	for e := range ui.PollEvents() {
		switch e.ID {
		case "<Down>":
			maxIndex := min((currentPage+1)*servicesPerPage, len(filteredServices)) - currentPage*servicesPerPage - 1
			currentSelected = (currentSelected + 1) % (maxIndex + 1)
		case "<Up>":
			maxIndex := min((currentPage+1)*servicesPerPage, len(filteredServices)) - currentPage*servicesPerPage - 1
			currentSelected = (currentSelected - 1 + maxIndex + 1) % (maxIndex + 1)
		case "<Right>", "l":
			if (currentPage+1)*servicesPerPage < len(filteredServices) {
				currentPage++
				currentSelected = 0
			}
		case "<Left>", "h":
			if currentPage > 0 {
				currentPage--
				currentSelected = 0
			}
		case "/":
			searchQuery = prompt("Enter search query: ")
			filterServices(serviceTable)
		case "<Enter>":
			if len(filteredServices) > 0 {
				serviceIndex := currentPage*servicesPerPage + currentSelected
				if serviceIndex < len(filteredServices) {
					performAction(serviceTable, filteredServices[serviceIndex])
				}
			}
		case "q", "<C-c>":
			return
		}

		updateUI(serviceTable, filteredServices)
	}
}

func listServices() ([]dbus.UnitStatus, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	units, err := conn.ListUnits()
	if err != nil {
		return nil, err
	}

	return units, nil
}

func updateUI(serviceTable *widgets.Table, services []dbus.UnitStatus) {
	start := currentPage * servicesPerPage
	end := start + servicesPerPage
	if end > len(services) {
		end = len(services)
	}

	visibleServices := services[start:end]

	serviceTable.Rows = [][]string{{"Name", "Load", "Active", "Sub", "Description"}}
	for i, service := range visibleServices {
		row := []string{service.Name, service.LoadState, service.ActiveState, service.SubState, service.Description}
		if i == currentSelected {
			serviceTable.RowStyles[start+i+1] = ui.NewStyle(ui.ColorWhite, ui.ColorBlue, ui.ModifierBold)
		} else {
			serviceTable.RowStyles[start+i+1] = ui.NewStyle(ui.ColorWhite)
		}
		serviceTable.Rows = append(serviceTable.Rows, row)
	}
	ui.Render(serviceTable)
}

func filterServices(serviceTable *widgets.Table) {
	filteredServices = []dbus.UnitStatus{}
	for _, service := range services {
		if strings.Contains(strings.ToLower(service.Name), strings.ToLower(searchQuery)) {
			filteredServices = append(filteredServices, service)
		}
	}
	currentPage = 0
	currentSelected = 0 // Reset selection on new filter
	updateUI(serviceTable, filteredServices)
}

func performAction(serviceTable *widgets.Table, service dbus.UnitStatus) {
	conn, err := dbus.New()
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	defer conn.Close()

	action := prompt("Enter action (start/stop/restart/enable/disable): ")

	var actionErr error
	switch action {
	case "start":
		_, actionErr = conn.StartUnit(service.Name, "replace", nil)
	case "stop":
		_, actionErr = conn.StopUnit(service.Name, "replace", nil)
	case "restart":
		_, actionErr = conn.RestartUnit(service.Name, "replace", nil)
	case "enable":
		_, _, actionErr = conn.EnableUnitFiles([]string{service.Name}, false, true)
	case "disable":
		_, actionErr = conn.DisableUnitFiles([]string{service.Name}, false)
	default:
		fmt.Println("Invalid action")
		return
	}

	if actionErr != nil {
		log.Printf("Error performing action on service %s: %v", service.Name, err)
	} else {
		fmt.Printf("Action '%s' performed successfully on %s\n", action, service.Name)
	}

	refreshServices(serviceTable)
}

func refreshServices(serviceTable *widgets.Table) {
	var err error
	services, err = listServices()
	if err != nil {
		log.Printf("Error refreshing services: %v", err)
		return
	}
	filterServices(serviceTable)
}

func prompt(text string) string {
	fmt.Print(text)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
