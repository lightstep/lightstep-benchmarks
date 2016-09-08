package main

import (
	bench "benchlib"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/GaryBoone/GoStats/stats"
	"github.com/golang/glog"
	hstats "github.com/hermanschaaf/stats"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

const (
	testStorageBucket = "lightstep-client-benchmarks"
)

var (
	testName = flag.String("test", "", "Name of the test")

	// tranchNames = []string{
	// 	"high load",
	// 	"med load",
	// 	"low load",
	// }

	// // These should be the same size as numTranches
	// tracedColors = []string{
	// 	"#ff0000",
	// 	"#ff8000",
	// 	"#ffff00",
	// }
	// untracedColors = []string{
	// 	"#888888",
	// 	"#777777",
	// 	"#666666",
	// }
)

func tranchName(l float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", l), ".", "_", -1)
}

func tracedColor(l float64) string {
	return fmt.Sprintf("#ff%02x00", int(255*(1-l)))
}

func untracedColor(l float64) string {
	return fmt.Sprintf("#%02xff00", int(255*(1-l)))
}

func usage() {
	fmt.Printf("usage: %s --test=<...>\n", os.Args[0])
	os.Exit(1)
}

type summarizer struct {
}

func main() {
	flag.Parse()

	if *testName == "" {
		usage()
	}

	ctx := context.Background()
	gcpClient, err := google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		glog.Fatal("GCP Default client: ", err)
	}
	storageClient, err := storage.NewClient(ctx, cloud.WithBaseHTTP(gcpClient))
	if err != nil {
		log.Fatal("GCP Storage client", err)
	}
	defer storageClient.Close()
	bucket := storageClient.Bucket(testStorageBucket)

	olist, err := bucket.List(ctx, nil)
	if err != nil {
		log.Fatal("GCP Storage client", err)
	}
	if olist.Next != nil {
		log.Fatal("GCP unhandled Next result field: ", olist)
	}
	s := summarizer{}
	prefix := *testName + "/"
	for _, obj := range olist.Results {
		if !strings.HasPrefix(obj.Name, prefix) {
			continue
		}
		if err := s.getResults(ctx, bucket, obj.Name); err != nil {
			log.Fatal("Couldn't read results: ", obj.Name)
		}
	}

}

type outputDir struct {
	*bench.Output
	dir string
}

func newOutputDir(output *bench.Output) outputDir {
	dir := fmt.Sprintf("./%s-%s-%s", output.Title, output.Client, output.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		glog.Fatal("Could not mkdir: ", dir)
	}
	return outputDir{output, dir}
}

// type ByLoad []*bench.DataPoint

// func (a ByLoad) Len() int           { return len(a) }
// func (a ByLoad) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
// func (a ByLoad) Less(i, j int) bool { return a[i].WorkRatio > a[j].WorkRatio }

func (s *summarizer) getResults(ctx context.Context, b *storage.BucketHandle, name string) error {
	oh := b.Object(name)
	reader, err := oh.NewReader(ctx)
	if err != nil {
		return err
	}
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	output := bench.Output{}
	if err := json.Unmarshal(data, &output); err != nil {
		return err
	}

	if err := s.getSleepCalibration(&output); err != nil {
		return err
	}

	return s.getMeasurements(&output)
}

func (s *summarizer) getSleepCalibration(output *bench.Output) error {
	fmt.Println("Sleep calibration for", output.Title, output.Client, output.Name)
	factorMap := map[int][]bench.SleepCalibration{}

	for _, s := range output.Sleeps {
		s := s
		factorMap[s.WorkFactor] = append(factorMap[s.WorkFactor], s)
	}

	var workVals []int
	for w, _ := range factorMap {
		workVals = append(workVals, w)
	}
	sort.Ints(workVals)

	for _, w := range workVals {
		sm := factorMap[w]

		var ras, rns, as []int64
		for _, s := range sm {
			ras = append(ras, int64(s.RunAndSleep*1e9))
			rns = append(ras, int64(s.RunNoSleep*1e9))
			as = append(as, int64(s.ActualSleep*1e9))

			// runtimeDiff := (s.RunAndSleep - s.RunNoSleep) / float64(sm[0].Repeats)
			// diff := math.Abs(runtimeDiff-bench.DefaultSleepInterval.Seconds()) / bench.DefaultSleepInterval.Seconds()
			// if diff <= 0 || diff >= 1 {
			// 	glog.Info("Skipping invalid sleep time: ", s, "time", runtimeDiff, "diff", diff)
			// 	continue
			// }
		}
		glog.Infof("Sleep cost @ %d work factor = ...", w)

		dur := func(ns float64) time.Duration {
			return time.Duration(int64(ns))
		}

		rasLow, rasHigh := hstats.NormalConfidenceInterval(ras)
		glog.Infof("RAS %v %v %v %v)", dur(hstats.Mean(ras)), dur(hstats.StandardDeviation(ras)), dur(rasLow), dur(rasHigh))

		rnsLow, rnsHigh := hstats.NormalConfidenceInterval(rns)
		glog.Infof("RNS %v %v %v %v", dur(hstats.Mean(rns)), dur(hstats.StandardDeviation(rns)), dur(rnsLow), dur(rnsHigh))

		asLow, asHigh := hstats.NormalConfidenceInterval(as)
		glog.Infof("AS %v %v %v %v", dur(hstats.Mean(as)), dur(hstats.StandardDeviation(as)), dur(asLow), dur(asHigh))
	}

	return nil
}

func (s *summarizer) getMeasurements(output *bench.Output) error {
	loadMap := map[float64][]bench.Measurement{}

	for _, p := range output.Results {
		p := p
		if p.Completion < 0.95 {
			continue
		}
		loadMap[p.TargetLoad] = append(loadMap[p.TargetLoad], p)
	}

	odir := newOutputDir(output)

	loadVals := []float64{}
	for l, _ := range loadMap {
		loadVals = append(loadVals, l)
	}
	sort.Float64s(loadVals)

	var script bytes.Buffer

	script.WriteString(`
set terminal png size 1000,1000
set output "scatter.png"
set datafile separator ","
set origin 0,0

set title "Tracing Cost"
set xlabel "Request Rate"
set ylabel "Visible CPU Impairment"
set style func points
`)

	plotCmds := []string{}
	lineCmds := []string{}

	for _, l := range loadVals {
		measurements := loadMap[l]
		count := len(measurements)

		tx := make([]float64, count)
		ty := make([]float64, count)
		ux := make([]float64, count)
		uy := make([]float64, count)

		var buffer bytes.Buffer

		for _, m := range measurements {
			tm := m.Traced
			um := m.Untraced

			tx = append(tx, tm.RequestRate)
			ux = append(ux, um.RequestRate)

			ty = append(ty, tm.VisibleImpairment())
			uy = append(uy, um.VisibleImpairment())

			buffer.Write([]byte(fmt.Sprintf("%.2f,%.5f,%.5f,%.2f,%.5f,%.5f\n",
				um.RequestRate,
				um.WorkRatio,
				1-um.WorkRatio-um.SleepRatio,
				tm.RequestRate,
				tm.WorkRatio,
				1-tm.WorkRatio-tm.SleepRatio)))
		}

		lstr := tranchName(l)

		tranchCsv := fmt.Sprintf("tranch%s.csv", lstr)
		err := ioutil.WriteFile(path.Join(odir.dir, tranchCsv), buffer.Bytes(), 0755)
		if err != nil {
			glog.Fatal("Could not write file: ", err)
		}
		tslope, tinter, _, _, _, _ := stats.LinearRegression(tx, ty)
		uslope, uinter, _, _, _, _ := stats.LinearRegression(ux, uy)

		script.WriteString(fmt.Sprintf("t%s(x)=%f*x+%f\n", lstr, tslope, tinter))
		script.WriteString(fmt.Sprintf("u%s(x)=%f*x+%f\n", lstr, uslope, uinter))

		plotCmds = append(plotCmds, fmt.Sprintf("'%s' using 1:3 title 'untraced - %s' with point lc rgb '%s'",
			tranchCsv, lstr, untracedColor(l)))
		plotCmds = append(plotCmds, fmt.Sprintf("'%s' using 4:6 title 'traced - %s' with point lc rgb '%s'",
			tranchCsv, lstr, tracedColor(l)))

		lineCmds = append(lineCmds, fmt.Sprintf("u%s(x) title 'untraced - %s' with line lc rgb '%s'",
			lstr, lstr, untracedColor(l)))
		lineCmds = append(lineCmds, fmt.Sprintf("t%s(x) title 'traced - %s' with line lc rgb '%s'",
			lstr, lstr, tracedColor(l)))
	}
	plotCmds = append(plotCmds, lineCmds...)

	script.WriteString("plot ")
	script.WriteString(strings.Join(plotCmds, ","))
	script.WriteString("\nquit\n")

	ioutil.WriteFile(path.Join(odir.dir, "script.gnuplot"), script.Bytes(), 0755)

	// TODO call gnuplot

	return nil
}
