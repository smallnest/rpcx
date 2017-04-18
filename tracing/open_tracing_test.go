package tracing

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"testing"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/smallnest/rpcx/core"
	"sourcegraph.com/sourcegraph/appdash"
	appdashtracer "sourcegraph.com/sourcegraph/appdash/opentracing"
	"sourcegraph.com/sourcegraph/appdash/traceapp"
)

func TestOpenTracingPlugin(t *testing.T) {
	tracer := getTracer()
	p := NewOpenTracingPlugin(tracer)

	header := make(map[string][]string)
	ctx := core.NewContext(context.Background(), header)
	err := p.DoPreCall(ctx, "greeting.Say", "smallnest", "")
	if err != nil {
		t.Fatalf("failed to DoPreCall: %v", err)
	}
	err = p.DoPostCall(ctx, "greeting.Say", "smallnest", "")
	if err != nil {
		t.Fatalf("failed to DoPostCall: %v", err)
	}

	ctx = core.NewMapContext(context.Background())
	err = p.DoPostReadRequestHeader(ctx, &core.Request{ServiceMethod: "greeting.Say"})
	if err != nil {
		t.Fatalf("failed to DoPostReadRequestHeader: %v", err)
	}

	err = p.DoPostWriteResponse(ctx, &core.Response{ServiceMethod: "greeting.Say"}, "world")
	if err != nil {
		t.Fatalf("failed to DoPostWriteResponse: %v", err)
	}

	time.Sleep(time.Hour)
	// see http://localhost:8700/traces to check
}
func getTracer() opentracing.Tracer {
	memStore := appdash.NewMemoryStore()
	store := &appdash.RecentStore{
		MinEvictAge: 20 * time.Second,
		DeleteStore: memStore,
	}

	url, err := url.Parse("http://localhost:8700")
	if err != nil {
		log.Fatal(err)
	}
	tapp, err := traceapp.New(nil, url)
	if err != nil {
		log.Fatal(err)
	}
	tapp.Store = store
	tapp.Queryer = memStore
	log.Println("Appdash web UI running on HTTP :8700")
	go func() {
		log.Fatal(http.ListenAndServe(":8700", tapp))
	}()

	collector := appdash.NewLocalCollector(store)

	tracer := appdashtracer.NewTracer(collector)
	opentracing.InitGlobalTracer(tracer)

	return tracer
}
