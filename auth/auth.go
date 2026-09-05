package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	Acmd "github.com/rbaylon/arkgatecmd"
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
	res, err := client.Do(req)
	if err != nil {
		res.Body.Close()
		log.Println(err.Error())
		return err.Error()
	}
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

var startTime time.Time

func refreshToken(t *string) {
	expired, err := CheckExpirationWithoutVerify(*t)
	if err != nil {
		log.Println(err)
	}
	if expired {
		token, err := GetToken()
		if err != nil {
			log.Println(err)
		}
		log.Println("Token refreshed")
		t = token
	}
}

func PfReloader(t *string, lock *bool) {
	startTime = time.Now()
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "runtime/query/updatepf"
	pf := Acmd.GetPFcmds(GetEnvVariable("RUN_DIR"))
	rid := GetEnvVariable("ROUTER_INDEX")
	url = url + "/" + rid
	for {
		refreshToken(t)
		client := &http.Client{}
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
		res, autherr := client.Do(req)
		if autherr != nil {
			res.Body.Close()
			log.Println(autherr.Error())
			return
		}
		if res.StatusCode == 200 {
			for locker.GetLock(lock, "pfreloader") {
				time.Sleep(50 * time.Millisecond)
			}
			locker.SetLock(lock, true, "pfreloader")
			log.Println("New update found")
			err := SendUnixCmd(pf["check"])
			if err == nil {
				log.Println("pf.conf valid")
				time.Sleep(time.Millisecond * 100)
				SendUnixCmd(pf["backup"])
				time.Sleep(time.Millisecond * 100)
				SendUnixCmd(pf["move"])
				time.Sleep(time.Millisecond * 100)
				err = SendUnixCmd(pf["apply"])
				if err != nil {
					time.Sleep(time.Millisecond * 100)
					SendUnixCmd(pf["revert"])
					log.Println("PF config reverted.")
				} else {
					delreq, _ := http.NewRequest("GET", api_url+"runtime/delete/"+rid, nil)
					delreq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
					delres, upferr := client.Do(delreq)
					if upferr != nil {
						delres.Body.Close()
						log.Println(upferr.Error())
					}
					time.Sleep(time.Millisecond * 100)
					_, _ = io.Copy(io.Discard, delres.Body)
					delres.Body.Close()
				}
			} else {
				log.Println("PF config bad: ", err)
				//ToDo: send sms alert
			}
			locker.SetLock(lock, false, "pfreloader")
		}
		_, _ = io.Copy(io.Discard, res.Body)
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
	if err != nil {
		res.Body.Close()
		log.Println(err.Error())
		return nil, err
	}
	defer res.Body.Close()
	responseData, ioerr := io.ReadAll(res.Body)
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
		return nil
	}
	return c
}

// SendUnixCmd dials arkgated's Unix socket and sends cmd over it, returning
// an error instead of letting cmd.SendCmd panic on a nil connection when the
// socket isn't reachable (e.g. arkgated is down or restarting - this is
// exactly what happened in the 2026-09-05 incident: arkgated crashed, and
// every caller here that used cmd.SendCmd(GetUnixConn()) directly panicked
// on the nil conn instead of just logging and moving on).
func SendUnixCmd(cmd *Acmd.Arkcmd) error {
	conn := GetUnixConn()
	if conn == nil {
		return fmt.Errorf("arkgated unix socket unavailable")
	}
	return cmd.SendCmd(conn)
}

func CheckExpirationWithoutVerify(tokenStr string) (bool, error) {
	parser := jwt.NewParser()
	var claims jwt.MapClaims

	// Parse unverified explicitly skips signature validation
	_, _, err := parser.ParseUnverified(tokenStr, &claims)
	if err != nil {
		return false, err
	}

	// Extract the standard 'exp' claim safely
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return false, fmt.Errorf("failed to get expiration: %w", err)
	}

	if exp == nil {
		return false, fmt.Errorf("exp claim is missing from token")
	}

	// Compare token expiration timestamp with current system time
	isExpired := exp.Before(time.Now())
	return isExpired, nil
}
