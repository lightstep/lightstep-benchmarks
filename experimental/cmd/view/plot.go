package main

import (
	"bytes"
	"log"
	"os/exec"
	"text/template"

	"github.com/lightstep/lightstep-benchmarks/common"
)

type (
	Experiment struct {
		Confidence float64
		Data       Inputs
		OutputFile string
	}

	Input struct {
		Filename string
		Param    interface{}
	}

	Inputs []Input

	RepeatExperiment struct {
		Confidence float64
		Repeats    []int
		Experiment []int
		Data       []Input
		OutputFile string
	}
)

const (
	// . is an Experiment
	pairwisePlot = `
set terminal png
set output '{{.OutputFile}}'
plot {{ range $index, $input := .Data }}{{ $input.TwoD }},{{ end }}
`

	// . is a RepeatExperiment
	repeatPlot = `
set border 127 front lt black linewidth 1.000 dashtype solid
set style fill transparent solid 0.50 border

set terminal png
set output '{{.OutputFile}}'
set xrange [{{.MinXRange}}:{{.MaxXRange}}]
set yrange [{{.MinRepeatRange}}:{{.MaxRepeatRange}}]

set pm3d depthorder

splot {{ range $index, $input := .Data }}{{ $input.ThreeD }},{{ end }}
`

	// . is an Input
	twoDLine   = `'{{.Filename}}' using 1:4:3:5 with errorbars`
	threeDLine = `'{{.Filename}}' using 1:({{.Param}}):4:3:5 with zerrorfill lt black fc lt {{.Param}} title "k = {{.Param}}"`
)

var (
	pairwisePlotT = template.Must(template.New("pairwisePlot").Parse(pairwisePlot))
	repeatPlotT   = template.Must(template.New("repeatPlot").Parse(repeatPlot))
	twoDLineT     = template.Must(template.New("twoDLine").Parse(twoDLine))
	threeDLineT   = template.Must(template.New("threeDLine").Parse(threeDLine))
)

func expand(tmpl *template.Template, arg interface{}) string {
	var out bytes.Buffer

	if err := tmpl.Execute(&out, arg); err != nil {
		log.Fatal("Template excute failure: ", err)
	}
	return out.String()
}

func plotPair(confidence common.ZValue, expt, cont, output string) {
	gnuplot(expand(pairwisePlotT, Experiment{
		Confidence: confidence.C,
		Data: []Input{
			Input{Filename: expt},
			Input{Filename: cont},
		},
		OutputFile: output + ".png",
	}))
}

func gnuplot(script string) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("gnuplot")
	cmd.Stdin = bytes.NewBufferString(script)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Fatal("Command failed: gnuplot:\n", stdout.String(), "\n", stderr.String(), "\nerror:", err)
	}
}

func plotExpt(title string,
	confidence common.ZValue,
	experiment []int,
	repeats []int,
	expt func(x int) string,
	cont func(x int) string,
	output string) {
	var ins Inputs
	for i := len(repeats) - 1; i >= 0; i-- {
		r := repeats[i]
		ins = append(ins, Input{
			Filename: expt(r),
			Param:    r,
		})
		ins = append(ins, Input{
			Filename: cont(r),
			Param:    r,
		})
	}
	gnuplot(expand(repeatPlotT, RepeatExperiment{
		Data:       ins,
		Repeats:    repeats,
		Experiment: experiment,
		Confidence: confidence.C,
		OutputFile: output + ".png",
	}))
}

func (i Input) TwoD() string {
	return expand(twoDLineT, i)
}

func (i Input) ThreeD() string {
	return expand(threeDLineT, i)
}

func (e RepeatExperiment) MinRepeatRange() float64 {
	return float64(e.Repeats[0]) - 0.5
}
func (e RepeatExperiment) MaxRepeatRange() float64 {
	return float64(e.Repeats[len(e.Repeats)-1]) + 0.5
}

func (e RepeatExperiment) MinXRange() float64 {
	return 0
}
func (e RepeatExperiment) MaxXRange() float64 {
	return float64(e.Experiment[len(e.Experiment)-1])
}
