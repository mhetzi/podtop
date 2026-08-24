package main

type DetailHistory struct {
	CPU       []float64
	Mem       []float64
	size      uint64
	Container ContainerStatsFloat
}

func (dh *DetailHistory) PushPop(container ContainerStatsFloat) {
	if len(dh.CPU) > int(dh.size) {
		dh.CPU = append(dh.CPU[1:], container.CPUPercent)
	} else {
		dh.CPU = append(dh.CPU, container.CPUPercent)
	}
	if len(dh.Mem) > int(dh.size) {
		dh.Mem = append(dh.Mem[1:], container.MemPercent)
	} else {
		dh.Mem = append(dh.Mem, container.MemPercent)
	}
	dh.Container = container
}

func NewDetailHistory(size int) *DetailHistory {
	return &DetailHistory{
		CPU: make([]float64, size),
		Mem: make([]float64, size),
	}
}
