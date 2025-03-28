package tenderduty

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	gov "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type DefaultAdapter struct {
	ChainConfig *ChainConfig
}

func (d *DefaultAdapter) CheckIfValidatorVoted(ctx context.Context, proposalID uint64, accAddress string) (bool, error) {
	params := url.Values{}
	query := fmt.Sprintf("\"proposal_vote.proposal_id='%d' AND proposal_vote.voter='%s'\"", proposalID, accAddress)
	params.Add("query", query)
	params.Add("prove", "false")
	params.Add("page", "1")
	params.Add("per_page", "1")

	// Create a reusable HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Second, // Add reasonable timeout
	}

	// Store the last error to return if all nodes fail
	var lastErr error

	// Try each node in the list until we find a vote or exhaust all options
	for _, node := range d.ChainConfig.Nodes {
		reqURL := fmt.Sprintf("%s/tx_search?%s", node.Url, params.Encode())

		// Make the HTTP request with context
		req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
		if err != nil {
			lastErr = err
			continue // Try next node
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue // Try next node
		}

		// Use defer in a function to ensure it's called before continuing the loop
		found := false
		func() {
			defer resp.Body.Close()

			// check for existence of txs
			var result map[string]any
			if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
				lastErr = err
				return // Exit this func, continue loop
			}

			// Navigate the JSON structure to check if txs exist
			if resultObj, ok := result["result"].(map[string]any); ok {
				if txs, ok := resultObj["txs"].([]any); ok && len(txs) > 0 {
					// Set found to true so we return true outside the loop
					found = true
				}
			}
		}()

		// If we found a vote with this node, return immediately
		if found {
			return true, nil
		}

		// Otherwise, continue to next node
	}

	// If we've tried all nodes and found no votes, return false
	// If there were errors, return the last one
	if lastErr != nil {
		return false, fmt.Errorf("failed to check validator vote across all nodes: %w", lastErr)
	}

	return false, nil
}

func (d *DefaultAdapter) CountUnvotedOpenProposals(ctx context.Context) (int, error) {
	// get all proposals in voting period
	qProposal := gov.QueryProposalsRequest{
		// Filter for only proposals in voting period
		ProposalStatus: gov.StatusVotingPeriod,
	}
	b, err := qProposal.Marshal()
	if err == nil {
		resp, err := d.ChainConfig.client.ABCIQuery(ctx, "/cosmos.gov.v1.Query/Proposals", b)
		if resp == nil || resp.Response.Value == nil {
			return 0, fmt.Errorf("🛑 failed to query proposals for %s, error: %v", d.ChainConfig.name, err)
		} else {
			proposals := &gov.QueryProposalsResponse{}
			err = proposals.Unmarshal(resp.Response.Value)
			if err == nil {
				// Step 2: Filter out proposals the validator has already voted on
				var unvotedProposals []gov.Proposal

				for _, proposal := range proposals.Proposals {
					// For each proposal, check if the validator has voted
					accAddress, err := ConvertValopertToAccAddress(d.ChainConfig.ValAddress)
					if err != nil {
						l(fmt.Sprintf("⚠️ Cannot convert valoper to account address: %v", err))
						continue
					}

					hasVoted, err := d.CheckIfValidatorVoted(ctx, proposal.ProposalId, accAddress)
					if err != nil {
						l(fmt.Sprintf("⚠️ Error checking if validator voted: %v", err))
					}

					if !hasVoted {
						unvotedProposals = append(unvotedProposals, proposal)
					}
				}

				return len(unvotedProposals), nil
			}
		}
	}
	return 0, err
}
