package main

import (
	"html/template"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type PageData struct {
    Counter uint
    Double uint
}

func main() {
    var counter uint = 0

    router := chi.NewRouter()

    router.Use(middleware.Logger)
    router.Use(middleware.RedirectSlashes)
 
    templ := template.Must(template.ParseFiles(
        "application/web/template/index.html",
        "application/web/template/htmx.html",
        "application/web/template/counter.html",
        "application/web/template/double.html",
    ))

    router.Get("/", func(w http.ResponseWriter, r *http.Request) {
        pd := PageData{
            Counter: counter,
            Double: counter*2,
        }
        
        templ.Execute(w, pd)
    })

    router.Post("/update-counter", func(w http.ResponseWriter, r *http.Request) {
        r.ParseForm()
        v, _ := strconv.Atoi(r.Form["value"][0])
        counter += uint(v)
        w.Header().Set("HX-Trigger", "update-counter")
        w.Write([]byte{})
    })

    router.Get("/counter",func(w http.ResponseWriter, r *http.Request) {
        templ.ExecuteTemplate(w, "counter", counter)
    }) 

    router.Get("/double-counter",func(w http.ResponseWriter, r *http.Request) {
        templ.ExecuteTemplate(w, "double", counter*2)
    })

    http.ListenAndServe(":3333", router)
}

