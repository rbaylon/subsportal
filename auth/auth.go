package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

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
	Value  string `json:"value" bson:"value"`
	Type   string `json:"type" bson:"type"`
	Hours  int    `json:"hours" bson:"hours"`
	Status string `json:"status" bson:"status"`
}

type Token struct {
	Name string
	Jwt  string
}

type Access struct {
	Ip   string
	Code Voucher
}

func GetEnvVariable(key string) string {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file", err)
	}
	return os.Getenv(key)
}

func ValidateCode(code string, t *string) (*Voucher, error) {
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "vouchers/value/" + code

	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
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
