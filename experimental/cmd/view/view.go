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
	if err := os.MkdirAll(d, os.ModePerm); err != nil {
		log.Fatal("Couldn't mkdir: ", d, ": ", err)
	}
}

func view(result diffbench.Exported, dir string) {
	viewDimension(
		result, dir, "backoff", "repeat",
		result.ExperimentParams, result.RepeatParams,
		func(t *diffbench.Trials, x, y int) diffbench.Timings {
			return *t.Repeat[y].Backoff[x]
		},
	)

	viewDimension(
		result, dir, "repeat", "backoff",
		result.RepeatParams, result.ExperimentParams,
		func(t *diffbench.Trials, x, y int) diffbench.Timings {
			return *t.Repeat[x].Backoff[y]
		},
	)
}

func viewDimension(result diffbench.Exported, dir, xname, yname string, xvals, yvals []int, values func(t *diffbench.Trials, x, y int) diffbench.Timings) {
	for _, confidence := range common.ConfidenceAll {
		cdir := path.Join(dir, fmt.Sprint("conf=", confidence.C))

		yDir := func(confidence common.ZValue, y int) string {
			return path.Join(cdir, fmt.Sprint(yname, "=", y))
		}

		for _, yval := range yvals {
			var (
				ews, eus, ess, cws, cus, css common.StatsSeries
			)
			for _, xval := range xvals {
				var expt, cont common.TimingStats

				for _, t := range values(result.Experiment, xval, yval) {
					expt.Update(t)
				}
				for _, t := range values(result.Control, xval, yval) {
					cont.Update(t)
				}

				ewsc := expt.Wall.Summary(confidence)
				eusc := expt.User.Summary(confidence)
				essc := expt.UserSys().Summary(confidence)

				cwsc := cont.Wall.Summary(confidence)
				cusc := cont.User.Summary(confidence)
				cssc := cont.UserSys().Summary(confidence)

				ews.Add(float64(xval), ewsc.SubScalar(cwsc.Mean))
				eus.Add(float64(xval), eusc.SubScalar(cusc.Mean))
				ess.Add(float64(xval), essc.SubScalar(cssc.Mean))

				cws.Add(float64(xval), cwsc.SubScalar(cwsc.Mean))
				cus.Add(float64(xval), cusc.SubScalar(cusc.Mean))
				css.Add(float64(xval), cssc.SubScalar(cssc.Mean))
			}

			mkdir(yDir(confidence, yval))
			nameFor := func(n string) string {
				return path.Join(yDir(confidence, yval), n)
			}

			writeTiming(nameFor("expt.wall"), ews)
			writeTiming(nameFor("expt.user"), eus)
			writeTiming(nameFor("expt.u--s"), ess)

			writeTiming(nameFor("cont.wall"), cws)
			writeTiming(nameFor("cont.user"), cus)
			writeTiming(nameFor("cont.u--s"), css)

			plotPair(confidence, nameFor("expt.wall"), nameFor("cont.wall"), nameFor("wall"))
			plotPair(confidence, nameFor("expt.user"), nameFor("cont.user"), nameFor("user"))
			plotPair(confidence, nameFor("expt.u--s"), nameFor("cont.u--s"), nameFor("u--s"))
		}

		plotOne := func(kind string) {
			dataLoc := func(loc string) func(r int) string {
				return func(y int) string {
					return path.Join(yDir(confidence, y), fmt.Sprint(loc, ".", kind))
				}
			}
			plotExpt(fmt.Sprint(kind, " conf=", confidence.C),
				confidence,
				xvals,
				yvals,
				dataLoc("expt"),
				dataLoc("cont"),
				path.Join(cdir, yname),
			)
		}
		plotOne("wall")
		plotOne("user")
		plotOne("u--s")
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
