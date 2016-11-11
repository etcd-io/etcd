/*
Package sling is a Go HTTP client library for creating and sending API requests.

Slings store HTTP Request properties to simplify sending requests and decoding
responses. Check the examples to learn how to compose a Sling into your API
client.

Usage

Use a Sling to create an http.Request with a chained API for setting properties
(path, method, queries, body, etc.).

	type Params struct {
	    Count int `url:"count,omitempty"`
	}
	params := &Params{Count: 5}

	req, err := sling.New().Get("https://example.com").QueryStruct(params).Request()
	client.Do(req)

Path

Use Path to set or extend the URL for created Requests. Extension means the
path will be resolved relative to the existing URL.

	// sends a GET request to http://example.com/foo/bar
	req, err := sling.New().Base("http://example.com/").Path("foo/").Path("bar").Request()

Use Get, Post, Put, Patch, Delete, or Head which are exactly the same as Path
except they set the HTTP method too.

	req, err := sling.New().Post("http://upload.com/gophers")

Headers

Add or Set headers which should be applied to the Requests created by a Sling.

	base := sling.New().Base(baseUrl).Set("User-Agent", "Gophergram API Client")
	req, err := base.New().Get("gophergram/list").Request()

QueryStruct

Define url parameter structs (https://godoc.org/github.com/google/go-querystring/query)
and use QueryStruct to encode query parameters.

	// Github Issue Parameters
	type IssueParams struct {
	    Filter    string `url:"filter,omitempty"`
	    State     string `url:"state,omitempty"`
	    Labels    string `url:"labels,omitempty"`
	    Sort      string `url:"sort,omitempty"`
	    Direction string `url:"direction,omitempty"`
	    Since     string `url:"since,omitempty"`
	}

	githubBase := sling.New().Base("https://api.github.com/").Client(httpClient)
	path := fmt.Sprintf("repos/%s/%s/issues", owner, repo)

	params := &IssueParams{Sort: "updated", State: "open"}
	req, err := githubBase.New().Get(path).QueryStruct(params).Request()

Json Body

Make a Sling include JSON in the Body of its Requests using BodyJSON.

	type IssueRequest struct {
	    Title     string   `json:"title,omitempty"`
	    Body      string   `json:"body,omitempty"`
	    Assignee  string   `json:"assignee,omitempty"`
	    Milestone int      `json:"milestone,omitempty"`
	    Labels    []string `json:"labels,omitempty"`
	}

	githubBase := sling.New().Base("https://api.github.com/").Client(httpClient)
	path := fmt.Sprintf("repos/%s/%s/issues", owner, repo)

	body := &IssueRequest{
	    Title: "Test title",
	    Body:  "Some issue",
	}
	req, err := githubBase.New().Post(path).BodyJSON(body).Request()

Requests will include an "application/json" Content-Type header.

Form Body

Make a Sling include a url-tagged struct as a url-encoded form in the Body of
its Requests using BodyForm.

	type StatusUpdateParams struct {
	    Status             string   `url:"status,omitempty"`
	    InReplyToStatusId  int64    `url:"in_reply_to_status_id,omitempty"`
	    MediaIds           []int64  `url:"media_ids,omitempty,comma"`
	}

	tweetParams := &StatusUpdateParams{Status: "writing some Go"}
	req, err := twitterBase.New().Post(path).BodyForm(tweetParams).Request()

Requests will include an "application/x-www-form-urlencoded" Content-Type
header.

Extend a Sling

Each distinct Sling generates an http.Request (say with some path and query
params) each time Request() is called, based on its state. When creating
different kinds of requests using distinct Slings, you may wish to extend
an existing Sling to minimize duplication (e.g. a common client).

Each Sling instance provides a New() method which creates an independent copy,
so setting properties on the child won't mutate the parent Sling.

	const twitterApi = "https://api.twitter.com/1.1/"
	base := sling.New().Base(twitterApi).Client(httpAuthClient)

	// statuses/show.json Sling
	tweetShowSling := base.New().Get("statuses/show.json").QueryStruct(params)
	req, err := tweetShowSling.Request()

	// statuses/update.json Sling
	tweetPostSling := base.New().Post("statuses/update.json").BodyForm(params)
	req, err := tweetPostSling.Request()

Without the calls to base.New(), tweetShowSling and tweetPostSling reference
the base Sling and POST to
"https://api.twitter.com/1.1/statuses/show.json/statuses/update.json", which
is undesired.

Recap: If you wish to extend a Sling, create a new child copy with New().

Receive

Define a JSON struct to decode a type from 2XX success responses. Use
ReceiveSuccess(successV interface{}) to send a new Request and decode the
response body into successV if it succeeds.

	// Github Issue (abbreviated)
	type Issue struct {
	    Title  string `json:"title"`
	    Body   string `json:"body"`
	}

	issues := new([]Issue)
	resp, err := githubBase.New().Get(path).QueryStruct(params).ReceiveSuccess(issues)
	fmt.Println(issues, resp, err)

Most APIs return failure responses with JSON error details. To decode these,
define success and failure JSON structs. Use
Receive(successV, failureV interface{}) to send a new Request that will
automatically decode the response into the successV for 2XX responses or into
failureV for non-2XX responses.

	type GithubError struct {
	    Message string `json:"message"`
	    Errors  []struct {
	        Resource string `json:"resource"`
	        Field    string `json:"field"`
	        Code     string `json:"code"`
	    } `json:"errors"`
	    DocumentationURL string `json:"documentation_url"`
	}

	issues := new([]Issue)
	githubError := new(GithubError)
	resp, err := githubBase.New().Get(path).QueryStruct(params).Receive(issues, githubError)
	fmt.Println(issues, githubError, resp, err)

Pass a nil successV or failureV argument to skip JSON decoding into that value.
*/
package sling
