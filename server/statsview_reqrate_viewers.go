package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/smallnest/statsview/viewer"
)

const (
	// RPCXRequestRateMetrics is the name of RequestRateViewer
	RPCXRequestRateMetrics = "rpcx_request_rate"
)

// RequestRateViewer collects metrics of rpcx.
type RequestRateViewer struct {
	lastTime time.Time
	lastReq  uint64
	s        *Server
	smgr     *viewer.StatsMgr
	graph    *charts.Line
}

// NewRequestRateViewer returns the RequestRateViewer instance
func NewRequestRateViewer(s *Server) viewer.Viewer {
	graph := viewer.NewBasicView(RPCXRequestRateMetrics)
	graph.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "RPCX Request Rate"}),
		charts.WithYAxisOpts(opts.YAxis{Show: true, Name: "reqs/sec"}),
	)
	graph.AddSeries("Rate", []opts.LineData{})

	return &RequestRateViewer{s: s, graph: graph}
}

func (vr *RequestRateViewer) SetStatsMgr(smgr *viewer.StatsMgr) {
	vr.smgr = smgr
}

func (vr *RequestRateViewer) Name() string {
	return RPCXRequestRateMetrics
}

func (vr *RequestRateViewer) View() *charts.Line {
	return vr.graph
}

func (vr *RequestRateViewer) Serve(w http.ResponseWriter, _ *http.Request) {
	vr.smgr.Tick()

	if vr.lastTime.IsZero() {
		metrics := viewer.Metrics{
			Values: []float64{
				0.0,
			},
			Time: viewer.MemStats().T,
		}

		bs, _ := json.Marshal(metrics)
		w.Write(bs)

		vr.lastTime = time.Now()
		vr.lastReq = vr.s.requestCount.Load()

		return
	}

	now := time.Now()
	d := now.Sub(vr.lastTime).Seconds()
	currentReqCount := vr.s.requestCount.Load()
	count := currentReqCount - vr.lastReq
	rate := float64(count / uint64(d))

	vr.lastTime = now
	vr.lastReq = currentReqCount

	metrics := viewer.Metrics{
		Values: []float64{
			float64(rate),
		},
		Time: viewer.MemStats().T,
	}

	bs, _ := json.Marshal(metrics)
	w.Write(bs)
}
