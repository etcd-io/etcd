package etcd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"
)

func (c *Client) Delete(key string) (*Response, error) {

	resp, err := c.sendRequest("DELETE", path.Join("keys", key), "")

	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(resp.Body)

	resp.Body.Close()

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, handleError(b)
	}

	var result Response

	err = json.Unmarshal(b, &result)

	if err != nil {
		return nil, err
	}

	return &result, nil

}
