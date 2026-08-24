package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/haadi-coder/filesize"
	ui "github.com/metaspartan/gotui/v5"
	"github.com/metaspartan/gotui/v5/widgets"
)

type Mode string

const (
	ModeCompose Mode = "compose"
	ModeAll     Mode = "all"
	ModeOne     Mode = "one"
)

type StatsBatch struct {
	Items []ContainerStatsFloat
	Err   error
}

type UIState struct {
	list   *widgets.List
	detail *widgets.Paragraph
	cpu    *widgets.Sparkline
	mem    *widgets.Sparkline
}

func podman(app *AppState, exit chan<- bool, update_received chan<- bool) {
	cmd := buildCommand(app)
	app.lock.Lock()
	app.message = fmt.Sprintf("Running command: %v", cmd)
	app.lock.Unlock()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fatal(err)
	}
	if err := cmd.Start(); err != nil {
		fatal(err)
	}

	updates := make(chan StatsBatch, 8)
	go decodeStream(stdout, updates, app)
	go drainStderr(stderr)

	for {
		update := <-updates
		if update.Err != nil {
			app.lock.Lock()
			app.message = fmt.Sprintf("Error decoding stats: %v", update.Err)
			app.lock.Unlock()
			break
		}
		app.lock.Lock()
		app.message = fmt.Sprintf("Received update: %v containers", len(update.Items))
		app.lock.Unlock()
		app.pushUpdate(update)
		update_received <- true
	}

	if cmd.Process.Signal(syscall.SIGTERM) != nil {
		app.lock.Lock()
		defer app.lock.Unlock()
		app.message = fmt.Sprintf("Error sending SIGTERM to podman stats process: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		app.lock.Lock()
		defer app.lock.Unlock()
		app.message = fmt.Sprintf("Error waiting for podman stats process: %v", err)
	}
	exit <- true
	update_received <- true
}

func main() {

	fmt.Printf("Setup signal handler...\n")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app := &AppState{
		history: make(map[string]*DetailHistory),
	}
	fmt.Printf("Parsing command line...\n")
	app.ParseCmdLine()

	fmt.Printf("Starting podman stats...\n")
	proc_dead := make(chan bool, 1)
	proc_update := make(chan bool, 1)
	go podman(app, proc_dead, proc_update)

	fmt.Printf("Starting UI...\n")
	exit := make(chan bool, 1)
	defer ui.Close()
	time.Sleep(1 * time.Second) // Give some time for the podman stats process to start
	go renderUI(app, exit, proc_update)

	for {
		select {
		case <-proc_dead:
			app.lock.Lock()
			app.message = "Podman stats process has exited."
			app.lock.Unlock()
			go podman(app, proc_dead, proc_update)
		case <-ctx.Done():
			app.lock.Lock()
			app.message = "Received termination signal. Exiting..."
			app.lock.Unlock()
			time.Sleep(2 * time.Second) // Give some time for the UI to update
			return
		case <-exit:
			return
		}
	}

}

func updateContainerListIndex(app *AppState, list *widgets.List, lastSelectedContainerIndex *int, userChange bool) {
	app.lock.Lock()
	defer app.lock.Unlock()

	longest_name := 0
	for _, c := range app.containers {
		cl := len(c.Name)
		if cl > int(longest_name) {
			longest_name = cl
		}
	}

	clstr := []string{}
	for idx, container := range app.containers {
		cID := container.ID
		if container.Name != "" {
			cID = container.Name
		}
		ath := app.containersAth[cID]

		clstr = append(clstr,
			fmt.Sprintf(
				"%*v [%v] %v CPU | %v MEM # ATH %3.2f%% CPU | %3.2f%% MEM",
				longest_name,
				container.Name,
				container.ID,
				container.CPUPercent,
				container.MemPercent,
				ath.CPU,
				ath.Mem,
			),
		)
		if *lastSelectedContainerIndex != list.SelectedRow {
			if idx == list.SelectedRow {
				app.selectedID = cID
				*lastSelectedContainerIndex = list.SelectedRow
			}
		} else if cID == app.selectedID {
			list.SelectedRow = idx
		}
	}
	list.Rows = clstr
}

func updateDetailData(app *AppState, cpu *widgets.Sparkline, mem *widgets.Sparkline) {
	app.lock.RLock()
	rID := app.resolveSelectedId()

	max_cpu := 100.0
	max_mem := 100.0

	if rID != nil {
		hist := app.history[*rID]
		ath := app.containersAth[*rID]

		for _, cd := range hist.CPU {
			if cd > max_cpu {
				max_cpu = cd
			}
		}
		for _, md := range hist.Mem {
			if md > max_mem {
				max_mem = md
			}
		}

		mem.Data = hist.Mem
		mem.MaxVal = max_mem
		mem.Title = fmt.Sprintf(
			"MEM Usage: %3.2f%% %s/%s | ATH: %3.2f%% %s",
			hist.Container.MemPercent,
			filesize.Format(hist.Container.MemUsage),
			filesize.Format(hist.Container.MemMax),
			ath.Mem,
			filesize.Format(ath.MemSize),
		)

		cpu.Data = hist.CPU
		cpu.MaxVal = max_cpu
		cpu.Title = fmt.Sprintf("CPU Usage: %3.2f%% Avg: %3.2f%% ATH: %3.2f%% | max_val: %3.2f", hist.Container.CPUPercent, hist.Container.AvgCPU, ath.CPU, max_cpu)

	}
	app.lock.RUnlock()
}

func renderUI(app *AppState, out chan<- bool, update chan bool) {

	// 1. Header
	title := widgets.NewParagraph()
	title.Title = "podtop"
	title.Text = "PRESS q TO QUIT"
	title.TextStyle.Fg = ui.ColorWhite
	title.BorderStyle.Fg = ui.ColorLightCyan
	title.TitleStyle = ui.NewStyle(ui.ColorLightCyan, ui.ColorClear, ui.ModifierBold)
	//title.TitleAlignment = ui.AlignCenter
	//title.TitleRight = "v1.0.0"
	title.BorderRounded = false // Variety: Non-rounded border

	msg := widgets.NewParagraph()
	msg.Title = "Message"
	msg.Text = "No messages yet."
	msg.TextStyle.Fg = ui.ColorWhite
	msg.BorderStyle.Fg = ui.ColorLightCyan
	msg.TitleStyle = ui.NewStyle(ui.ColorLightCyan, ui.ColorClear, ui.ModifierBold)
	msg.TitleAlignment = ui.AlignCenter
	msg.BorderRounded = false // Variety: Non-rounded border

	cpu := widgets.NewSparkline()
	cpu.Title = "CPU Usage"
	cpu.Data = []float64{-1.0, 0, 100.0, 0}
	cpu.LineColor = ui.ColorGreen
	cpu.TitleStyle = ui.NewStyle(ui.ColorLightCyan, ui.ColorClear, ui.ModifierBold)
	cpu.MaxVal = 100.0

	mem := widgets.NewSparkline()
	mem.Title = "Memory Usage"
	mem.Data = []float64{-1.0, 0, 100.0, 0}
	mem.LineColor = ui.ColorMagenta
	mem.TitleStyle = ui.NewStyle(ui.ColorLightCyan, ui.ColorClear, ui.ModifierBold)
	mem.MaxVal = 100.0

	slg := widgets.NewSparklineGroup(cpu, mem)
	slg.Title = "Ressoure Monitor"
	slg.TitleStyle.Fg = ui.ColorGreen
	slg.BorderStyle.Fg = ui.ColorGreen
	slg.TitleRight = "CPU & MEM" // Right title demo
	slg.BorderRounded = true

	containers := widgets.NewList()
	containers.Title = "Running containers"
	containers.Rows = []string{}
	containers.TextStyle.Fg = ui.ColorYellow
	containers.SelectedStyle = ui.NewStyle(ui.ColorBlack, ui.ColorGreen)
	containers.TitleStyle.Fg = ui.ColorYellow
	containers.BorderStyle.Fg = ui.ColorYellow
	containers.BorderRounded = true

	rootFlex := widgets.NewFlex()

	rootFlex.Direction = widgets.FlexColumn   // Vertical layout
	rootFlex.AddItem(title, 3, 0, false)      // Fixed 3 height
	rootFlex.AddItem(slg, 0, 1, false)        // 100% remaining height
	rootFlex.AddItem(containers, 6, 0, false) // 100% remaining height
	rootFlex.AddItem(msg, 3, 0, false)        // Fixed 1 height

	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize gotui: %v", err)
	}
	defer func() {
		out <- true
	}()

	termWidth, termHeight := ui.TerminalDimensions()
	rootFlex.SetRect(0, 0, termWidth, termHeight)

	// Update Function
	tickerCount := 0
	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(250 * time.Millisecond) // 10 FPS updates
	defer ticker.Stop()

	lastSelectedContainerIndex := -1
	userChange := false

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "<Up>":
				fallthrough
			case "<MouseWheelUp>":
				containers.ScrollUp()
				userChange = true
				ui.Render(rootFlex)
			case "<Down>":
				fallthrough
			case "<MouseWheelDown>":
				containers.ScrollDown()
				userChange = true
				ui.Render(rootFlex)
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				rootFlex.SetRect(0, 0, payload.Width, payload.Height)
				ui.Clear()
				ui.Render(rootFlex)
			}
		case <-update:
			updateDetailData(app, cpu, mem)
			updateContainerListIndex(app, containers, &lastSelectedContainerIndex, userChange)

		case <-ticker.C:
			tickerCount++

			if lastSelectedContainerIndex != containers.SelectedRow {
				updateContainerListIndex(app, containers, &lastSelectedContainerIndex, false)
				updateDetailData(app, cpu, mem)
			}
			app.lock.RLock()
			msg.Text = app.message

			app.lock.RUnlock()
			ui.Render(rootFlex)
		}
	}
}

func buildCommand(app *AppState) *exec.Cmd {
	app.lock.Lock()
	defer app.lock.Unlock()

	intv := app.interval.Seconds()

	switch app.mode {
	case ModeCompose:
		if app.escapeBox {
			return exec.Command("host-spawn", "podman-compose", "-f", app.file, "stats", "--format", "json", "--interval", fmt.Sprintf("%g", intv))
		} else {
			return exec.Command("podman-compose", "-f", app.file, "stats", "--format", "json", "--interval", fmt.Sprintf("%g", intv))
		}
	case ModeOne:
		if app.container == "" {
			app.container = "my-container"
		}
		if app.escapeBox {
			return exec.Command("host-spawn", "podman", "stats", "--format", "json", app.container, "--interval", fmt.Sprintf("%g", intv))
		}
		return exec.Command("podman", "stats", "--format", "json", app.container, "--interval", fmt.Sprintf("%g", intv))
	default:
		if app.escapeBox {
			return exec.Command("host-spawn", "podman", "stats", "--all", "--format", "json", "--interval", fmt.Sprintf("%g", intv))
		} else {
			return exec.Command("podman", "stats", "--all", "--format", "json", "--interval", fmt.Sprintf("%g", intv))
		}
	}
}

func decodeStream(r io.Reader, out chan<- StatsBatch, App *AppState) {
	defer close(out)
	dec := json.NewDecoder(r)

	for {
		var batch []ContainerStats
		if err := dec.Decode(&batch); err != nil {
			App.lock.Lock()
			App.message = fmt.Sprintf("Error decoding JSON: %v", err)
			App.lock.Unlock()
			out <- StatsBatch{Err: err}
			continue
		}
		App.lock.Lock()
		App.containers = batch
		App.lock.Unlock()
		floatBatch, err := BathchToFloat(batch)
		if err != nil {
			App.lock.Lock()
			App.message = fmt.Sprintf("Error converting stats to float: %v", err)
			App.lock.Unlock()
			out <- StatsBatch{Err: err}
			continue
		}
		out <- StatsBatch{Items: *floatBatch}
	}
}

func drainStderr(r io.Reader) {
	_, _ = io.Copy(io.Discard, r)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
