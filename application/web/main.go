package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

type PageData struct {
	Counter    CounterData
	Double     CounterData
	TodosCount CounterData
	Todos      map[string]Todo
}

type CounterData struct {
	Endpoint string
	Value    uint
}

type Todo struct {
	Id          string
	Title       string
	Description string
}

type SsEvent struct {
	Id   string
	Name string
	Data string
}

func (e SsEvent) String() string {
	return fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", e.Id, e.Name, e.Data)
}

type SseBroker struct {
	Notifier       chan SsEvent
	OpeningClients chan chan SsEvent
	ClosingClients chan chan SsEvent
	Clients        map[chan SsEvent]bool
}

func NewSseBroker() *SseBroker {
	broker := new(SseBroker)
	broker.Notifier = make(chan SsEvent)
	broker.OpeningClients = make(chan chan SsEvent)
	broker.ClosingClients = make(chan chan SsEvent)
	broker.Clients = make(map[chan SsEvent]bool)
	return broker
}

func (sb *SseBroker) Listen() {
	for {
		select {
		case c := <-sb.OpeningClients:
			sb.Clients[c] = true
			log.Printf("Client added. %d registered clients\n", len(sb.Clients))
		case c := <-sb.ClosingClients:
			delete(sb.Clients, c)
			log.Printf("Removed client. %d registered clients\n", len(sb.Clients))
		case event := <-sb.Notifier:
			for clientChannel := range sb.Clients {
				clientChannel <- event
			}
		}
	}
}

func main() {
	sseBroker := NewSseBroker()
	go sseBroker.Listen()

	var counter uint = 0
	todos := map[string]Todo{
		uuid.NewString(): {Id: uuid.NewString(), Title: "Do this"},
		uuid.NewString(): {Id: uuid.NewString(), Title: "Do that"},
	}

	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.RedirectSlashes)

	templ := template.Must(template.ParseFiles(
		"template/index.html",
		"template/htmx.html",
		"template/counter.html",
		"template/todos.html",
	))

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		pd := PageData{
			Counter:    CounterData{Endpoint: "/counter", Value: counter},
			Double:     CounterData{Endpoint: "/counter?m=2", Value: counter * 2},
			TodosCount: CounterData{Endpoint: "/todos/count", Value: uint(len(todos))},
			Todos:      todos,
		}

		templ.Execute(w, pd)
	})

	router.Post("/counter", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		v, _ := strconv.Atoi(r.Form.Get("value"))

		if v <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		counter += uint(v)

		sseBroker.Notifier <- SsEvent{Id: uuid.NewString(), Name: "update-counter"}

		w.Header().Set("HX-Trigger", "update-counter")
		w.WriteHeader(http.StatusOK)
	})

	router.Get("/counter", func(w http.ResponseWriter, r *http.Request) {
		multiplier := r.URL.Query().Get("m")
		cd := CounterData{Endpoint: "/counter", Value: counter}
		if multiplier != "" {
			cd.Endpoint = fmt.Sprintf("/counter?m=%s", multiplier)
			m, _ := strconv.Atoi(multiplier)
			cd.Value = counter * uint(m)
		}
		templ.ExecuteTemplate(w, "counter", cd)
	})

	router.Post("/todos", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		todos[uuid.NewString()] = Todo{Id: uuid.NewString(), Title: r.Form.Get("title")}
		sseBroker.Notifier <- SsEvent{Id: uuid.NewString(), Name: "update-todos"}
		w.Header().Set("HX-Trigger", "update-todos")
		w.WriteHeader(http.StatusOK)
	})

	router.Delete("/todos/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		delete(todos, id)

		sseBroker.Notifier <- SsEvent{Id: uuid.NewString(), Name: "update-todos"}

		w.Header().Set("HX-Trigger", "update-todos")
		w.WriteHeader(http.StatusOK)
	})

	router.Get("/todos", func(w http.ResponseWriter, r *http.Request) {
		templ.ExecuteTemplate(w, "todos", todos)
	})

	router.Get("/todos/count", func(w http.ResponseWriter, r *http.Request) {
		cd := CounterData{Endpoint: "/todos/count", Value: uint(len(todos))}
		templ.ExecuteTemplate(w, "counter", cd)
	})

	router.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)

		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := make(chan SsEvent)
		sseBroker.OpeningClients <- ch

		defer func() {
			sseBroker.ClosingClients <- ch
		}()

		go func() {
			<-r.Context().Done()
			sseBroker.ClosingClients <- ch
		}()

		for {
			fmt.Fprintf(w, "%s", <-ch)
			flusher.Flush()
		}
	})

	http.ListenAndServe(":3333", router)
}
