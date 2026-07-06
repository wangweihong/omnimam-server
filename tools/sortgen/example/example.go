package main

// +gen:sortfields
type Manifets struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        ImageConfig  `json:"config"`
	Layers        []ImageLayer `json:"layers"`
	Embed
}

type Embed struct {
	Name string
}

type ImageConfig struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

type ImageLayer struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}
