package common

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

func (s *BaseService) HttpGet(addr, urlpath string) (rspBody []byte, err error) {
	return SendHttpReq(s.Cfg, "GET", addr, urlpath, nil)
}

func (s *BaseService) HttpPost(addr, urlpath string, reqBody []byte) (rspBody []byte, err error) {
	return SendHttpReq(s.Cfg, "POST", addr, urlpath, reqBody)
}

func (s *BaseService) HttpDelete(addr, urlpath string) (err error) {
	_, err = SendHttpReq(s.Cfg, "DELETE", addr, urlpath, nil)
	return
}

func SendHttpReq(cfg *Config, method, addr, urlpath string, reqBody []byte) (rspBody []byte, err error) {
	var client *http.Client
	schema := "http"
	if cfg.Net.Tls != nil {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = &http.Client{Transport: tr}
		schema = "https"
	} else {
		client = &http.Client{}
	}

	if cfg.Server && !strings.Contains(addr, ":") {
		addr = fmt.Sprintf("%s:%v", addr, cfg.Net.ClientMgntPort)
	}

	url := fmt.Sprintf("%s://%s%s", schema, addr, urlpath)
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cfg.Auth.Username, cfg.Auth.Passowrd)
	req.Header.Set("Content-Type", "application/json")
	//log.Debugf("Sending http request %v", req)

	client.Timeout = 2 * time.Second
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		return nil, errors.New(fmt.Sprintf("Recv http status code %v", resp.StatusCode))
	}

	if resp.ContentLength > 0 {
		rspBody, err = ioutil.ReadAll(resp.Body)
	}
	return
}
