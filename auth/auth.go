package auth

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
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

// apiTimeout bounds calls to the srvcman API (API_URL) for the same reason
// arkgateTimeout bounds arkgated calls: without it, an unresponsive srvcman
// leaves client.Do blocked forever, leaking the calling goroutine - on the
// request path (ValidateCode) that's one leaked goroutine per guest
// request, and in PfReloader's unrecovered background goroutine it stalls
// pf reloads and token refresh indefinitely with no way to recover.
const apiTimeout = 10 * time.Second

var envOnce sync.Once

// GetEnvVariable loads .env exactly once (not on every call) and reads the
// given key from the process environment. Loading on every call meant any
// transient failure to re-read .env (permissions blip, NFS hiccup, etc.)
// would log.Fatal and kill the whole server on the very next request, since
// serveTemplate calls this for ROUTER_ID on every hit.
func GetEnvVariable(key string) string {
	envOnce.Do(func() {
		if err := godotenv.Load(".env"); err != nil {
			log.Fatal("Error loading .env file", err)
		}
	})
	return os.Getenv(key)
}

func ValidateCode(code string, t *string) string {
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "vouchers/value/" + code
	log.Println(url)
	client := &http.Client{Timeout: apiTimeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err.Error())
		return err.Error()
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
	res, err := client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return err.Error()
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
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

// refreshToken writes a freshly-fetched JWT through t, not into it, so the
// update is visible to every caller sharing that pointer (PfReloader's own
// *t and main.go's apitoken both point at the same string). A plain
// "t = token" reassignment here only rebound this function's local copy of
// the pointer and never reached either of them - the token effectively
// never refreshed for the life of the process.
func refreshToken(t *string) {
	expired, err := CheckExpirationWithoutVerify(*t)
	if err != nil {
		log.Println(err)
		return
	}
	if !expired {
		return
	}
	token, err := GetToken()
	if err != nil {
		log.Println(err)
		return
	}
	*t = *token
	log.Println("Token refreshed")
}

func PfReloader(t *string, lock *bool) {
	startTime = time.Now()
	var (
		api_url = GetEnvVariable("API_URL")
	)
	url := api_url + "runtime/query/updatepf"
	pf := Acmd.GetPFcmds("/tmp/")
	rid := GetEnvVariable("ROUTER_INDEX")
	url = url + "/" + rid
	for {
		refreshToken(t)
		client := &http.Client{Timeout: apiTimeout}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Println(err)
			return
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
		res, autherr := client.Do(req)
		if autherr != nil {
			log.Println(autherr.Error())
			return
		}
		if res.StatusCode == 200 {
			for locker.GetLock(lock, "pfreloader") {
				time.Sleep(50 * time.Millisecond)
			}
			locker.SetLock(lock, true, "pfreloader")
			log.Println("New update found")
			pferr := SendArkgateCmd(pf["check"])
			if pferr == nil {
				log.Println("pf.conf valid")
				time.Sleep(time.Millisecond * 100)
				if err := SendArkgateCmd(pf["backup"]); err != nil {
					log.Println("pf backup failed: ", err)
				}
				time.Sleep(time.Millisecond * 100)
				if err := SendArkgateCmd(pf["move"]); err != nil {
					log.Println("pf move failed: ", err)
				}
				time.Sleep(time.Millisecond * 100)
				pferr = SendArkgateCmd(pf["apply"])
				if pferr != nil {
					time.Sleep(time.Millisecond * 100)
					if err := SendArkgateCmd(pf["revert"]); err != nil {
						log.Println("pf revert failed: ", err)
					}
					log.Println("PF config reverted.")
				} else {
					delreq, err := http.NewRequest("GET", api_url+"runtime/delete/"+rid, nil)
					if err != nil {
						log.Println(err)
					} else {
						delreq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *t))
						delres, upferr := client.Do(delreq)
						if upferr != nil {
							log.Println(upferr.Error())
						} else {
							time.Sleep(time.Millisecond * 100)
							_, _ = io.Copy(io.Discard, delres.Body)
							delres.Body.Close()
						}
					}
				}
			} else {
				log.Println("PF config bad: ", pferr)
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
	client := &http.Client{Timeout: apiTimeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", api_auth))
	res, err := client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return nil, err
	}
	defer res.Body.Close()
	responseData, ioerr := io.ReadAll(res.Body)
	if ioerr != nil {
		return nil, ioerr
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("login failed: status %d: %s", res.StatusCode, responseData)
	}

	var t Token
	if err := json.Unmarshal(responseData, &t); err != nil {
		return nil, fmt.Errorf("parsing login response: %w", err)
	}
	return &t.Jwt, nil
}

// arkgateTLSConfig builds the mTLS client config for talking to arkgated:
// our client cert/key (arkgated authenticates us) plus the CA that signed
// arkgated's server cert (we authenticate it back).
func arkgateTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(GetEnvVariable("ARKGATE_TLS_CERT"), GetEnvVariable("ARKGATE_TLS_KEY"))
	if err != nil {
		return nil, fmt.Errorf("loading arkgate client cert/key: %w", err)
	}
	caPEM, err := os.ReadFile(GetEnvVariable("ARKGATE_TLS_CA"))
	if err != nil {
		return nil, fmt.Errorf("reading arkgate CA cert: %w", err)
	}
	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no certs parsed from arkgate CA file")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// arkgateTimeout bounds how long a single Arkcmd round-trip (connect, write,
// read reply) may take. Without a deadline, a connection to an arkgated that
// accepts the connection but never replies (hung, overloaded, or a bad
// actor) leaves cmd.SendCmd blocked in conn.Read forever - its
// "defer conn.Close()" never runs, so the goroutine and its socket never
// get released. validateCode alone can open up to 5 of these per voucher
// submission, each on its own request goroutine, so that leak is unbounded
// under load. Setting a deadline here guarantees Read/Write eventually fail
// and SendCmd returns, closing the connection.
const arkgateTimeout = 5 * time.Second

// GetArkgateConn connects to arkgated's IPC listener, preferring the local
// Unix domain socket when ARKGATE_SOCKET is set (subsportal and arkgated
// running on the same host - no PKI material needed, trust is filesystem
// permissions) and falling back to mutual TLS over the network otherwise,
// which is what lets arkgated run on a separate host.
func GetArkgateConn() net.Conn {
	if sockPath := GetEnvVariable("ARKGATE_SOCKET"); sockPath != "" {
		c, err := net.Dial("unix", sockPath)
		if err == nil {
			return withArkgateDeadline(c)
		}
		log.Println("arkgate unix socket dial error, falling back to mTLS: ", err)
	}

	tlsConfig, err := arkgateTLSConfig()
	if err != nil {
		log.Println("arkgate TLS config error: ", err)
		return nil
	}
	c, err := tls.Dial("tcp", GetEnvVariable("ARKGATE_ADDR"), tlsConfig)
	if err != nil {
		log.Println("Dial error ", err)
		return nil
	}
	return withArkgateDeadline(c)
}

// withArkgateDeadline applies arkgateTimeout to conn so a hung arkgated
// can't block its caller (and leak the connection) indefinitely.
func withArkgateDeadline(conn net.Conn) net.Conn {
	if err := conn.SetDeadline(time.Now().Add(arkgateTimeout)); err != nil {
		log.Println("arkgate set deadline error: ", err)
		conn.Close()
		return nil
	}
	return conn
}

// SendArkgateCmd dials arkgated over mTLS and sends cmd over it, returning
// an error instead of letting cmd.SendCmd panic on a nil connection when the
// daemon isn't reachable (e.g. arkgated is down or restarting - this is
// exactly what happened in the 2026-09-05 incident: arkgated crashed, and
// every caller here that used cmd.SendCmd(GetArkgateConn()) directly panicked
// on the nil conn instead of just logging and moving on).
func SendArkgateCmd(cmd *Acmd.Arkcmd) error {
	conn := GetArkgateConn()
	if conn == nil {
		return fmt.Errorf("arkgated unavailable")
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
