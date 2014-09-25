package etcdhttp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func SendAddMemberRequest(endpoint string, id int64, peerURLs []string) error {
	u := endpoint + adminMembersPrefix + strconv.FormatInt(id, 10)
	form := url.Values{}
	form["PeerURLs"] = peerURLs
	body := strings.NewReader(form.Encode())
	req, err := http.NewRequest("PUT", u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	return nil
}

func SendRemoveMemberRequest(endpoint string, id int64) error {
	u := endpoint + adminMembersPrefix + strconv.FormatInt(id, 10)
	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	return nil
}
