package main

import (
	"flag"
	"slices"
	"sync"
	"time"
)

type AppState struct {
	containers    []ContainerStats
	containersAth map[string]AllTimeHigh
	selectedID    string
	history       map[string]*DetailHistory
	mode          Mode
	historyLen    int
	refresh       time.Duration
	interval      time.Duration
	file          string
	container     string
	escapeBox     bool

	lock    sync.RWMutex
	message string
}

func (app *AppState) ParseCmdLine() {
	var (
		container  = flag.String("container", "", "container name/id for mode=one")
		file       = flag.String("file", "compose.yml", "compose file")
		history    = flag.Int("history", 60, "history length")
		refresh    = flag.Duration("refresh", 2*time.Second, "UI refresh interval")
		mode       = flag.String("mode", "all", "compose|all|one")
		escape_box = flag.Bool("escape-box", false, "escape distrobution box (for podman-compose or podman)")
		interval   = flag.Duration("interval", 5*time.Second, "podman report interval")
	)
	flag.Parse()

	app.container = *container
	app.file = *file
	app.historyLen = *history
	app.refresh = *refresh
	app.interval = *interval
	app.mode = Mode(*mode)
	app.escapeBox = *escape_box

	app.history = make(map[string]*DetailHistory)
	app.containersAth = make(map[string]AllTimeHigh)
	app.selectedID = ""
}

func (app *AppState) resolveSelectedId() *string {

	if app.selectedID == "" {
		for hid, history := range app.history {
			if history.Container.ID != "" {
				app.selectedID = hid
				break
			}
		}
	}

	if _, ok := app.history[app.selectedID]; ok {
		return &app.selectedID
	}
	for _, history := range app.history {
		if history.Container.Name == app.selectedID {
			return &history.Container.ID
		}
	}
	return nil
}

func (app *AppState) pushUpdate(update StatsBatch) {
	app.lock.Lock()
	defer app.lock.Unlock()

	seen_ids := []string{}

	for _, container := range update.Items {

		cID := container.ID
		if container.Name != "" {
			cID = container.Name
		}

		seen_ids = append(seen_ids, cID)

		if _, ok := app.history[cID]; !ok {
			app.history[cID] = &DetailHistory{
				CPU:       make([]float64, 0, app.historyLen),
				Mem:       make([]float64, 0, app.historyLen),
				size:      uint64(app.historyLen),
				Container: container,
			}
		}
		history := app.history[cID]
		history.PushPop(container)

		if ath, ok := app.containersAth[cID]; !ok {
			app.containersAth[cID] = AllTimeHigh{
				CPU:  container.CPUPercent,
				Mem:  container.MemPercent,
				size: uint64(app.historyLen),
				id:   container.ID,
			}
		} else {
			ath.Update(container.CPUPercent, container.MemPercent, container.MemUsage)
			app.containersAth[cID] = ath
		}
	}

	for key, value := range app.history {
		found := slices.Contains(seen_ids, key)
		if !found {
			value.PushPop(ContainerStatsFloat{
				Name:       key,
				ID:         key,
				CPUPercent: -1,
				MemUsage:   -1,
				MemPercent: -1,
			})
		}
	}

}
