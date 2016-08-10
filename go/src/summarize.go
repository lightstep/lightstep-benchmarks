package main

import (
	"benchlib"
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

	"github.com/GaryBoone/GoStats/stats"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

const (
	testStorageBucket = "lightstep-client-benchmarks"

	numTranches = 3
)

var (
	testName = flag.String("test", "", "Name of the test")

	tranchNames = []string{
		"high load",
		"med load",
		"low load",
	}

	// These should be the same size as numTranches
	tracedColors = []string{
		"#ff0000",
		"#ff8000",
		"#ffff00",
	}
	untracedColors = []string{
		"#888888",
		"#777777",
		"#666666",
	}
)

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

type ByLoad []*benchlib.DataPoint

func (a ByLoad) Len() int           { return len(a) }
func (a ByLoad) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByLoad) Less(i, j int) bool { return a[i].WorkRatio > a[j].WorkRatio }

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
	output := benchlib.Output{}
	if err := json.Unmarshal(data, &output); err != nil {
		return err
	}
	var umeasure ByLoad
	var tmeasure ByLoad

	for _, p := range output.Results {
		p := p
		if p.Completion < 0.95 {
			continue
		}
		// Note: current experiment doesn't have the input load setting
		// so we'll use the effective load factor which underestimates
		// the input load.
		umeasure = append(umeasure, &p.Untraced)
		tmeasure = append(tmeasure, &p.Traced)
	}
	sort.Sort(umeasure)
	sort.Sort(tmeasure)

	dir := fmt.Sprintf("./%s-%s-%s", output.Title, output.Client, output.Name)
	if err := os.Mkdir(dir, 0755); err != nil {
		glog.Fatal("Could not mkdir: ", dir)
	}

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

	perTranch := len(umeasure) / numTranches

	plotCmds := []string{}
	lineCmds := []string{}

	for i := numTranches - 1; i >= 0; i -= 1 {
		tx := make([]float64, perTranch)
		ty := make([]float64, perTranch)
		ux := make([]float64, perTranch)
		uy := make([]float64, perTranch)

		var buffer bytes.Buffer

		for j := 0; j < perTranch; j++ {
			tm := tmeasure[i*perTranch+j]
			um := umeasure[i*perTranch+j]

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

		tranchCsv := fmt.Sprintf("tranch%d.csv", i)
		err := ioutil.WriteFile(path.Join(dir, tranchCsv), buffer.Bytes(), 0755)
		if err != nil {
			glog.Fatal("Could not write file: ", err)
		}
		tslope, tinter, _, _, _, _ := stats.LinearRegression(tx, ty)
		uslope, uinter, _, _, _, _ := stats.LinearRegression(ux, uy)

		script.WriteString(fmt.Sprintf("t%d(x)=%f*x+%f\n", i, tslope, tinter))
		script.WriteString(fmt.Sprintf("u%d(x)=%f*x+%f\n", i, uslope, uinter))

		plotCmds = append(plotCmds, fmt.Sprintf("'%s' using 1:3 title 'untraced - %s' with point lc rgb '%s'",
			tranchCsv, tranchNames[i], untracedColors[i]))
		plotCmds = append(plotCmds, fmt.Sprintf("'%s' using 4:6 title 'traced - %s' with point lc rgb '%s'",
			tranchCsv, tranchNames[i], tracedColors[i]))

		lineCmds = append(lineCmds, fmt.Sprintf("u%d(x) title 'untraced - %s' with line lc rgb '%s'",
			i, tranchNames[i], untracedColors[i]))
		lineCmds = append(lineCmds, fmt.Sprintf("t%d(x) title 'traced - %s' with line lc rgb '%s'",
			i, tranchNames[i], tracedColors[i]))
	}
	plotCmds = append(plotCmds, lineCmds...)

	script.WriteString("plot ")
	script.WriteString(strings.Join(plotCmds, ","))
	script.WriteString("\nquit\n")

	ioutil.WriteFile(path.Join(dir, "script.gnuplot"), script.Bytes(), 0755)

	return nil
}
