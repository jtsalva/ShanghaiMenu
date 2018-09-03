package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sahilm/fuzzy"
	fuzzySpelling "github.com/sajari/fuzzy"
)

type Item struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Vegetarian bool   `json:"vegetarian"`
}

type ItemIndex struct {
	Id         string
	Terms      []string
	TermLength int32
}

// Does the string exist in the slice
func stringInSlice(s string, list []string) bool {
	matches := fuzzy.Find(s, list)
	if len(matches) > 0 {
		return true
	}
	return false
}

// http://www.golangprograms.com/remove-duplicate-values-from-slice.html
func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

type MenuEngine struct {
	Items    []Item
	index    []ItemIndex
	allTerms []string
	Spelling *fuzzySpelling.Model
}

func (m *MenuEngine) Index() {
	cleaner := strings.NewReplacer(
		"&", "and",
		"-", " ",
		",", "",
		"'", "")
	for _, item := range m.Items {
		// Replace hard to search characters with easier ones
		// Convert all characters to lower case
		name := strings.ToLower(cleaner.Replace(item.Name))

		// If there is a bracket section remove it
		bracketIndex := strings.Index(name, "(")
		if bracketIndex > -1 {
			name = name[:bracketIndex-1]
		}

		terms := strings.Split(name, " ")

		m.index = append(m.index, ItemIndex{
			Id:         item.Id,
			Terms:      terms,
			TermLength: int32(len(terms)),
		})

		m.allTerms = append(m.allTerms, terms...)
	}

	m.allTerms = unique(m.allTerms)

	m.Spelling.SetThreshold(1)
	m.Spelling.SetDepth(5)
	m.Spelling.Train(m.allTerms)
}

func (m *MenuEngine) Find(query string) []string {
	terms := strings.Split(strings.TrimSpace(strings.ToLower(query)), " ")

	log.Println(terms[0])
	log.Println(m.Spelling.Suggestions(terms[0], false))

	// List of item ids
	var passedTerms int32

	var matches []string
	for _, itemIndex := range m.index {
		passedTerms = 0
		for _, term := range terms {
			if !stringInSlice(term, itemIndex.Terms) {
				continue
			}
			passedTerms += 1
		}
		if passedTerms == int32(len(terms)) {
			matches = append(matches, itemIndex.Id)
		}
	}

	return matches
}

var engine = MenuEngine{
	Items:    menu,
	Spelling: fuzzySpelling.NewModel(),
}

func main() {
	engine.Index()

	r := mux.NewRouter()

	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/search/", searchQueryHandler)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static"))))

	http.ListenAndServe(":80", r)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("./web/index.html")
	if err != nil {
		log.Fatal(err.Error())
	}

	t.Execute(w, struct {
		Items []Item
	}{
		Items: menu,
	})
}

func searchQueryHandler(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("q")

	results := engine.Find(searchQuery)

	if len(results) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	payload, err := json.Marshal(results)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(payload)
}
