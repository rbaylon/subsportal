package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/rbaylon/captiveportal/auth"
)

var apitoken *string

func main() {
	var (
		app_ip   = auth.GetEnvVariable("APP_IP")
		app_port = auth.GetEnvVariable("APP_PORT")
	)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	files := []string{
		"./templates/base.tmpl",
		"./templates/index.tmpl",
		"./templates/errindex.tmpl",
		"./templates/errbase.tmpl",
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		log.Print(err.Error())
		return
	}
	http.HandleFunc("/", serveTemplate(tmpl))
	token, err := auth.GetToken()
	apitoken = token
	if err != nil {
		log.Println(err)
	}
	log.Printf("%s:%s", app_ip, app_port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%s", app_ip, app_port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func serveTemplate(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("Captured: ", r.RemoteAddr)
		if r.Method != http.MethodPost {
			tmpl.ExecuteTemplate(w, "base", nil)
			return
		}

		data := &auth.Code{
			r.FormValue("one"),
			r.FormValue("two"),
			r.FormValue("three"),
			r.FormValue("four"),
			r.FormValue("five"),
			r.FormValue("six"),
			r.FormValue("seven"),
			r.FormValue("eight"),
		}

		log.Println(data.Joinnum())
		v, err := auth.ValidateCode(data.Joinnum(), apitoken)
		if err != nil {
			log.Println(err.Error())
			tmpl.ExecuteTemplate(w, "errbase", nil)
		}
		log.Println(v)

		//redirect to landing page instead of below
		err = tmpl.ExecuteTemplate(w, "base", nil)
		if err != nil {
			log.Print(err.Error())
			tmpl.ExecuteTemplate(w, "layout", nil)
		}
	}
}
