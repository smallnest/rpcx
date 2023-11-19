package server

import (
	"encoding/json"
	"net/http"
	"sync/atomic"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/smallnest/statsview/viewer"
)

const (
	// RPCXHandlerMetrics is the name of HandlerViewer
	RPCXHandlerMetrics = "rpcx_handler"
)

// HandlerViewer collects metrics of rpcx.
type HandlerViewer struct {
	s     *Server
	smgr  *viewer.StatsMgr
	graph *charts.Line
}

// NewHandlerViewer returns the HandlerViewer instance
func NewHandlerViewer(s *Server) viewer.Viewer {
	graph := viewer.NewBasicView(RPCXHandlerMetrics)
	graph.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "RPCX Handler"}),
		charts.WithYAxisOpts(opts.YAxis{Show: true, Name: "Requests"}),
	)
	graph.AddSeries("Inflight", []opts.LineData{})

	return &HandlerViewer{s: s, graph: graph}
}

func (vr *HandlerViewer) SetStatsMgr(smgr *viewer.StatsMgr) {
	vr.smgr = smgr
}

func (vr *HandlerViewer) Name() string {
	return RPCXHandlerMetrics
}

func (vr *HandlerViewer) View() *charts.Line {
	return vr.graph
}

func (vr *HandlerViewer) Serve(w http.ResponseWriter, _ *http.Request) {
	vr.smgr.Tick()

	metrics := viewer.Metrics{
		Values: []float64{float64(atomic.LoadInt32(&vr.s.handlerMsgNum))},
		Time:   viewer.MemStats().T,
	}

	bs, _ := json.Marshal(metrics)
	w.Write(bs)
}
