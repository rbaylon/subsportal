package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/rbaylon/subsportal/cmd"
	"github.com/rbaylon/subsportal/locker"
)

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

func (c *Code) Joinnum() string {
	return fmt.Sprintf("%s%s%s%s%s%s%s%s", c.One, c.Two, c.Three, c.Four, c.Five, c.Six, c.Seven, c.Eight)
}

type Token struct {
	Name string
	Jwt  string
}

func GetEnvVariable(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	return os.Getenv(key)
}

func ValidateCode(code string, t *string) string {
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "vouchers/value/" + code
	log.Println(url)
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
	res, _ := client.Do(req)
	defer res.Body.Close()
	v := "active"
	if res.StatusCode == 404 {
		v = "NotFound"
	}
	if res.StatusCode == 201 {
		v = "activated"
	}
	if res.StatusCode == 202 {
		v = "updated"
	}
	log.Println("Code validated")
	return v
}

func PfReloader(t *string, lock *bool) {
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "runtime/query/updatepf"
	pf := cmd.GetPFcmds(GetEnvVariable("RUN_DIR"))
	for {
		client := &http.Client{}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
		res, _ := client.Do(req)
		if res.StatusCode == 200 {
			for locker.GetLock(lock, "pfreloader") {
				time.Sleep(50 * time.Millisecond)
			}
			locker.SetLock(lock, true, "pfreloader")
			log.Println("New update found")
			err := pf["check"].SendCmd(GetUnixConn())
			if err == nil {
				log.Println("pf.conf valid")
				time.Sleep(time.Millisecond * 100)
				pf["backup"].SendCmd(GetUnixConn())
				time.Sleep(time.Millisecond * 100)
				pf["move"].SendCmd(GetUnixConn())
				time.Sleep(time.Millisecond * 100)
				err = pf["apply"].SendCmd(GetUnixConn())
				if err != nil {
					time.Sleep(time.Millisecond * 100)
					pf["revert"].SendCmd(GetUnixConn())
					log.Println("PF config reverted.")
				} else {
					delreq, _ := http.NewRequest("GET", api_url+"runtime/delete/updatepf", nil)
					delreq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
					delres, _ := client.Do(delreq)
					time.Sleep(time.Millisecond * 100)
					delres.Body.Close()
				}
			} else {
				log.Println("PF config bad: ", err)
				//ToDo: send sms alert
			}
			locker.SetLock(lock, false, "pfreloader")
		}
		res.Body.Close()
		time.Sleep(120 * time.Second)
	}
}

func GetToken() (*string, error) {
	var (
		api_auth = GetEnvVariable("API_AUTH")
		api_url  = GetEnvVariable("API_URL")
	)
	url := api_url + "login"
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", api_auth))
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}
	responseData, ioerr := ioutil.ReadAll(res.Body)
	if ioerr != nil {
		return nil, ioerr
	}

	var t Token
	json.Unmarshal(responseData, &t)
	return &t.Jwt, nil
}

func GetUnixConn() net.Conn {
	c, err := net.Dial("unix", GetEnvVariable("UNIX_SOCK"))
	if err != nil {
		log.Println("Dial error ", err)
	}
	return c
}
