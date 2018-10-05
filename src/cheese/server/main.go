package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type scoreboard struct {
	namesToSolves map[string]map[int64]struct{}
}

func main() {
	var solutionFile string
	var staticDir string
	port := 11000

	flag.StringVar(&staticDir, "static", staticDir, "path to the static dir for the ui")
	flag.IntVar(&port, "port", port, "The port to listen on")
	flag.StringVar(&solutionFile, "solution_file", solutionFile, "path to the file with solutions")
	flag.Parse()

	if solutionFile == "" {
		panic("Must specify solution file")
	}
	if staticDir == "" {
		panic("Must specify static dir")
	}

	f, err := os.Open(solutionFile)
	if err != nil {
		panic(err)
	}

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		panic(err)
	}
	solutions := make(map[int64]int64, 15)
	for _, record := range records {
		if len(record) != 2 {
			continue
		}
		probNum, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			panic(err)
		}

		solution, err := strconv.ParseInt(record[1], 10, 64)
		if err != nil {
			panic(err)
		}
		solutions[probNum] = solution
	}

	// Jank scoreboard data
	s := scoreboard{
		namesToSolves: make(map[string]map[int64]struct{}, 0),
	}

	// Initialize the actual server
	http.HandleFunc("/", staticHandler(staticDir))
	http.HandleFunc("/submit", submitHandler(&s, solutions))
	http.HandleFunc("/scoreboard/", scoreboardHandler(staticDir, &s, solutions))

	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		panic(err)
	}
}

func submitHandler(s *scoreboard, solutions map[int64]int64) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(r.Method)
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form.", 400)
			return
		}

		name := r.PostFormValue("nickname")
		problem := r.PostFormValue("problem")
		answer := r.PostFormValue("answer")

		if name == "" {
			http.Error(w, "you must send a name", 400)
			return
		}

		if problem == "" {
			http.Error(w, "you must send a problem", 400)
			return
		}

		if answer == "" {
			http.Error(w, "you must send an answer", 400)
			return
		}

		problemNum, err := strconv.ParseInt(problem, 10, 64)
		if err != nil {
			http.Error(w, "don't send bad data 1", 400)
			return
		}
		answerNum, err := strconv.ParseInt(answer, 10, 64)
		if err != nil {
			http.Error(w, "don't send bad data 2", 400)
			return
		}

		if realAnswer, ok := solutions[problemNum]; !ok || realAnswer != answerNum {
			http.Redirect(w, r, "/wrong.html", 303)
			return
		}

		if _, ok := s.namesToSolves[name]; !ok {
			s.namesToSolves[name] = make(map[int64]struct{}, 15)
		}
		s.namesToSolves[name][problemNum] = struct{}{}
		http.Redirect(w, r, "/correct.html", 303)
	}
}

func staticHandler(staticDir string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fullPath := staticDir + "/" + r.URL.Path[1:]
		fmt.Println("trying to serve", fullPath)
		http.ServeFile(w, r, fullPath)
	}
}

func scoreboardHandler(
	staticDir string,
	s *scoreboard,
	solutions map[int64]int64,
) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := template.ParseFiles(path.Join(staticDir, "scoreboard.html"))
		if err != nil {
			http.Error(w, "couldn't parse template", 500)
		}

		// Okay, sort the names by num solves
		names := make([]string, 0, len(s.namesToSolves))
		for name := range s.namesToSolves {
			names = append(names, name)
		}

		sort.Slice(names, func(i, j int) bool {
			iSolves := len(s.namesToSolves[names[i]])
			jSolves := len(s.namesToSolves[names[j]])
			if iSolves == jSolves {
				return strings.Compare(names[i], names[j]) < 0
			}
			return iSolves > jSolves
		})

		// Sort the problems by number
		problems := make([]int64, 0, len(solutions))
		for problem := range solutions {
			problems = append(problems, problem)
		}
		sort.Slice(problems, func(i, j int) bool {
			return problems[i] < problems[j]
		})

		// Now we want to generate a table with each of these
		grid := make([][]interface{}, len(names)+2)
		grid[0] = append(grid[0], "Nickname:")
		grid[0] = append(grid[0], "Total:")
		for _, problemNum := range problems {
			grid[0] = append(grid[0], fmt.Sprintf("#%d", problemNum))
		}
		for i, name := range names {
			grid[i+1] = append(grid[i+1], name)
			grid[i+1] = append(grid[i+1], len(s.namesToSolves[name]))
			for _, problemNum := range problems {
				ans := ""
				if _, ok := s.namesToSolves[name][problemNum]; ok {
					ans = "YES"
				}
				grid[i+1] = append(grid[i+1], ans)
			}
		}
		t.Execute(w, grid)
	}
}
