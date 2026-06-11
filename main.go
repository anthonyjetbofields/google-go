package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Repository represents a GitHub repository.
type Repository struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ListOptions specifies the optional parameters to various List methods that support pagination.
type ListOptions struct {
	Page    int
	PerPage int
}

// Response represents a GitHub API response.
type Response struct {
	*http.Response
	NextPage int
}

// Client represents a GitHub API client.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new GitHub API client.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: httpClient,
	}
}

var linkRE = regexp.MustCompile(`\<([^>]+)\>;\s*rel="([^"]+)"`)

func parseLinkHeader(header string) int {
	if header == "" {
		return 0
	}
	for _, part := range strings.Split(header, ",") {
		matches := linkRE.FindStringSubmatch(part)
		if len(matches) == 3 {
			if matches[2] == "next" {
				u, err := url.Parse(matches[1])
				if err != nil {
					return 0
				}
				pageStr := u.Query().Get("page")
				if pageStr != "" {
					page, err := strconv.Atoi(pageStr)
					if err == nil {
						return page
					}
				}
			}
		}
	}
	return 0
}

// ListRepositories fetches a page of repositories for an organization.
func (c *Client) ListRepositories(ctx context.Context, org string, opts *ListOptions) ([]*Repository, *Response, error) {
	u := fmt.Sprintf("%s/orgs/%s/repos", c.BaseURL, org)
	if opts != nil {
		q := url.Values{}
		if opts.Page > 0 {
			q.Set("page", strconv.Itoa(opts.Page))
		}
		if opts.PerPage > 0 {
			q.Set("per_page", strconv.Itoa(opts.PerPage))
		}
		if len(q) > 0 {
			u = fmt.Sprintf("%s?%s", u, q.Encode())
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var repos []*Repository
	if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
		return nil, nil, err
	}

	nextPage := parseLinkHeader(resp.Header.Get("Link"))

	return repos, &Response{Response: resp, NextPage: nextPage}, nil
}

// ListAllRepositories fetches all pages of repositories for an organization, respecting context cancellation.
func (c *Client) ListAllRepositories(ctx context.Context, org string, opts *ListOptions) ([]*Repository, error) {
	if opts == nil {
		opts = &ListOptions{}
	}
	var allRepos []*Repository
	for {
		// Check context cancellation before initiating the HTTP request
		if err := ctx.Err(); err != nil {
			return allRepos, err
		}

		repos, resp, err := c.ListRepositories(ctx, org, opts)
		if err != nil {
			return allRepos, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return allRepos, nil
}

func main() {
	fmt.Println("Hello, Bounty Hunter!")
}
