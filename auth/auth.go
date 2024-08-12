package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
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

type Voucher struct {
	Value         string    `json:"value"`
	Type          string    `json:"type"`
	Hours         int       `json:"hours"`
	Status        string    `json:"status"`
	Downspeed     int       `json:"downspeed"`
	Upspeed       int       `json:"upspeed"`
	Burstspeed    int       `json:"burstspeed"`
	Duration      int       `json:"duration"`
	Ip            string    `json:"ip"`
	DateStarted   time.Time `json:"date_started"`
	DateEnd       time.Time `json:"date_end"`
	DateExpires   time.Time `json:"date_expires"`
	HoursConsumed float64   `json:"hours_consumed"`
	PfconfigID    uint      `json:"pfconfig_id"`
}

type Pfiface struct {
	Name       string `json:"name"`
	Speed      string `json:"speed"`
	Device     string `json:"device"`
	Default    bool   `json:"default"`
	Type       string `json:"type"`
	PfconfigID uint   `json:"pfconfig_id"`
}

type Dhcp struct {
	Subnet     string `json:"subnet"`
	Netmask    string `json:"netmask"`
	Routers    string `json:"routers"`
	Dnsservers string `json:"dnsservers"`
	Range      string `json:"range"`
	Type       string `json:"type"`
	PfconfigID uint   `json:"pfconfig_id"`
}

type Sub struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	FramedIp   string `json:"framed_ip"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Mac        string `json:"mac"`
	Loc        string `json:"loc"`
	Downspeed  int    `json:"downspeed"`
	Upspeed    int    `json:"upspeed"`
	Burstspeed int    `json:"burstspeed"`
	Duration   int    `json:"duration"`
	PfConfigID uint   `json:"pfconfig_id"`
}

type PfConfig struct {
	Ifaces            []Pfiface `json:"ifaces"`
	WifiIpList        string    `json:"wifi_ip_list"`
	SubsIpList        string    `json:"subs_ip_list"`
	SubsPortalPort    int       `json:"subs_portal_port"`
	CaptivePortalPort int       `json:"captive_portal_port"`
	Router            string    `json:"router"`
	Vouchers          []Voucher `json:"vouchers"`
	Dhcps             []Dhcp    `json:"dhcps"`
	Subs              []Sub     `json:"subs"`
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
