package main

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	Acmd "github.com/rbaylon/arkgatecmd"
	"github.com/rbaylon/subsportal/auth"
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
	}

	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/", serveTemplate(tmpl, &locker.Lock))

	token, err := auth.GetToken()
	if err != nil {
		// apitoken stays nil below if we don't bail here, and every
		// subsequent request/reload would dereference it and panic -
		// fail fast instead and let the rc.d/daemon_manager watchdog
		// restart us once the API is reachable again.
		log.Fatal(err)
	}
	apitoken = token
	go auth.PfReloader(apitoken, &locker.Lock)
	log.Printf("%s:%s", app_ip, app_port)
	err = http.ListenAndServe(fmt.Sprintf("%s:%s", app_ip, app_port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

func validateCode(urlsuffix string, token *string, lock *bool) error {
	if token == nil {
		return fmt.Errorf("no api token available")
	}
	result := auth.ValidateCode(urlsuffix, token)
	if result == "NotFound" {
		log.Println("Code error: Not Found")
		return fmt.Errorf("code error: not found")
	}
	for locker.GetLock(lock, "voucher") {
		time.Sleep(50 * time.Millisecond)
	}
	locker.SetLock(lock, true, "voucher")
	pf := Acmd.GetPFcmds("/tmp/")
	err := auth.SendArkgateCmd(pf["check"])
	if err == nil {
		log.Println("pf.conf valid")
		time.Sleep(time.Millisecond * 100)
		if err := auth.SendArkgateCmd(pf["backup"]); err != nil {
			log.Println("pf backup failed: ", err)
		}
		time.Sleep(time.Millisecond * 100)
		if err := auth.SendArkgateCmd(pf["move"]); err != nil {
			log.Println("pf move failed: ", err)
		}
		time.Sleep(time.Millisecond * 100)
		err = auth.SendArkgateCmd(pf["apply"])
		if err != nil {
			time.Sleep(time.Millisecond * 100)
			if rerr := auth.SendArkgateCmd(pf["revert"]); rerr != nil {
				log.Println("pf revert failed: ", rerr)
			}
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
		remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			log.Println("Could not parse remote address ", r.RemoteAddr, ": ", err)
			remoteIP = r.RemoteAddr
		}
		routerid := auth.GetEnvVariable("ROUTER_ID")
		log.Println("Captured: ", remoteIP)
		type errmsg struct {
			Message string
		}
		msg := errmsg{Message: ""}
		if r.Method != http.MethodPost {
			cookie, err := r.Cookie("code")
			if err != nil {
				if err == http.ErrNoCookie {
					log.Println("No cookie found!")
				} else {
					log.Println("Error retrieving cookie: ", err)
				}
			} else {
				log.Println("Cookie found, validating access")
				urlsuffix := url.QueryEscape(cookie.Value) + "/" + url.QueryEscape(remoteIP) + "/" + routerid
				cerr := validateCode(urlsuffix, apitoken, lock)
				if cerr == nil {
					log.Println("Success")
					http.Redirect(w, r, "https://www.google.com", http.StatusSeeOther)
					return
				}
				log.Println("Cookie invalid: ", cerr)
				expire := time.Now().Add(-7 * 24 * time.Hour)
				expiredCookie := http.Cookie{
					Name:     "code",
					Value:    "",
					HttpOnly: true,
					Expires:  expire,
				}
				http.SetCookie(w, &expiredCookie)
			}
			if err := tmpl.ExecuteTemplate(w, "base", &msg); err != nil {
				log.Println("Template execution error: ", err)
			}
			return
		}
		code := strings.TrimSpace(r.FormValue("voucher"))
		log.Println(code)
		if code == "" {
			log.Println("Empty voucher code submitted")
			msg := errmsg{Message: "yes"}
			if err := tmpl.ExecuteTemplate(w, "base", &msg); err != nil {
				log.Println("Template execution error: ", err)
			}
			return
		}
		urlsuffix := url.QueryEscape(code) + "/" + url.QueryEscape(remoteIP) + "/" + routerid
		cerr := validateCode(urlsuffix, apitoken, lock)
		if cerr != nil {
			log.Println(cerr)
			msg := errmsg{Message: "yes"}
			if err := tmpl.ExecuteTemplate(w, "base", &msg); err != nil {
				log.Println("Template execution error: ", err)
			}
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
