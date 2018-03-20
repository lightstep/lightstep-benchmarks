package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"

	"github.com/lightstep/lightstep-benchmarks/common"
	"github.com/lightstep/lightstep-benchmarks/experimental/diffbench"
)

func main() {
	var result diffbench.Exported

	if len(os.Args) != 3 {
		log.Fatalf("Usage: %v filename output\n", os.Args[0])
	}

	file := os.Args[1]
	dir := os.Args[2]

	mkdir(dir)

	if data, err := ioutil.ReadFile(file); err != nil {
		log.Fatal("Could not read: ", err)
	} else if err := json.Unmarshal(data, &result); err != nil {
		log.Fatal("Could not parse: ", err)
	} else {
		view(result, dir)
	}
}

func mkdir(d string) {
	if err := os.Mkdir(d, os.ModePerm); err != nil {
		log.Fatal("Couldn't mkdir: ", d, ": ", err)
	}
}

func view(result diffbench.Exported, dir string) {
	for _, confidence := range common.ConfidenceAll {
		cdir := path.Join(dir, fmt.Sprint("conf=", confidence.C))
		mkdir(cdir)
		for _, repeat := range result.RepeatParams {
			var (
				ews, eus, ess, cws, cus, css common.StatsSeries
			)
			rdir := path.Join(cdir, fmt.Sprint("repeat=", repeat))
			mkdir(rdir)
			for _, e := range result.ExperimentParams {
				var expt, cont common.TimingStats

				for _, t := range *result.Experiment.Repeat[repeat].Backoff[e] {
					expt.Update(t)
				}
				for _, t := range *result.Control.Repeat[repeat].Backoff[e] {
					cont.Update(t)
				}

				ewsc := expt.Wall.Summary(confidence)
				eusc := expt.User.Summary(confidence)
				essc := expt.UserSys().Summary(confidence)

				cwsc := cont.Wall.Summary(confidence)
				cusc := cont.User.Summary(confidence)
				cssc := cont.UserSys().Summary(confidence)

				ews.Add(float64(e), ewsc.SubScalar(cwsc.Mean))
				eus.Add(float64(e), eusc.SubScalar(cusc.Mean))
				ess.Add(float64(e), essc.SubScalar(cssc.Mean))

				cws.Add(float64(e), cwsc.SubScalar(cwsc.Mean))
				cus.Add(float64(e), cusc.SubScalar(cusc.Mean))
				css.Add(float64(e), cssc.SubScalar(cssc.Mean))
			}

			nameFor := func(n string) string {
				return path.Join(rdir, n)
			}

			writeTiming(nameFor("expt.wall"), ews)
			writeTiming(nameFor("expt.user"), eus)
			writeTiming(nameFor("expt.u--s"), ess)

			writeTiming(nameFor("cont.wall"), cws)
			writeTiming(nameFor("cont.user"), cus)
			writeTiming(nameFor("cont.u--s"), css)

			plotPair(repeat, confidence, nameFor("expt.wall"), nameFor("cont.wall"), nameFor("wall"))
			plotPair(repeat, confidence, nameFor("expt.user"), nameFor("cont.user"), nameFor("user"))
			plotPair(repeat, confidence, nameFor("expt.u--s"), nameFor("cont.u--s"), nameFor("u--s"))
		}
	}
}

func writeTiming(file string, ts common.StatsSeries) {
	var buf bytes.Buffer
	for i, summary := range ts.Summaries {
		str := fmt.Sprintf("%.9g", []float64{
			ts.Coordinates[i],
			summary.ZValue.C,
			summary.CLow,
			summary.Mean,
			summary.CHigh,
			summary.Min,
			summary.P25,
			summary.P50,
			summary.P75,
			summary.Max})
		buf.WriteString(str[1 : len(str)-2])
		buf.WriteString("\n")
	}
	if err := ioutil.WriteFile(file, buf.Bytes(), os.ModePerm); err != nil {
		log.Fatal("Could not write: ", file, ": ", err)
	}
}
