package common

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
)

func (s *BaseService) HttpGet(addr, urlpath string) (rspBody []byte, err error) {
	return sendHttpReq(s.cfg, "GET", addr, urlpath, nil)
}

func (s *BaseService) HttpPost(addr, urlpath string, reqBody []byte) (rspBody []byte, err error) {
	return sendHttpReq(s.cfg, "POST", addr, urlpath, reqBody)
}

func (s *BaseService) HttpDelete(addr, urlpath string) (err error) {
	_, err = sendHttpReq(s.cfg, "DELETE", addr, urlpath, nil)
	return
}

func sendHttpReq(cfg *Config, method, addr, urlpath string, reqBody []byte) (rspBody []byte, err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := fmt.Sprintf("https://%s/%s", addr, urlpath)
	req, err := http.NewRequest(method, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(cfg.Auth.Username, cfg.Auth.Passowrd)

	resp, err := client.Do(req)
	defer resp.Body.Close()

	if resp.StatusCode > 300 {
		return nil, errors.New(fmt.Sprintf("Recv http status code %v", resp.StatusCode))
	}

	if resp.ContentLength > 0 {
		rspBody, err = ioutil.ReadAll(resp.Body)
	}
	return
}
