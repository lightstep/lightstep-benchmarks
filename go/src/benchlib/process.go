package benchlib

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
)

type MachineInfo struct {
	CPU_ModelName string
	CPU_MHz       float64
	CPU_Cores     int

	Mem_Bytes uint64

	TCP_MaxSynBacklog uint64
}

type procFunc map[string]func(string, *MachineInfo)

var (
	processStartTimeMicros Micros

	processMachineInfo *MachineInfo

	cpuFuncs = procFunc{"processor": func(value string, mi *MachineInfo) {
		if num, err := strconv.Atoi(value); err == nil && mi.CPU_Cores <= num {
			mi.CPU_Cores = num + 1
		}
	},
		"model name": func(value string, mi *MachineInfo) {
			mi.CPU_ModelName = value
		},
		"cpu MHz": func(value string, mi *MachineInfo) {
			if num, err := strconv.ParseFloat(value, 64); err == nil {
				mi.CPU_MHz = num
			}
		}}

	memFuncs = procFunc{"MemTotal": func(value string, mi *MachineInfo) {
		if !strings.HasSuffix(value, " kB") {
			return
		}
		if kb, err := strconv.ParseUint(value[0:len(value)-3], 10, 64); err == nil {
			mi.Mem_Bytes = kb * 1024
		}
	}}

	processOnce sync.Once
)

func initProcess() {
	processStartTimeMicros = NowMicros()
	processMachineInfo = readMachineInfo()
}

func ProcessStartTimeMicros() Micros {
	processOnce.Do(initProcess)
	return processStartTimeMicros
}

func ProcessMachineInfo() *MachineInfo {
	processOnce.Do(initProcess)
	return processMachineInfo
}

func readMachineInfo() *MachineInfo {
	var mi MachineInfo
	readProcKeyValues("/proc/cpuinfo", &mi, cpuFuncs)
	readProcKeyValues("/proc/meminfo", &mi, memFuncs)
	readProcFileUint64("/proc/sys/net/ipv4/tcp_max_syn_backlog", &mi.TCP_MaxSynBacklog)
	return &mi
}

func readProcKeyValues(path string, mi *MachineInfo, pf procFunc) {
	f, err := os.Open(path)
	if err == nil {
		err = scanProcKeyValues(f, mi, pf)
	}
	if err != nil {
		Print("Could not read ", path, ": ", err)
	}
}

func scanProcKeyValues(f io.Reader, mi *MachineInfo, pf procFunc) error {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		kv := strings.SplitN(scanner.Text(), ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		if kf, ok := pf[key]; ok {
			kf(val, mi)
		}
	}
	return scanner.Err()
}

func readProcFileUint64(path string, p *uint64) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		Print("Could not read ", path, ": ", err)
	}
	if err := parseProcFileUint64(b, p); err != nil {
		Print("Could not parse in ", path, ": '", string(b), "': ", err)
	}

}

func parseProcFileUint64(b []byte, p *uint64) error {
	s := strings.TrimSpace(string(b))
	if ui, err := strconv.ParseUint(s, 10, 64); err != nil {
		return err
	} else {
		*p = ui
		return nil
	}
}
