package bench

import (
	"bytes"
	"testing"
)

const (
	sampleProcCpuInfo = `processor       : 0
vendor_id       : GenuineIntel
cpu family      : 6
model           : 79
model name      : Intel(R) Xeon(R) CPU @ 2.20GHz
stepping        : 0
microcode       : 0x1
cpu MHz         : 2200.000
cache size      : 56320 KB
physical id     : 0
siblings        : 4
core id         : 0
cpu cores       : 2
apicid          : 0
initial apicid  : 0
fpu             : yes
fpu_exception   : yes
cpuid level     : 13
wp              : yes
flags           : fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ss ht syscall nx pdpe1gb rdtscp lm constant_tsc nopl xtopology nonstop_tsc eagerfpu pni pclmulqdq ssse3 fma cx16 sse4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm abm 3dnowprefetch fsgsbase bmi1 hle avx2 smep bmi2 rtm xsaveopt
bugs            :
bogomips        : 4400.00
clflush size    : 64
cache_alignment : 64
address sizes   : 46 bits physical, 48 bits virtual
power management:

processor       : 1
vendor_id       : GenuineIntel
cpu family      : 6
model           : 79
model name      : Intel(R) Xeon(R) CPU @ 2.20GHz
stepping        : 0
microcode       : 0x1
cpu MHz         : 2200.000
cache size      : 56320 KB
physical id     : 0
siblings        : 4
core id         : 0
cpu cores       : 2
apicid          : 1
initial apicid  : 1
fpu             : yes
fpu_exception   : yes
cpuid level     : 13
wp              : yes
flags           : fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2
 ss ht syscall nx pdpe1gb rdtscp lm constant_tsc nopl xtopology nonstop_tsc eagerfpu pni pclmulqdq ssse3 fma cx16 s
se4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm abm 3dnowprefetch fsgsbase bmi1 hle a
vx2 smep bmi2 rtm xsaveopt
bugs            :
bogomips        : 4400.00
clflush size    : 64
cache_alignment : 64
address sizes   : 46 bits physical, 48 bits virtual
power management:
processor       : 2
vendor_id       : GenuineIntel
cpu family      : 6
model           : 79
model name      : Intel(R) Xeon(R) CPU @ 2.20GHz
stepping        : 0
microcode       : 0x1
cpu MHz         : 2200.000
cache size      : 56320 KB
physical id     : 0
siblings        : 4
core id         : 1
cpu cores       : 2
apicid          : 2
initial apicid  : 2
fpu             : yes
fpu_exception   : yes
cpuid level     : 13
wp              : yes
flags           : fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2
 ss ht syscall nx pdpe1gb rdtscp lm constant_tsc nopl xtopology nonstop_tsc eagerfpu pni pclmulqdq ssse3 fma cx16 s
se4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm abm 3dnowprefetch fsgsbase bmi1 hle a
vx2 smep bmi2 rtm xsaveopt
bugs            :
bogomips        : 4400.00
clflush size    : 64
cache_alignment : 64
address sizes   : 46 bits physical, 48 bits virtual
power management:
processor       : 3
vendor_id       : GenuineIntel
cpu family      : 6
model           : 79
model name      : Intel(R) Xeon(R) CPU @ 2.20GHz
stepping        : 0
microcode       : 0x1
cpu MHz         : 2200.000
cache size      : 56320 KB
physical id     : 0
siblings        : 4
core id         : 1
cpu cores       : 2
apicid          : 3
initial apicid  : 3
fpu             : yes
fpu_exception   : yes
cpuid level     : 13
wp              : yes
flags           : fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2
 ss ht syscall nx pdpe1gb rdtscp lm constant_tsc nopl xtopology nonstop_tsc eagerfpu pni pclmulqdq ssse3 fma cx16 s
se4_1 sse4_2 x2apic movbe popcnt aes xsave avx f16c rdrand hypervisor lahf_lm abm 3dnowprefetch fsgsbase bmi1 hle a
vx2 smep bmi2 rtm xsaveopt
bugs            :
bogomips        : 4400.00
clflush size    : 64
cache_alignment : 64
address sizes   : 46 bits physical, 48 bits virtual
power management:
`

	sampleProcMemInfo = `MemTotal:       15400564 kB
MemFree:        11059192 kB
MemAvailable:   14846296 kB
Buffers:          216624 kB
Cached:          3620964 kB
SwapCached:            0 kB
Active:          2772440 kB
Inactive:        1150732 kB
Active(anon):      92864 kB
Inactive(anon):    20492 kB
Active(file):    2679576 kB
Inactive(file):  1130240 kB
Unevictable:        3660 kB
Mlocked:            3660 kB
SwapTotal:             0 kB
SwapFree:              0 kB
Dirty:                96 kB
Writeback:             0 kB
AnonPages:         89268 kB
Mapped:            76164 kB
Shmem:             25352 kB
Slab:             339340 kB
SReclaimable:     311580 kB
SUnreclaim:        27760 kB
KernelStack:        3088 kB
PageTables:         3928 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:     7700280 kB
Committed_AS:     554908 kB
VmallocTotal:   34359738367 kB
VmallocUsed:           0 kB
VmallocChunk:          0 kB
HardwareCorrupted:     0 kB
AnonHugePages:     24576 kB
CmaTotal:              0 kB
CmaFree:               0 kB
HugePages_Total:       0
HugePages_Free:        0
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
DirectMap4k:      116724 kB
DirectMap2M:     5126144 kB
DirectMap1G:    12582912 kB
`
)

func TestScanProcKeyValues(t *testing.T) {
	var mi MachineInfo
	scanProcKeyValues(bytes.NewBuffer([]byte(sampleProcCpuInfo)), &mi, ":", cpuFuncs)
	scanProcKeyValues(bytes.NewBuffer([]byte(sampleProcMemInfo)), &mi, ":", memFuncs)
	parseProcFileUint64([]byte("512\n"), &mi.TCP_MaxSynBacklog)

	if mi.CPU_Cores != 4 {
		t.Error("Incorrect cores: ", mi)
	}
	if mi.CPU_MHz != 2200.0 {
		t.Error("Incorrect MHz: ", mi)
	}
	if mi.CPU_ModelName != "Intel(R) Xeon(R) CPU @ 2.20GHz" {
		t.Error("Incorrect model: ", mi)
	}
	if mi.Mem_Bytes != 15770177536 {
		t.Error("Incorrect memory bytes: ", mi)
	}
	if mi.TCP_MaxSynBacklog != 512 {
		t.Error("Incorrect max tcp syn backlog: ", mi)
	}
}
