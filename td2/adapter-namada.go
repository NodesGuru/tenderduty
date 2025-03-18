package tenderduty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"time"
)

type NamadaAdapter struct {
	ChainConfig *ChainConfig
}

func getVotingPeriodProposalIds(httpClient *http.Client, indexers []string) ([]string, error) {
	// Store the last error to return if all indexer endpoints fail
	var lastErr error

	// Prepare query parameters
	params := url.Values{}
	params.Add("status", "votingPeriod")

	// Slice to store proposal IDs
	votingPeriodProposalIds := []string{}

	// Try each indexer in the list
	for _, indexer := range indexers {
		reqURL := fmt.Sprintf("%s/api/v1/gov/proposal?%s", indexer, params.Encode())

		// Make the HTTP request
		req, err := http.NewRequest("GET", reqURL, nil)
		if err != nil {
			lastErr = err
			continue // Try next node
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue // Try next node
		}

		func() {
			defer resp.Body.Close()

			var respJson map[string]any
			if err = json.NewDecoder(resp.Body).Decode(&respJson); err != nil {
				lastErr = err
				return
			}

			if results, ok := respJson["results"].([]any); ok {
				for _, proposal := range results {
					if vMap, ok := proposal.(map[string]any); ok {
						if id, ok := vMap["id"].(string); ok && !slices.Contains(votingPeriodProposalIds, id) {
							votingPeriodProposalIds = append(votingPeriodProposalIds, id)
						}
					}
				}
			}
		}()

		// If we found proposals with this node, return them
		if len(votingPeriodProposalIds) > 0 {
			return votingPeriodProposalIds, nil
		}
	}

	return votingPeriodProposalIds, lastErr
}

func (d *NamadaAdapter) CountUnvotedOpenProposals(ctx context.Context) (int, error) {
	// Store the last error to return if all indexer endpoints fail
	var lastErr error
	unVotedProposalIds := []string{}

	indexers, ok1 := d.ChainConfig.Adapter.Configs["indexers"].([]any)
	validatorAddress, ok2 := d.ChainConfig.Adapter.Configs["validator_address"].(string)
	if ok1 && ok2 {
		// Create a reusable HTTP client with timeout
		httpClient := &http.Client{
			Timeout: 5 * time.Second, // Add reasonable timeout
		}

		urls := make([]string, len(indexers))
		for i, v := range indexers {
			if str, ok := v.(string); ok {
				urls[i] = str
			}
		}

		votingPeriodProposalIds, err := getVotingPeriodProposalIds(httpClient, urls)
		votedProposalIds := []string{}
		if err != nil {
			return 0, err
		}

		// check voting results using different indexers
		for _, indexer := range indexers {
			reqURL := fmt.Sprintf("%s/api/v1/gov/voter/%s/votes", indexer, validatorAddress)

			// Make the HTTP request with context
			req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
			if err != nil {
				lastErr = err
				continue // Try next node
			}

			resp, err := httpClient.Do(req)
			if err != nil {
				lastErr = err
				continue // Try next node
			}

			// Use defer in a function to ensure it's called before continuing the loop
			func() {
				defer resp.Body.Close()

				var results []map[string]any
				if err = json.NewDecoder(resp.Body).Decode(&results); err != nil {
					lastErr = err
					return // Exit this func, continue loop
				}

				// check the voting results
				for _, vote := range results {
					if id, ok := vote["proposalId"].(float64); ok && !slices.Contains(votedProposalIds, strconv.Itoa(int(id))) {
						votedProposalIds = append(votedProposalIds, strconv.Itoa(int(id)))
					}
				}
			}()
		}

		for _, id := range votingPeriodProposalIds {
			if !slices.Contains(votedProposalIds, id) {
				unVotedProposalIds = append(unVotedProposalIds, id)
			}
		}
	}

	return len(unVotedProposalIds), lastErr
}
