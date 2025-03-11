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
	"github.com/rbaylon/subsportal/locker"
)

var apitoken *string

func main() {
	var (
		app_ip   = auth.GetEnvVariable("APP_IP")
		app_port = auth.GetEnvVariable("APP_PORT")
	)
	locker.Lock = false
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
	http.HandleFunc("/", serveTemplate(tmpl, &locker.Lock))
	token, err := auth.GetToken()
	apitoken = token
	if err != nil {
		log.Println(err)
	}
	go auth.PfReloader(apitoken, &locker.Lock)
	log.Printf("%s:%s", app_ip, app_port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%s", app_ip, app_port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func validateCode(urlsuffix string, token *string, lock *bool) error {
	result := auth.ValidateCode(urlsuffix, token)
	if result == "NotFound" {
		log.Println("Code error: Not Found")
		return fmt.Errorf("Code error: Not Found")
	}
	for locker.GetLock(lock, "voucher") {
		time.Sleep(50 * time.Millisecond)
	}
	locker.SetLock(lock, true, "voucher")
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
	locker.SetLock(lock, false, "voucher")
	time.Sleep(time.Millisecond * 100)
	return nil
}

func serveTemplate(tmpl *template.Template, lock *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		remote := strings.Split(r.RemoteAddr, ":")
		routerid := auth.GetEnvVariable("ROUTER_ID")
		log.Println("Captured: ", remote[0])
		if r.Method != http.MethodPost {
			cookie, err := r.Cookie("code")
			if err != nil {
				if err == http.ErrNoCookie {
					log.Println("No cookie found!")
				} else {
					log.Println("Error retrieving cookie")
				}
			} else {
				urlsuffix := url.QueryEscape(cookie.Value) + "/" + url.QueryEscape(remote[0]) + "/" + routerid
				cerr := validateCode(urlsuffix, apitoken, lock)
				if cerr == nil {
					http.Redirect(w, r, "https://www.google.com", http.StatusSeeOther)
					return
				}
			}
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
		code := data.Joinnum()
		log.Println(code)
		urlsuffix := url.QueryEscape(code) + "/" + url.QueryEscape(remote[0]) + "/" + routerid
		cerr := validateCode(urlsuffix, apitoken, lock)
		if cerr != nil {
			log.Println(cerr)
			tmpl.ExecuteTemplate(w, "errbase", nil)
			return
		}
		expiration := time.Now().Add(32 * 24 * time.Hour)
		cookie := http.Cookie{
			Name:     "code",
			Value:    code,
			HttpOnly: true,
			Expires:  expiration,
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "https://www.google.com", http.StatusSeeOther)
	}
}
