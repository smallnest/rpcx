package reflection

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/ChimeraCoder/gojson"
	jsoniter "github.com/json-iterator/go"
	"github.com/smallnest/rpcx/v5/log"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()
var typeOfContext = reflect.TypeOf((*context.Context)(nil)).Elem()

var json = jsoniter.Config{
	TagKey: "-",
}.Froze()

// ServiceInfo service info.
type ServiceInfo struct {
	Name    string
	PkgPath string
	Methods []*MethodInfo
}

// MethodInfo method info
type MethodInfo struct {
	Name      string
	ReqName   string
	Req       string
	ReplyName string
	Reply     string
}

var siTemplate = `package {{.PkgPath}}

type {{.Name}} struct{}
{{$name := .Name}}
{{range .Methods}}
{{.Req}}
{{.Reply}}
type (s *{{$name}}) {{.Name}}(ctx context.Context, arg *{{.ReqName}}, reply *{{.ReplyName}}) error {
	return nil
}
{{end}}
`

func (si ServiceInfo) String() string {
	tpl := template.Must(template.New("service").Parse(siTemplate))
	var buf bytes.Buffer
	tpl.Execute(&buf, si)
	return buf.String()
}

type Reflection struct {
	Services map[string]*ServiceInfo
}

func New() *Reflection {
	return &Reflection{
		Services: make(map[string]*ServiceInfo),
	}
}
func (r *Reflection) Register(name string, rcvr interface{}, metadata string) error {
	var si = &ServiceInfo{}

	val := reflect.ValueOf(rcvr)
	typ := reflect.TypeOf(rcvr)
	vTyp := reflect.Indirect(val).Type()
	si.Name = vTyp.Name()
	pkg := vTyp.PkgPath()
	if strings.Index(pkg, ".") > 0 {
		pkg = pkg[strings.LastIndex(pkg, ".")+1:]
	}
	pkg = filepath.Base(pkg)
	pkg = strings.ReplaceAll(pkg, "-", "_")
	si.PkgPath = pkg

	for m := 0; m < val.NumMethod(); m++ {
		method := typ.Method(m)
		mtype := method.Type

		if method.PkgPath != "" {
			continue
		}
		if mtype.NumIn() != 4 {
			continue
		}
		// First arg must be context.Context
		ctxType := mtype.In(1)
		if !ctxType.Implements(typeOfContext) {
			continue
		}

		// Second arg need not be a pointer.
		argType := mtype.In(2)
		if !isExportedOrBuiltinType(argType) {
			continue
		}
		// Third arg must be a pointer.
		replyType := mtype.In(3)
		if replyType.Kind() != reflect.Ptr {
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {

			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			continue
		}

		mi := &MethodInfo{}
		mi.Name = method.Name

		if argType.Kind() == reflect.Ptr {
			argType = argType.Elem()
		}
		replyType = replyType.Elem()

		mi.ReqName = argType.Name()
		mi.Req = generateTypeDefination(mi.ReqName, si.PkgPath, generateJSON(argType))
		mi.ReplyName = replyType.Name()
		mi.Reply = generateTypeDefination(mi.ReplyName, si.PkgPath, generateJSON(replyType))

		si.Methods = append(si.Methods, mi)
	}

	if len(si.Methods) > 0 {
		r.Services[name] = si
	}

	return nil
}

func (r *Reflection) Unregister(name string) error {
	delete(r.Services, name)
	return nil
}

func (r *Reflection) GetService(ctx context.Context, s string, reply *string) error {
	si, ok := r.Services[s]
	if !ok {
		return fmt.Errorf("not found service %s", s)
	}
	*reply = si.String()

	return nil
}

func (r *Reflection) GetServices(ctx context.Context, s string, reply *string) error {

	var buf bytes.Buffer

	var pkg = `package `

	for _, si := range r.Services {
		if pkg == `` {
			pkg = pkg + si.PkgPath + "\n\n"
		}
		buf.WriteString(strings.ReplaceAll(si.String(), pkg, ""))
	}

	if pkg != `package ` {
		*reply = pkg + buf.String()
	}

	return nil
}

func generateTypeDefination(name, pkg string, jsonValue string) string {
	jsonValue = strings.TrimSpace(jsonValue)
	if jsonValue == "" || jsonValue == `""` {
		return ""
	}
	r := strings.NewReader(jsonValue)
	output, err := gojson.Generate(r, gojson.ParseJson, name, pkg, nil, false, false)
	if err != nil {
		log.Errorf("failed to generate json: %v", err)
		return ""
	}
	rt := strings.ReplaceAll(string(output), "``", "")
	return strings.ReplaceAll(rt, "package "+pkg+"\n\n", "")
}

func generateJSON(typ reflect.Type) string {
	v := reflect.New(typ).Interface()

	data, _ := json.Marshal(v)
	return string(data)
}

func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return isExported(t.Name()) || t.PkgPath() == ""
}
