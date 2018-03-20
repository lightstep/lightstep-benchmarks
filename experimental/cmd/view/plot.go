package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os/exec"

	"github.com/lightstep/lightstep-benchmarks/common"
)

type (
	Experiment struct {
		Repeat         int
		Confidence     float64
		ExperimentFile string
		ControlFile    string
		OutputFile     string
	}
)

const (
	pairwiseScript = `
set terminal png
set output '{{.OutputFile}}'
plot '{{.ExperimentFile}}' using 1:4:3:5 with errorbars, '{{.ControlFile}}' using 1:4:3:5 with errorbars
`
)

var (
	pairwisePlot = template.Must(template.New("pairwisePlot").Parse(pairwiseScript))
)

func plotPair(repeat int, confidence common.ZValue, expt, cont, output string) {
	var script bytes.Buffer

	err := pairwisePlot.Execute(&script, Experiment{
		Repeat:         repeat,
		Confidence:     confidence.C,
		ExperimentFile: expt,
		ControlFile:    cont,
		OutputFile:     output + ".png",
	})
	if err != nil {
		log.Fatal("Could not exec template: ", err)
	}

	fmt.Println("Script is ", script.String())
	cmd := exec.Command("gnuplot")
	cmd.Stdin = &script

	if cmd.Run(); err != nil {
		log.Fatal("Command failed: gnutplot: ", err)
	}
}
