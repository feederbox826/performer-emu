package main

import (
	"encoding/json"
	"log"
	"net/http"
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

type MeResponse struct {
	Data struct {
		Me struct {
			Name string `json:"name"`
		} `json:"me"`
	} `json:"data"`
}

type RootResponse struct {
	Endpoint string `json:"endpoint"`
	APIKey   string `json:"apikey"`
}

type SearchPerformerResponse struct {
	Data struct {
		SearchPerformer []Performer `json:"searchPerformer"`
	} `json:"data"`
}

// init
var (
	baseURL      string
	basePath     string
	endpoint     string
	file_ext     string
	performers   []Performer
	performerMap map[string]string
)

func main() {
	// Load environment variables with defaults
	baseURL = os.Getenv("BASE_URL")
	basePath = os.Getenv("BASE_PATH")
	endpoint = os.Getenv("ENDPOINT")
	file_ext = os.Getenv("FILE_EXT")
	if file_ext == "" {
		file_ext = ".webp"
	}
	// check if all are defined
	if baseURL == "" || basePath == "" || endpoint == "" {
		log.Fatal("BASE_URL, BASE_PATH, and ENDPOINT environment variables must be set")
	}

	loadImages()
	responseInit()

	http.HandleFunc("/graphql", graphqlHandler)
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/performers/", performerHandler)

	log.Println("Server running on :10103")
	log.Fatal(http.ListenAndServe(":10103", nil))
}

// load all images recursively
func loadImages() {
	performers = []Performer{}
	performerMap = make(map[string]string)
	filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), file_ext) {
			name := strings.TrimSuffix(d.Name(), file_ext)
			relPath := filepath.ToSlash(path[len(basePath):])
			url := baseURL + relPath
			perf := Performer{Name: name, ID: name, Images: []PerformerImage{{URL: url}}}
			performers = append(performers, perf)
			performerMap[name] = url
		}
		return nil
	})
}

var meResponse []byte
var rootResponse []byte

func responseInit() {
	// set up meResponse
	meResp := MeResponse{}
	meResp.Data.Me.Name = "anonymous"
	meResponse, _ = json.Marshal(meResp)
	// set up rootResponse
	rootResp := RootResponse{
		Endpoint: endpoint,
		APIKey:   "whatever",
	}
	rootResponse, _ = json.Marshal(rootResp)
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
		termLower := strings.ToLower(term)
		for _, p := range performers {
			if strings.HasPrefix(strings.ToLower(p.Name), termLower) {
				matches = append(matches, p)
			}
		}
		resp := SearchPerformerResponse{}
		resp.Data.SearchPerformer = matches
		json.NewEncoder(w).Encode(resp)
	default:
		json.NewEncoder(w).Encode(map[string]interface{}{req.OperationName: map[string]interface{}{}})
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
