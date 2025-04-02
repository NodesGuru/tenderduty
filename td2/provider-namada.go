// the code about querying validator information is taken from https://github.com/ekhvalov/tenderduty/blob/main/td2/validator.go with minor modifications
package tenderduty

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	slashing "github.com/cosmos/cosmos-sdk/x/slashing/types"
	namada "github.com/firstset/tenderduty/v2/td2/namada"
	"github.com/near/borsh-go"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

type NamadaProvider struct {
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

func (d *NamadaProvider) QueryUnvotedOpenProposalIds(ctx context.Context) ([]uint64, error) {
	// Store the last error to return if all indexer endpoints fail
	var lastErr error
	var unVotedProposalIds []uint64

	indexers, ok1 := d.ChainConfig.Provider.Configs["indexers"].([]any)
	validatorAddress, ok2 := d.ChainConfig.Provider.Configs["validator_address"].(string)
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
					if id, ok := vote["proposalId"].(float64); ok && !slices.Contains(votedProposalIds, strconv.Itoa(int(id))) {
						votedProposalIds = append(votedProposalIds, strconv.Itoa(int(id)))
					}
				}
			}()
		}

		for _, id := range votingPeriodProposalIds {
			if !slices.Contains(votedProposalIds, id) {
				if idUint, err := strconv.ParseUint(id, 10, 64); err == nil {
					unVotedProposalIds = append(unVotedProposalIds, idUint)
				} else {
					l(fmt.Sprintf("🛑 error converting proposal ID %s to uint64: %v", id, err))
				}
			}
		}
	}

	return unVotedProposalIds, lastErr
}

func (d *NamadaProvider) QueryValidatorInfo(ctx context.Context) (pub []byte, moniker string, jailed bool, bonded bool, err error) {
	hexAddress := ""
	if strings.Contains(d.ChainConfig.ValAddress, "valcons") {
		_, bz, err := bech32.DecodeAndConvert(d.ChainConfig.ValAddress)
		if err != nil {
			return nil, "", false, false, errors.New("could not decode and convert your address " + d.ChainConfig.ValAddress)
		}
		hexAddress = fmt.Sprintf("%X", bz)
	}

	validatorAddress, ok := d.ChainConfig.Provider.Configs["validator_address"].(string)

	if ok {
		response, err := d.ChainConfig.client.ABCIQuery(ctx, fmt.Sprintf("/vp/pos/validator/state/%s", validatorAddress), nil)
		if err != nil {
			return nil, "", false, false, errors.New("failed to query Namada validator's state " + validatorAddress)
		}

		state := namada.ValidatorStateInfo{}
		err = borsh.Deserialize(&state, response.Response.Value)
		if err != nil {
			return nil, "", false, false, fmt.Errorf("unmarshal validator state: %w", err)
		}
		info := ValInfo{}
		info.Bonded = state.State != nil && *state.State == namada.ValidatorStateConsensus
		info.Jailed = state.State != nil && *state.State == namada.ValidatorStateJailed

		response, err = d.ChainConfig.client.ABCIQuery(ctx, fmt.Sprintf("/vp/pos/validator/metadata/%s", validatorAddress), nil)
		if err != nil {
			return nil, "", false, false, fmt.Errorf("query validator metadata: %w", err)
		}
		metadata := namada.ValidatorMetaData{}
		err = borsh.Deserialize(&metadata, response.Response.Value)
		if err != nil {
			return nil, "", false, false, fmt.Errorf("unmarshal validator metadata: %w", err)
		}
		if metadata.Metadata != nil && metadata.Metadata.Name != nil {
			info.Moniker = *metadata.Metadata.Name
		}
		return ToBytes(hexAddress), info.Moniker, info.Jailed, info.Bonded, nil
	}

	return ToBytes(hexAddress), d.ChainConfig.ValAddress, false, true, nil
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
