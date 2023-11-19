package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/smallnest/statsview/viewer"
)

const (
	// RPCXProcessTimeMetrics is the name of ProcessTimeViewer
	RPCXProcessTimeMetrics = "rpcx_processtime"
)

// ProcessTimeViewer collects metrics of rpcx.
type ProcessTimeViewer struct {
	s     *Server
	smgr  *viewer.StatsMgr
	graph *charts.Line
}

// NewProcessTimeViewer returns the ProcessTimeViewer instance
func NewProcessTimeViewer(s *Server) viewer.Viewer {
	graph := viewer.NewBasicView(RPCXProcessTimeMetrics)
	graph.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "RPCX Process Time"}),
		charts.WithYAxisOpts(opts.YAxis{Show: true, Name: "Time", AxisLabel: &opts.AxisLabel{Show: true, Formatter: "{value} ms"}}),
	)
	graph.AddSeries("Avg", []opts.LineData{})
	graph.AddSeries("P99", []opts.LineData{})
	graph.AddSeries("Max", []opts.LineData{})
	graph.AddSeries("Min", []opts.LineData{})
	graph.AddSeries("Long 5%", []opts.LineData{})
	graph.AddSeries("Short 5%", []opts.LineData{})

	return &ProcessTimeViewer{s: s, graph: graph}
}

func (vr *ProcessTimeViewer) SetStatsMgr(smgr *viewer.StatsMgr) {
	vr.smgr = smgr
}

func (vr *ProcessTimeViewer) Name() string {
	return RPCXProcessTimeMetrics
}

func (vr *ProcessTimeViewer) View() *charts.Line {
	return vr.graph
}

func (vr *ProcessTimeViewer) Serve(w http.ResponseWriter, _ *http.Request) {
	vr.smgr.Tick()

	calc := vr.s.tachymeter.Calc()
	metrics := viewer.Metrics{
		Values: []float64{
			float64(calc.Time.Avg) / 1000000,
			float64(calc.Time.P99) / 1000000,
			float64(calc.Time.Max) / 1000000,
			float64(calc.Time.Min) / 1000000,
			float64(calc.Time.Long5p) / 1000000,
			float64(calc.Time.Short5p) / 1000000,
		},
		Time: viewer.MemStats().T,
	}

	bs, _ := json.Marshal(metrics)
	w.Write(bs)
}
