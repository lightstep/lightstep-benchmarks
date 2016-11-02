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
	"os/exec"
	"path"
	"path/filepath"
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
		fmt.Println("Found test", obj.Name)
		if err := s.getResults(ctx, bucket, obj.Name); err != nil {
			log.Fatal("Couldn't read results: ", obj.Name)
		}
	}

}

type outputDir struct {
	*bench.Output
	dir string
	out string
}

func newOutputDir(output *bench.Output) outputDir {
	idir := fmt.Sprintf("./%s/%s-%s", output.Title, output.Client, output.Name)
	if err := os.MkdirAll(idir, 0755); err != nil {
		glog.Fatal("Could not mkdir: ", idir)
	}
	odir := fmt.Sprintf("./%s/output", output.Title)
	if err := os.MkdirAll(odir, 0755); err != nil {
		glog.Fatal("Could not mkdir: ", odir)
	}
	return outputDir{output, idir, odir}
}

func (od *outputDir) ipathFor(name string) string {
	p, _ := filepath.Abs(path.Join(od.dir, name))
	return p
}
func (od *outputDir) opathFor(name string) string {
	p, _ := filepath.Abs(path.Join(od.out, name))
	return p
}

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

type plotScript struct {
	Name string
	outputDir
	cmds []string
	bytes.Buffer
}

type multiScript struct {
	plots []*plotScript
}

func multiScripter(plots ...*plotScript) multiScript {
	return multiScript{plots: plots}
}

func newPlotScript(output *bench.Output, name string, odir outputDir) *plotScript {
	p := &plotScript{Name: name, outputDir: odir}
	p.writeHeader()
	return p
}

func (p *plotScript) writeHeader() {
	p.WriteString(`
set terminal png size 1000,1000
set output "`)
	p.WriteString(p.outputDir.opathFor(p.Client + "_" + p.Output.Name + "_" + p.Name + ".png"))
	p.WriteString(`"
set datafile separator ","
set xrange [0:*]
set yrange [0:*]

set title "Tracing Cost"
set xlabel "Request Rate"
set ylabel "Visible CPU Impairment"
set style func points
`)
}

func (s *plotScript) writeBody() {
	s.WriteString("plot ")
	s.WriteString(strings.Join(s.cmds, ","))
	s.WriteString("\n")
	s.WriteString("quit\n")

	ioutil.WriteFile(s.ipathFor(s.Name+".gnuplot"), s.Bytes(), 0755)
}

func (s *plotScript) add(cmd string) {
	s.cmds = append(s.cmds, cmd)
}

func (m multiScript) add(cmd string) {
	for _, p := range m.plots {
		p.add(cmd)
	}
}

func (m multiScript) WriteString(s string) (int, error) {
	for _, p := range m.plots {
		p.WriteString(s)
	}
	return len(s), nil
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

		var ras, rns []int64
		repeats := 0
		for _, s := range sm {
			ras = append(ras, s.RunAndSleep.User.Nanoseconds()+s.RunAndSleep.Sys.Nanoseconds())
			rns = append(ras, s.RunNoSleep.User.Nanoseconds()+s.RunNoSleep.Sys.Nanoseconds())
			repeats = s.Repeats
		}

		dur := func(ns float64) time.Duration {
			return time.Duration(int64(ns))
		}

		rasLow, rasHigh := hstats.NormalConfidenceInterval(ras)
		glog.V(1).Infof("RAS %v %v %v %v", dur(hstats.Mean(ras)), dur(hstats.StandardDeviation(ras)), dur(rasLow), dur(rasHigh))

		rnsLow, rnsHigh := hstats.NormalConfidenceInterval(rns)
		glog.V(1).Infof("RNS %v %v %v %v", dur(hstats.Mean(rns)), dur(hstats.StandardDeviation(rns)), dur(rnsLow), dur(rnsHigh))

		glog.Infof("Sleep mean difference: %v", dur((hstats.Mean(ras)-hstats.Mean(rns))/float64(repeats)))
		glog.Infof("Sleep error separated: %v", dur((rasLow-rnsHigh)/float64(repeats)))
	}

	return nil
}

func (s *summarizer) getMeasurements(output *bench.Output) error {
	loadMap := map[float64][]bench.Measurement{}
	count := 0
	for _, p := range output.Results {
		p := p
		// TODO note this is still here.
		if p.Completion < 0.95 {
			continue
		}
		loadMap[p.TargetLoad] = append(loadMap[p.TargetLoad], p)
		count++
	}
	if count == 0 {
		glog.Info("Insufficient completion")
		return nil
	}

	odir := newOutputDir(output)

	loadVals := []float64{}
	for l, _ := range loadMap {
		loadVals = append(loadVals, l)
	}
	sort.Float64s(loadVals)

	comboScript := newPlotScript(output, "all", odir)
	allScripts := []*plotScript{comboScript}

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

			buffer.Write([]byte(fmt.Sprintf("%.8f,%.8f,%.8f,%.8f,%.8f,%.8f\n",
				um.RequestRate,
				um.WorkRatio,
				um.VisibleImpairment(),
				tm.RequestRate,
				tm.WorkRatio,
				tm.VisibleImpairment())))
		}

		lstr := tranchName(l)

		tname := "load" + lstr
		loadCsv := tname + ".csv"
		err := ioutil.WriteFile(path.Join(odir.dir, loadCsv), buffer.Bytes(), 0755)
		if err != nil {
			glog.Fatal("Could not write file: ", err)
		}
		tslope, tinter, _, _, _, _ := stats.LinearRegression(tx, ty)
		uslope, uinter, _, _, _, _ := stats.LinearRegression(ux, uy)

		oneScript := newPlotScript(output, tname, odir)
		allScripts = append(allScripts, oneScript)
		mScript := multiScripter(oneScript, comboScript)

		mScript.WriteString(fmt.Sprintf("t%s(x)=%f*x+%f\n", lstr, tslope, tinter))
		mScript.WriteString(fmt.Sprintf("u%s(x)=%f*x+%f\n", lstr, uslope, uinter))

		mScript.add(fmt.Sprintf("'%s' using 1:3 title 'untraced - %s' with point lc rgb '%s'",
			loadCsv, lstr, untracedColor(l)))
		mScript.add(fmt.Sprintf("'%s' using 4:6 title 'traced - %s' with point lc rgb '%s'",
			loadCsv, lstr, tracedColor(l)))

		mScript.add(fmt.Sprintf("u%s(x) title 'untraced - %s' with line lc rgb '%s'",
			lstr, lstr, untracedColor(l)))
		mScript.add(fmt.Sprintf("t%s(x) title 'traced - %s' with line lc rgb '%s'",
			lstr, lstr, tracedColor(l)))

		oneScript.writeBody()
	}
	comboScript.writeBody()

	for _, s := range allScripts {
		path := s.ipathFor(s.Name + ".gnuplot")
		gp := exec.Command("gnuplot", path)
		gp.Dir = odir.dir
		if err := gp.Run(); err != nil {
			bench.Fatal("gnuplot", path, err)
		}
	}

	return nil
}
