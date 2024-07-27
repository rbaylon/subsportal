package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func GetEnvVariable(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	return os.Getenv(key)
}

type Code struct {
	One   string
	Two   string
	Three string
	Four  string
	Five  string
	Six   string
	Seven string
	Eight string
}

type Voucher struct {
	Value  string `json:"value" bson:"value"`
	Type   string `json:"type" bson:"type"`
	Hours  int    `json:"hours" bson:"hours"`
	Status string `json:"status" bson:"status"`
}

type Token struct {
	Name string
	Jwt  string
}

func (c *Code) Joinnum() string {
	return fmt.Sprintf("%s%s%s%s%s%s%s%s", c.One, c.Two, c.Three, c.Four, c.Five, c.Six, c.Seven, c.Eight)
}

var apitoken *string

func getToken() (*string, error) {
	var (
		api_auth = GetEnvVariable("API_AUTH")
		api_url  = GetEnvVariable("API_URL")
	)
	url := api_url + "login"
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", api_auth))
	res, _ := client.Do(req)
	responseData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var t Token
	json.Unmarshal(responseData, &t)
	return &t.Jwt, nil
}

func main() {
	var (
		app_ip   = GetEnvVariable("APP_IP")
		app_port = GetEnvVariable("APP_PORT")
	)
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.HandleFunc("/", serveTemplate)
	token, err := getToken()
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

func serveTemplate(w http.ResponseWriter, r *http.Request) {
	lp := filepath.Join("templates", "layout.html")
	fp := filepath.Join("templates", filepath.Clean(r.URL.Path))

	// Return a 404 if the template doesn't exist
	info, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
	}

	// Return a 404 if the request is for a directory
	if info.IsDir() {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFiles(lp, fp)
	if err != nil {
		// Log the detailed error
		log.Print(err.Error())
		// Return a generic "Internal Server Error" message
		http.Error(w, http.StatusText(500), 500)
		return
	}

	if r.Method != http.MethodPost {
		tmpl.ExecuteTemplate(w, "layout", nil)
		return
	}

	data := &Code{
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
	v, err := ValidateCode(data.Joinnum())
	if err != nil {
		log.Println(err.Error())
		http.Redirect(w, r, "/error.html", http.StatusSeeOther)
	}
	log.Println(v)

	//redirect to landing page instead of below
	err = tmpl.ExecuteTemplate(w, "layout", nil)
	if err != nil {
		log.Print(err.Error())
		tmpl.ExecuteTemplate(w, "layout", nil)
	}
}

func ValidateCode(code string) (*Voucher, error) {
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "vouchers/value/" + code

	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *apitoken))
	res, _ := client.Do(req)
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("Record Not Found")
	}
	responseData, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var v Voucher
	json.Unmarshal(responseData, &v)
	log.Println("Received value: ", v.Value)
	return &v, nil
}
