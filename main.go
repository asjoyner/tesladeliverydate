package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

var (
	loginURL   = "https://www.tesla.com/user/login/?destination=/teslaaccount"
	profileURL = "https://www.tesla.com/teslaaccount/profile?rn="
	configPath = flag.String("config", path.Join(os.Getenv("HOME"), ".tesladeliverydate"), "Path to the JSON formated config file")
	refresh    = flag.Duration("refresh", 60*time.Minute, "How often to check the delivery date")
	userAgent  = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36`
)

// Config describes the JSON format of the config file
type Config struct {
	Username    string
	Password    string
	Reservation string
}

func readConfig(filename string) (*Config, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	if err := json.Unmarshal(b, config); err != nil {
		return nil, err
	}
	if config.Username == "" {
		return nil, errors.New("no username specified")
	}
	if config.Password == "" {
		return nil, errors.New("no password specified")
	}
	if config.Reservation == "" {
		return nil, errors.New("no reservation specified")
	}
	return config, nil
}

func unpackAttrs(attrs []html.Attribute) map[string]string {
	attrMap := make(map[string]string, 0)
	for _, a := range attrs {
		attrMap[a.Key] = a.Val
	}
	return attrMap
}

func login(client *http.Client, config *Config) error {

	glog.Info("requesting login form")
	formReq, err := http.NewRequest("GET", loginURL, nil)
	formReq.Header.Set("User-Agent", userAgent)
	formResp, err := client.Do(formReq)
	if err != nil {
		return fmt.Errorf("fetching login form page: %s", err)
	}
	defer formResp.Body.Close()
	buf := &bytes.Buffer{}
	buf.ReadFrom(formResp.Body)
	doc, err := html.Parse(buf)
	if err != nil {
		return fmt.Errorf("parsing login page: %s", err)
	}

	// recursively parse the HTML doc, node by node
	// find the form with id=form, and copy its hidden input fields
	hiddenFields := make(map[string]string, 0)
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "form" {
				var id string
				for _, a := range n.Attr {
					if a.Key == "id" {
						id = a.Val
					}
				}
				if id != "form" {
					return // skip forms with the wrong id
				}
			}
			if n.Data == "input" {
				attrMap := unpackAttrs(n.Attr)
				if attrMap["type"] == "hidden" {
					hiddenFields[attrMap["name"]] = attrMap["value"]
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Build the login request FORM POST
	formValues := url.Values{}
	for k, v := range hiddenFields {
		formValues.Add(k, v)
	}
	formValues.Add("identity", config.Username)
	formValues.Add("credential", config.Password)

	glog.Infof("logging in as: %s", config.Username)
	loginReq, err := http.NewRequest("POST", formResp.Request.URL.String(), strings.NewReader(formValues.Encode()))
	if err != nil {
		return fmt.Errorf("creating login request: %s", err)
	}
	loginReq.Header.Set("User-Agent", userAgent)
	loginReq.Header.Set("Content-Type", `application/x-www-form-urlencoded`)

	loginResp, err := client.Do(loginReq)
	if err != nil {
		return fmt.Errorf("fetching account page: %s", err)
	}
	defer loginResp.Body.Close()

	buf.Reset()
	buf.ReadFrom(loginResp.Body)
	if !strings.Contains(buf.String(), "Sign Out") {
		return errors.New("account page does not say Sign Out")
	}
	return nil
}

func getDeliveryDate(client *http.Client, reservation string) (string, error) {
	purl := profileURL + reservation
	req, err := http.NewRequest("GET", purl, nil)
	req.Header.Set("User-Agent", userAgent)
	glog.Infof("requesting reservation details for: %s", reservation)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching reservation profile page: %s", err)
	}
	defer resp.Body.Close()

	buf := &bytes.Buffer{}
	buf.ReadFrom(resp.Body)
	bodyString := buf.String()
	searchString := `copyOverride":"Estimated Delivery: `
	loc := strings.Index(bodyString, searchString)
	if loc == -1 {
		return "", fmt.Errorf("no delivery date found")
	}
	if loc+100 > len(bodyString) {
		return "", fmt.Errorf("no delivery date found (short string)")
	}
	dateWithSuffix := bodyString[loc : loc+100]
	dates := strings.Split(dateWithSuffix, `"`)[2]
	return dates, nil
}

func monitorDeliveryDate(config *Config) {
	var deliveryDate string
	for {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			glog.Error("Could not initialize cookieJar: ", err)
			os.Exit(2)
		}

		client := &http.Client{Jar: jar}

		if err := login(client, config); err != nil {
			glog.Error(err)
			time.Sleep(1 * time.Minute)
			continue
		}

		for ; true; <-time.Tick(*refresh) { // tick once, then every *refresh
			d, err := getDeliveryDate(client, config.Reservation)
			if err != nil {
				glog.Error(err)
				time.Sleep(1 * time.Minute)
				break // try logging in again
			}
			if d != deliveryDate {
				deliveryDate = d
				t := time.Now().Format(time.RFC3339)
				fmt.Printf("%s: New Delivery Date! %s\n", t, deliveryDate)
				glog.Warning(deliveryDate)
			} else {
				glog.Info(deliveryDate)
			}
		}
	}
}

func main() {
	flag.Parse()
	config, err := readConfig(*configPath)
	if err != nil {
		glog.Error(err)
		os.Exit(1)
	}

	monitorDeliveryDate(config)
}
