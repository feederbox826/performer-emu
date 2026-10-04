package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// typings
type GraphQLRequest struct {
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

type PerformerImage struct {
	URL string `json:"url"`
}

type Performer struct {
	Name   string           `json:"name"`
	ID     string           `json:"id"`
	Images []PerformerImage `json:"images"`
}

type RootResponse struct {
	Endpoint   string         `json:"endpoint"`
	APIKey     string         `json:"apikey"`
	Performers map[string]int `json:"performers"`
}

type SearchPerformerResponse struct {
	Data struct {
		SearchPerformer []Performer `json:"searchPerformer"`
	} `json:"data"`
}

type FindPerformerResponse struct {
	Data struct {
		FindPerformer *Performer `json:"findPerformer"`
	} `json:"data"`
}

var meResponse = []byte(`{"data":{"me":{"name":"anonymous"}}}`)
var rootResponse []byte
var (
	baseURL      string
	basePath     string
	endpoint     string
	fileExt      string
	performers   []Performer
	performerMap map[string]string
)

func main() {
	baseURL = os.Getenv("BASE_URL")
	basePath = os.Getenv("BASE_PATH")
	endpoint = os.Getenv("ENDPOINT")
	fileExt = os.Getenv("FILE_EXT")
	if fileExt == "" {
		fileExt = ".webp"
	}
	if baseURL == "" || basePath == "" || endpoint == "" {
		log.Fatal("BASE_URL, BASE_PATH, and ENDPOINT environment variables must be set")
	}

	loadImages()

	http.HandleFunc("/graphql", graphqlHandler)
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/performers/", performerHandler)

	log.Println("Server running on :10103")
	log.Fatal(http.ListenAndServe(":10103", nil))
}

func samePerformer(name, base string) bool {
	return name == base || strings.HasPrefix(name, base+" ") || strings.HasPrefix(name, base+" (")
}

func normalizeName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "-", " ")
	return strings.Join(strings.Fields(s), " ")
}

func loadImages() {
	type image struct {
		rel, name, url string
	}

	var files []image
	var names []string

	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), fileExt) {
			return nil
		}

		rel, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		name := strings.TrimSuffix(d.Name(), fileExt)
		imageURL, err := url.JoinPath(baseURL, rel)
		if err != nil {
			return err
		}

		files = append(files, image{rel, name, imageURL})
		if dir := filepath.Dir(rel); dir != "." {
			names = append(names, filepath.Base(dir))
		} else {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	groups := map[string][]PerformerImage{}
	performerMap = map[string]string{}
	index := map[string]int{}

	for _, f := range files {
		group := f.name
		if dir := filepath.Dir(f.rel); dir != "." {
			group = filepath.Base(dir)
		} else {
			for _, n := range names {
				if samePerformer(f.name, n) && len(n) < len(group) {
					group = n
				}
			}
		}

		groups[group] = append(groups[group], PerformerImage{URL: f.url})
		performerMap[f.name] = f.url
		performerMap[strings.TrimSuffix(f.rel, fileExt)] = f.url
		if f.name == group || performerMap[group] == "" {
			performerMap[group] = f.url
		}
	}

	performers = make([]Performer, 0, len(groups))
	for name, images := range groups {
		performers = append(performers, Performer{Name: name, ID: name, Images: images})
		index[name] = len(images)
	}

	rootResponse, _ = json.Marshal(RootResponse{
		Endpoint:   endpoint,
		APIKey:     "whatever",
		Performers: index,
	})
	log.Printf("loaded %d performers, %d images", len(performers), len(files))
}

func graphqlHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	switch req.OperationName {
	case "Me":
		w.Header().Set("Content-Type", "application/json")
		w.Write(meResponse)
	case "SearchPerformer":
		term, _ := req.Variables["term"].(string)
		matches := []Performer{}
		if term != "" {
			termNorm := normalizeName(term)
			for _, p := range performers {
				if strings.HasPrefix(normalizeName(p.Name), termNorm) {
					matches = append(matches, p)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		resp := SearchPerformerResponse{}
		resp.Data.SearchPerformer = matches
		json.NewEncoder(w).Encode(resp)
	case "FindPerformerByID":
		id, _ := req.Variables["id"].(string)
		resp := FindPerformerResponse{}
		if url, ok := performerMap[id]; ok {
			resp.Data.FindPerformer = &Performer{Name: id, ID: id, Images: []PerformerImage{{URL: url}}}
		}
		json.NewEncoder(w).Encode(resp)
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]interface{}{}})
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(rootResponse)
}

func performerHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/performers/")
	if url, ok := performerMap[id]; ok {
		http.Redirect(w, r, url, http.StatusFound)
	} else {
		http.Error(w, "Performer not found", http.StatusNotFound)
	}
}
