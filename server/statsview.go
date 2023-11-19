package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/templates"
	"github.com/rs/cors"
	"github.com/smallnest/statsview/statics"
	"github.com/smallnest/statsview/viewer"
)

// ViewManager
type ViewManager struct {
	ln  net.Listener
	srv *http.Server

	Smgr   *viewer.StatsMgr
	Ctx    context.Context
	Cancel context.CancelFunc
	Views  []viewer.Viewer
}

// Register registers views to the ViewManager
func (vm *ViewManager) Register(views ...viewer.Viewer) {
	vm.Views = append(vm.Views, views...)

}

// Start runs a http server and begin to collect metrics
func (vm *ViewManager) Start() error {
	return vm.srv.Serve(vm.ln)
}

// Stop shutdown the http server gracefully
func (vm *ViewManager) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	vm.srv.Shutdown(ctx)
	vm.Cancel()
}

func init() {
	templates.PageTpl = `
{{- define "page" }}
<!DOCTYPE html>
<html>
    {{- template "header" . }}
<body>
<p>&nbsp;&nbsp;rpcx profiler</em></p>
<style> .box { justify-content:center; display:flex; flex-wrap:wrap } </style>
<div class="box"> {{- range .Charts }} {{ template "base" . }} {{- end }} </div>
</body>
</html>
{{ end }}
`
}

// NewViewManager creates a new ViewManager instance
func NewViewManager(ln net.Listener, s *Server) *ViewManager {
	viewer.SetConfiguration(viewer.WithAddr(ln.Addr().String()), viewer.WithLinkAddr(ln.Addr().String()))

	page := components.NewPage()
	page.PageTitle = "Statsview"
	page.AssetsHost = fmt.Sprintf("http://%s/debug/statsview/statics/", viewer.LinkAddr())
	page.Assets.JSAssets.Add("jquery.min.js")

	srv := &http.Server{
		ReadTimeout:    time.Minute,
		WriteTimeout:   time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	mgr := &ViewManager{
		ln:  ln,
		srv: srv,
	}

	mgr.Ctx, mgr.Cancel = context.WithCancel(context.Background())
	mgr.Register(
		viewer.NewGoroutinesViewer(),
		viewer.NewHeapViewer(),
		viewer.NewStackViewer(),
		viewer.NewGCNumViewer(),
		viewer.NewGCSizeViewer(),
		viewer.NewGCCPUFractionViewer(),
		NewHandlerViewer(s),
		NewProcessTimeViewer(s),
		NewRequestRateViewer(s),
	)
	smgr := viewer.NewStatsMgr(mgr.Ctx)
	for _, v := range mgr.Views {
		v.SetStatsMgr(smgr)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	for _, v := range mgr.Views {
		page.AddCharts(v.View())
		mux.HandleFunc("/debug/statsview/view/"+v.Name(), v.Serve)
	}

	mux.HandleFunc("/debug/statsview", func(w http.ResponseWriter, _ *http.Request) {
		page.Render(w)
	})

	staticsPrev := "/debug/statsview/statics/"
	mux.HandleFunc(staticsPrev+"echarts.min.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(statics.EchartJS))
	})

	mux.HandleFunc(staticsPrev+"jquery.min.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(statics.JqueryJS))
	})

	mux.HandleFunc(staticsPrev+"themes/westeros.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(statics.WesterosJS))
	})

	mux.HandleFunc(staticsPrev+"themes/macarons.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(statics.MacaronsJS))
	})

	mgr.srv.Handler = cors.AllowAll().Handler(mux)

	return mgr
}
