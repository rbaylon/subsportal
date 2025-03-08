package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rbaylon/subsportal/auth"
	"github.com/rbaylon/subsportal/cmd"
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
	go auth.PfReloader(apitoken)
	log.Printf("%s:%s", app_ip, app_port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%s", app_ip, app_port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func serveTemplate(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remote := strings.Split(r.RemoteAddr, ":")
		log.Println("Captured: ", remote[0])
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
		routerid := auth.GetEnvVariable("ROUTER_ID")
		urlsuffix := url.QueryEscape(data.Joinnum()) + "/" + url.QueryEscape(remote[0]) + "/" + routerid
		result := auth.ValidateCode(urlsuffix, apitoken)
		if result == "NotFound" {
			log.Println("Code error: Not Found")
			tmpl.ExecuteTemplate(w, "errbase", nil)
			return
		}
		pf := cmd.GetPFcmds(auth.GetEnvVariable("RUN_DIR"))
		err := pf["check"].SendCmd(auth.GetUnixConn())
		if err == nil {
			log.Println("pf.conf valid")
			time.Sleep(time.Millisecond * 100)
			pf["backup"].SendCmd(auth.GetUnixConn())
			time.Sleep(time.Millisecond * 100)
			pf["move"].SendCmd(auth.GetUnixConn())
			time.Sleep(time.Millisecond * 100)
			err = pf["apply"].SendCmd(auth.GetUnixConn())
			if err != nil {
				time.Sleep(time.Millisecond * 100)
				pf["revert"].SendCmd(auth.GetUnixConn())
				log.Println("PF config reverted.")
			}
		} else {
			log.Println("PF config bad: ", err)
			//ToDo: send sms alert
		}
		time.Sleep(time.Millisecond * 100)
		//redirect to landing page instead of below
		http.Redirect(w, r, "https://www.google.com", http.StatusSeeOther)
	}
}
