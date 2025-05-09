// the code about querying validator information is taken from https://github.com/ekhvalov/tenderduty/blob/main/td2/validator.go with minor modifications
package tenderduty

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	bank "github.com/cosmos/cosmos-sdk/x/bank/types"
	gov "github.com/cosmos/cosmos-sdk/x/gov/types"
	slashing "github.com/cosmos/cosmos-sdk/x/slashing/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/types"
	namada "github.com/firstset/tenderduty/v2/td2/namada"
	"github.com/near/borsh-go"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

type NamadaProvider struct {
	ChainConfig *ChainConfig
}

func getVotingPeriodProposals(httpClient *http.Client, indexers []string) ([]gov.Proposal, error) {
	// Store the last error to return if all indexer endpoints fail
	var lastErr error

	// Prepare query parameters
	params := url.Values{}
	params.Add("status", "votingPeriod")

	// Slice to store proposal IDs
	votingPeriodProposalIds := []string{}
	votingPeriodProposals := []gov.Proposal{}

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

			var respJson namada.NamadaProposalResponse
			if err = json.NewDecoder(resp.Body).Decode(&respJson); err != nil {
				lastErr = err
				return
			}

			// Process each proposal
			for _, namadaProposal := range respJson.Results {
				govProposal, err := namadaProposal.ToGovProposal()
				if err != nil {
					// Log error but continue with other proposals
					l(fmt.Sprintf("Failed to convert proposal %s: %v", namadaProposal.ID, err))
					continue
				}
				if !slices.Contains(votingPeriodProposalIds, namadaProposal.ID) {
					votingPeriodProposals = append(votingPeriodProposals, *govProposal)
				}
			}
		}()

		// If we found proposals with this node, return them
		if len(votingPeriodProposalIds) > 0 {
			return votingPeriodProposals, nil
		}
	}

	return votingPeriodProposals, lastErr
}

func (d *NamadaProvider) QueryUnvotedOpenProposals(ctx context.Context) ([]gov.Proposal, error) {
	// Store the last error to return if all indexer endpoints fail
	var lastErr error
	var unVotedProposals []gov.Proposal

	indexers, ok1 := d.ChainConfig.Provider.Configs["indexers"].([]any)
	validatorAddress, ok2 := d.ChainConfig.Provider.Configs["validator_address"].(string)
	if ok1 && ok2 {
		// Create a reusable HTTP client with timeout
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: td.TLSSkipVerify},
		}
		httpClient := &http.Client{
			Transport: tr,
			Timeout:   5 * time.Second, // Add reasonable timeout
		}

		urls := make([]string, len(indexers))
		for i, v := range indexers {
			if str, ok := v.(string); ok {
				urls[i] = str
			}
		}

		votingPeriodProposals, err := getVotingPeriodProposals(httpClient, urls)
		votedProposalIds := []float64{}
		if err != nil {
			return nil, err
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
					if idFloat, ok := vote["proposalId"].(float64); ok {
						if !slices.Contains(votedProposalIds, idFloat) {
							votedProposalIds = append(votedProposalIds, idFloat)
						}
					}
				}
			}()
		}

		for _, proposal := range votingPeriodProposals {
			if !slices.Contains(votedProposalIds, float64(proposal.ProposalId)) {
				unVotedProposals = append(unVotedProposals, proposal)
			}
		}
	}

	return unVotedProposals, lastErr
}

func (d *NamadaProvider) QueryValidatorInfo(ctx context.Context) (pub []byte, moniker string, jailed bool, bonded bool, delegatedTokens float64, commissionRate float64, err error) {
	hexAddress := ""
	if strings.Contains(d.ChainConfig.ValAddress, "valcons") {
		_, bz, err := bech32.DecodeAndConvert(d.ChainConfig.ValAddress)
		if err != nil {
			return nil, "", false, false, 0, 0, errors.New("could not decode and convert your address " + d.ChainConfig.ValAddress)
		}
		hexAddress = fmt.Sprintf("%X", bz)
	}

	validatorAddress, ok := d.ChainConfig.Provider.Configs["validator_address"].(string)

	if ok {
		response, err := d.ChainConfig.client.ABCIQuery(ctx, fmt.Sprintf("/vp/pos/validator/state/%s", validatorAddress), nil)
		if err != nil {
			return nil, "", false, false, 0, 0, errors.New("failed to query Namada validator's state " + validatorAddress)
		}

		state := namada.ValidatorStateInfo{}
		err = borsh.Deserialize(&state, response.Response.Value)
		if err != nil {
			return nil, "", false, false, 0, 0, fmt.Errorf("unmarshal validator state: %w", err)
		}
		info := ValInfo{}
		info.Bonded = state.State != nil && *state.State == namada.ValidatorStateConsensus
		info.Jailed = state.State != nil && *state.State == namada.ValidatorStateJailed

		response, err = d.ChainConfig.client.ABCIQuery(ctx, fmt.Sprintf("/vp/pos/validator/metadata/%s", validatorAddress), nil)
		if err != nil {
			return nil, "", false, false, 0, 0, fmt.Errorf("query validator metadata: %w", err)
		}
		metadata := namada.ValidatorMetaData{}
		err = borsh.Deserialize(&metadata, response.Response.Value)
		if err != nil {
			return nil, "", false, false, 0, 0, fmt.Errorf("unmarshal validator metadata: %w", err)
		}
		if metadata.Metadata != nil && metadata.Metadata.Name != nil {
			info.Moniker = *metadata.Metadata.Name
		}
		return ToBytes(hexAddress), info.Moniker, info.Jailed, info.Bonded, 0, 0, nil
	}

	return ToBytes(hexAddress), d.ChainConfig.ValAddress, false, true, 0, 0, nil
}

func getLivenessInfo(ctx context.Context, client *rpchttp.HTTP) (*namada.LivenessInfo, error) {
	resp, err := client.ABCIQuery(ctx, "/vp/pos/validator/liveness_info", nil)
	if err != nil {
		return nil, fmt.Errorf("query validator liveness_info: %w", err)
	}

	livenessInfo := namada.LivenessInfo{}
	err = borsh.Deserialize(&livenessInfo, resp.Response.Value)
	if err != nil {
		return nil, fmt.Errorf("unmarshal liveness info: %w", err)
	}

	return &livenessInfo, nil
}

func (d *NamadaProvider) QuerySigningInfo(ctx context.Context) (*slashing.ValidatorSigningInfo, error) {
	livenessInfo, err := getLivenessInfo(ctx, d.ChainConfig.client)
	if err != nil {
		return nil, err
	}

	signingInfo := slashing.ValidatorSigningInfo{}
	hexAddress := strings.ToUpper(hex.EncodeToString(d.ChainConfig.valInfo.Conspub))
	for _, v := range livenessInfo.Validators {
		if v.CometAddress == hexAddress {
			signingInfo.MissedBlocksCounter = int64(v.MissedVotes)
		}
	}

	return &signingInfo, nil
}

func (d *NamadaProvider) QuerySlashingParams(ctx context.Context) (*slashing.Params, error) {
	livenessInfo, err := getLivenessInfo(ctx, d.ChainConfig.client)
	if err != nil {
		return nil, err
	}

	return &slashing.Params{SignedBlocksWindow: int64(livenessInfo.LivenessWindowLen), MinSignedPerWindow: cosmos_sdk_types.MustNewDecFromStr(livenessInfo.LivenessThreshold.String())}, nil
}

func (d *NamadaProvider) QueryDenomMetadata(ctx context.Context, denom string) (medatada *bank.Metadata, err error) {
	return nil, errors.New("Not Implemented")
}

func (d *NamadaProvider) QueryValidatorSelfDelegationRewardsAndCommission(ctx context.Context) (rewards *github_com_cosmos_cosmos_sdk_types.DecCoins, commission *github_com_cosmos_cosmos_sdk_types.DecCoins, err error) {
	return nil, nil, errors.New("Not Implemented")
}

func (d *NamadaProvider) QueryValidatorVotingPool(ctx context.Context) (votingPool *staking.Pool, err error) {
	return nil, errors.New("Not Implemented")
}
