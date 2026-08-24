package main

import (
	"strconv"
	"strings"
	"time"

	"github.com/haadi-coder/filesize"
)

/*
[
 {
  "id": "41b99a848257",
  "name": "esphome",
  "cpu_time": "8.416132s",
  "cpu_percent": "0.05%",
  "avg_cpu": "0.05%",
  "mem_usage": "104.6MB / 33.46GB",
  "mem_percent": "0.31%",
  "net_io": "0B / 0B",
  "block_io": "3.965MB / 21.34MB",
  "pids": "7"
 }
]
*/

type ContainerStats struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CPUTime    string `json:"cpu_time"`
	CPUPercent string `json:"cpu_percent"`
	AvgCPU     string `json:"avg_cpu"`
	MemUsage   string `json:"mem_usage"`
	MemPercent string `json:"mem_percent"`
	NetIO      string `json:"net_io"`
	BlockIO    string `json:"block_io"`
	PIDs       string `json:"pids"`
}

type ContainerStatsFloat struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	CPUTime    time.Duration `json:"cpu_time"`
	CPUPercent float64       `json:"cpu_percent"`
	AvgCPU     float64       `json:"avg_cpu"`
	MemUsage   int64         `json:"mem_usage"`
	MemMax     int64         `json:"mem_max"`
	MemPercent float64       `json:"mem_percent"`
	NetIO      string        `json:"net_io"`
	BlockIO    string        `json:"block_io"`
	PIDs       int32         `json:"pids"`
}

func (cs *ContainerStats) ToFloat() (*ContainerStatsFloat, error) {
	cpuTime, err := time.ParseDuration(cs.CPUTime)
	if err != nil {
		return nil, err
	}
	cpuPercent, err := strconv.ParseFloat(cs.CPUPercent[:len(cs.CPUPercent)-1], 64)
	if err != nil {
		return nil, err
	}
	avgCPU, err := strconv.ParseFloat(cs.AvgCPU[:len(cs.AvgCPU)-1], 64)
	if err != nil {
		return nil, err
	}

	mems := strings.Split(cs.MemUsage, " / ")
	memUsage, err := filesize.Parse(mems[0])
	if err != nil {
		return nil, err
	}
	memMax, err := filesize.Parse(mems[1])
	if err != nil {
		return nil, err
	}

	memPercent, err := strconv.ParseFloat(cs.MemPercent[:len(cs.MemPercent)-1], 64)
	if err != nil {
		return nil, err
	}

	pids, err := strconv.ParseInt(cs.PIDs, 10, 32)
	if err != nil {
		return nil, err
	}

	return &ContainerStatsFloat{
		ID:         cs.ID,
		Name:       cs.Name,
		CPUTime:    cpuTime,
		CPUPercent: cpuPercent,
		AvgCPU:     avgCPU,
		MemUsage:   memUsage,
		MemMax:     memMax,
		MemPercent: memPercent,
		NetIO:      cs.NetIO,
		BlockIO:    cs.BlockIO,
		PIDs:       int32(pids),
	}, nil
}

func BathchToFloat(batch []ContainerStats) (*[]ContainerStatsFloat, error) {
	var floatBatch []ContainerStatsFloat
	for _, cs := range batch {
		floatCS, err := cs.ToFloat()
		if err != nil {
			return nil, err
		}
		floatBatch = append(floatBatch, *floatCS)
	}
	return &floatBatch, nil
}

type AllTimeHigh struct {
	CPU     float64
	Mem     float64
	MemSize int64
	size    uint64
	id      string
}

func (ath *AllTimeHigh) Update(cpu float64, mem float64, memSize int64) {
	if cpu > ath.CPU {
		ath.CPU = cpu
	}
	if mem > ath.Mem {
		ath.Mem = mem
	}
	if memSize > ath.MemSize {
		ath.MemSize = memSize
	}
}
