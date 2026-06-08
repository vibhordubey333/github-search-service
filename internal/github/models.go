package github

type SearchResponse struct {
	TotalCount        int          `json:"total_count"`
	IncompleteResults bool         `json:"incomplete_results"`
	Items             []SearchItem `json:"items"`
}

type SearchItem struct {
	HTMLURL    string     `json:"html_url"`
	Repository Repository `json:"repository"`
}

type Repository struct {
	FullName string `json:"full_name"`
}

type SearchParams struct {
	Query   string
	PerPage int
	Page    int
}
