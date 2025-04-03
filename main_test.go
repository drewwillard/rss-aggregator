package main

import (
	"context"
	"testing"
)

func TestFeedFetcher(t *testing.T) {
	rssStruct, err := fetchFeed(context.Background(), "https://www.nasa.gov/rss/dyn/breaking_news.rss")
	if err != nil {
		t.Errorf("issue with feed fetching function: %v", err)
	}
	if rssStruct == nil {
		t.Error("feed that was fetched is blank")
	}
}
