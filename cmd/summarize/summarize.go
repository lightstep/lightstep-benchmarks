package main

import (
	"bytes"
	"context"
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

	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"cloud.google.com/go/storage"
	"github.com/golang/glog"
	"github.com/lightstep/lightstep-benchmarks/bench"
	"github.com/lightstep/lightstep-benchmarks/common"
	"github.com/lightstep/lightstep-benchmarks/env"
	"golang.org/x/oauth2/google"
)

const (
	testStorageBucket = "lightstep-client-benchmarks"
)

var (
	testName = flag.String("test", "", "Name of the test")
)

func tranchName(l float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", l), ".", ".", -1)
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

	// if *testName == "" {
	// 	usage()
	// }

	ctx := context.Background()
	gcpClient, err := google.DefaultClient(ctx, storage.ScopeFullControl)
	if err != nil {
		glog.Fatal("GCP Default client: ", err)
	}
	storageClient, err := storage.NewClient(ctx, option.WithHTTPClient(gcpClient))
	if err != nil {
		log.Fatal("GCP Storage client", err)
	}
	defer storageClient.Close()
	bucket := storageClient.Bucket(testStorageBucket)

	olist := bucket.Objects(ctx, nil)
	s := summarizer{}
	prefix := *testName + "/"
	for {
		obj, err := olist.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal("GCP bucket error: ", err)
		}
		if *testName == "" {
			fmt.Println("Found test", obj.Name)
			continue
		}
		if !strings.HasPrefix(obj.Name, prefix) {
			continue
		}
		if err := s.getResults(ctx, bucket, obj.Name); err != nil {
			log.Fatal("Couldn't read results: ", obj.Name)
		}
	}

}

type outputDir struct {
	*common.Output
	dir string
	out string
}

func newOutputDir(output *common.Output) outputDir {
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
	output := common.Output{}
	if err := json.Unmarshal(data, &output); err != nil {
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

func newPlotScript(output *common.Output, name string, odir outputDir) *plotScript {
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
set ylabel "Tracing CPU Impairment"
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

func (s *summarizer) getMeasurements(output *common.Output) error {
	// by target load factor, by target rate
	loadMap := map[float64]map[float64][]common.Measurement{}
	count := 0
	for _, p := range output.Results {
		p := p
		// TODO note this is still here.
		if p.Completion < 0.95 {
			continue
		}
		lm := loadMap[p.TargetLoad]
		if lm == nil {
			lm = map[float64][]common.Measurement{}
			loadMap[p.TargetLoad] = lm
		}
		lm[p.TargetRate] = append(lm[p.TargetRate], p)
		count++
	}
	if count < 5 {
		glog.Info("%s: %d incomplete results", output, len(output.Results)-count)
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
		//measurements := loadMap[l]
		//count := len(measurements)
		experiments := loadMap[l]

		// tx := make([]float64, count)
		// ty := make([]float64, count)
		// ux := make([]float64, count)
		// uy := make([]float64, count)

		var buffer bytes.Buffer

		rateVals := []float64{}
		for r, _ := range experiments {
			rateVals = append(rateVals, r)
		}
		sort.Float64s(rateVals)

		for _, r := range rateVals {

			var trate, urate bench.Stats
			var timpair, uimpair bench.Stats

			for _, m := range experiments[r] {
				tm := m.Traced
				um := m.Untraced
				trate.Update(tm.RequestRate)
				urate.Update(um.RequestRate)
				timpair.Update(tm.VisibleImpairment())
				uimpair.Update(um.VisibleImpairment())
			}
			trateLow, trateHigh := trate.NormalConfidenceInterval()
			timpairLow, timpairHigh := timpair.NormalConfidenceInterval()
			urateLow, urateHigh := urate.NormalConfidenceInterval()
			uimpairLow, uimpairHigh := uimpair.NormalConfidenceInterval()

			baseline := uimpair.Mean()

			buffer.Write([]byte(fmt.Sprintf("%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f,%.8f\n",
				trate.Mean(),
				trateLow, trateHigh,
				timpair.Mean()-baseline,
				timpairLow-baseline, timpairHigh-baseline,
				urate.Mean(),
				urateLow, urateHigh,
				uimpair.Mean()-baseline,
				uimpairLow-baseline, uimpairHigh-baseline)))
		}
		lstr := tranchName(l)

		tname := "load" + lstr
		loadCsv := tname + ".csv"
		err := ioutil.WriteFile(path.Join(odir.dir, loadCsv), buffer.Bytes(), 0755)
		if err != nil {
			glog.Fatal("Could not write file: ", err)
		}

		// http://stackoverflow.com/questions/25512006/gnuplot-smooth-confidence-interval-lines-as-opposed-to-error-bars

		oneScript := newPlotScript(output, tname, odir)
		allScripts = append(allScripts, oneScript)
		mScript := multiScripter(oneScript, comboScript)

		mScript.add(fmt.Sprintf("'%s' using 1:4:2:3:5:6 title 'traced - %s' lc rgb '%s' with xyerrorbars",
			loadCsv, lstr, tracedColor(l)))
		mScript.add(fmt.Sprintf("'%s' using 7:10:8:9:11:12 title 'untraced - %s' lc rgb '%s' with xyerrorbars",
			loadCsv, lstr, untracedColor(l)))

		oneScript.writeBody()
	}
	comboScript.writeBody()

	for _, s := range allScripts {
		path := s.ipathFor(s.Name + ".gnuplot")
		gp := exec.Command("gnuplot", path)
		gp.Dir = odir.dir
		if err := gp.Run(); err != nil {
			env.Fatal("gnuplot", path, err)
		}
	}

	return nil
}
