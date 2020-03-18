package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/yuin/goldmark"
)

type server struct {
	pagesDir *os.File
	pages    map[string]bool
	pMu      sync.Mutex
}

func listDir(dir *os.File) (map[string]bool, error) {
	fileNames, err := dir.Readdirnames(0)
	if err != nil {
		return nil, err
	}
	pages := map[string]bool{}
	for _, name := range fileNames {
		if strings.HasSuffix(name, ".md") {
			pages[name] = true
		}
	}
	return pages, nil
}

func (s *server) refresh() {
	s.pMu.Lock()
	defer s.pMu.Unlock()
	pages, err := listDir(s.pagesDir)
	if err != nil {
		fmt.Printf("failed to refresh pages: %s\n", err)
		return
	}
	s.pages = pages
	fmt.Println(s.pages)
}

var indexTemplate = template.Must(template.New("index").Parse(`<html>
	<body>
		<a href="/new">new</a> <a href="/refresh">refresh</a>
		<ul>
			{{range $index, $element := .}}<li><a href="/page/{{$index}}">{{$index}}</a></li>{{end}}
		</ul>
	</body>
</html>`))

func (s *server) index(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if req.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.pMu.Lock()
	defer s.pMu.Unlock()
	err := indexTemplate.Execute(w, s.pages)
	if err != nil {
		fmt.Printf("failed to execute index template: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	return
}

func (s *server) edit() error {
	return nil
}

var pageTemplate = template.Must(template.New("index").Parse(`<html>
	<body>
		<a href="/">index</a> <a href="/edit/{{.Name}}">edit</a> <a href="/delete/{{.Name}}">delete</a>
		{{.Content}}
	</body>
</html>`))

func (s *server) page(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		if strings.HasSuffix(req.URL.Path, "/edit") {
			err := s.edit()
			if err != nil {
				// ???
			}
			w.WriteHeader(http.StatusOK) // ??? redirect back to the page or something? idk
		}
		// idk if this is the most obvious status
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	page := strings.TrimPrefix(req.URL.Path, "/page/")
	fmt.Println(page)
	if _, ok := s.pages[page]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	content, err := ioutil.ReadFile(page)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// write out error
		return
	}
	buf := new(bytes.Buffer)
	err = goldmark.Convert(content, buf)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		// write out error
		return
	}
	err = pageTemplate.Execute(w, struct {
		Name    string
		Content string
	}{Name: page, Content: string(buf.Bytes())})
	if err != nil {
		fmt.Printf("failed to execute page template: %s\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	return
}

var newPage = `<html>
	<body>
		<form action="/new">
			file name:<br>
			<input type="text" id="fname" name="fname" value=""><br>
			content:<br>
			<input type="text" id="content" name="lname" value=""><br><br>
			<input type="submit" formmethod="post" value="submit">
		</form>
	</body>
</html>`

func (s *server) new(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		w.Write([]byte(newPage))
	case http.MethodPost:
		w.Write([]byte("well hello there"))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func isDir(f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("is not a directory")
	}
	return nil
}

func main() {
	pagesDir := flag.String("pages", "", "")
	listen := flag.String("listen", "", "")
	flag.Parse()

	pd, err := os.Open(*pagesDir)
	if err != nil {
		fmt.Printf("failed to open pages directory: %s\n", err)
		os.Exit(1)
	}
	if err := isDir(pd); err != nil {
		fmt.Printf("failed checking pages directory: %s\n", err)
	}

	s := &server{
		pagesDir: pd,
	}
	s.refresh()

	mux := http.NewServeMux()
	mux.HandleFunc("/page/", s.page)
	mux.HandleFunc("/new", s.new)
	mux.HandleFunc("/", s.index)

	err = http.ListenAndServe(*listen, mux)
	if err != nil {
		fmt.Printf("http server failed: %s\n", err)
		os.Exit(1)
	}
}
