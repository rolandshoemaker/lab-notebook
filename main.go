package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
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

func (s *server) refresh() error {
	s.pMu.Lock()
	defer s.pMu.Unlock()
	pages, err := listDir(s.pagesDir)
	if err != nil {
		return err
	}
	s.pages = pages
	return nil
}

func (s *server) refreshEndpoint(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	if err := s.refresh(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("failed to read pages directory %q: %s", s.pagesDir.Name(), err)))
		return
	}
	w.WriteHeader(http.StatusOK)
	return
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
	if _, ok := s.pages[page]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	content, err := ioutil.ReadFile(path.Join(s.pagesDir.Name(), page))
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
			<input type="text" name="fname" value=""><br>
			content:<br>
			<textarea name="content" rows="10" cols="30"></textarea><br>
			<input type="submit" formmethod="post" value="submit">
		</form>
	</body>
</html>`

func (s *server) new(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		w.Write([]byte(newPage))
	case http.MethodPost:
		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("failed to parse form: %s", err)))
			return
		}

		w.Write([]byte(fmt.Sprintf("%#v", req.PostForm)))
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func (s *server) delete(w http.ResponseWriter, req *http.Request) {
	fmt.Println("wee woo")
	if req.Method != http.MethodPost {
		fmt.Println("???")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	fmt.Println(req.URL.Path)
	page := strings.TrimPrefix(req.URL.Path, "/delete/")
	if _, ok := s.pages[page]; !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err := os.Remove(path.Join(s.pagesDir.Name(), page))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
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
		os.Exit(1)
	}

	s := &server{
		pagesDir: pd,
	}
	if err := s.refresh(); err != nil {
		fmt.Printf("failed to read pages directory: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/page/", s.page)
	mux.HandleFunc("/new", s.new)
	mux.HandleFunc("/delete/", s.delete)
	mux.HandleFunc("/refresh", s.refreshEndpoint)
	mux.HandleFunc("/", s.index)

	fmt.Printf("listening on %s\n", *listen)
	err = http.ListenAndServe(*listen, mux)
	if err != nil {
		fmt.Printf("http server failed: %s\n", err)
		os.Exit(1)
	}
}
