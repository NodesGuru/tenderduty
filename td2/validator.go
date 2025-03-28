package tenderduty

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	slashing "github.com/cosmos/cosmos-sdk/x/slashing/types"
	staking "github.com/cosmos/cosmos-sdk/x/staking/types"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

// ValInfo holds most of the stats/info used for secondary alarms. It is refreshed roughly every minute.
type ValInfo struct {
	Moniker    string `json:"moniker"`
	Bonded     bool   `json:"bonded"`
	Jailed     bool   `json:"jailed"`
	Tombstoned bool   `json:"tombstoned"`
	Missed     int64  `json:"missed"`
	Window     int64  `json:"window"`
	Conspub    []byte `json:"conspub"`
	Valcons    string `json:"valcons"`
}

func ConvertValopertToAccAddress(valoperAddr string) (string, error) {
	// Check if it's a valoper address
	if !strings.Contains(valoperAddr, "valoper") {
		return valoperAddr, nil // Already an account address or something else
	}

	// Decode the address
	prefix, bytes, err := bech32.DecodeAndConvert(valoperAddr)
	if err != nil {
		return "", fmt.Errorf("🌟 failed to decode valoper address: %w", err)
	}

	// Get the base prefix by removing "valoper"
	basePrefix := strings.Replace(prefix, "valoper", "", 1)

	// Re-encode with the base prefix
	accAddress, err := bech32.ConvertAndEncode(basePrefix, bytes)
	if err != nil {
		return "", fmt.Errorf("🌟 failed to encode account address: %w", err)
	}

	return accAddress, nil
}

func CheckIfValidatorVoted(ctx context.Context, cc *ChainConfig, proposalID uint64, accAddress string) (bool, error) {
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
	for _, node := range cc.Nodes {
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

// GetMinSignedPerWindow The check the minimum signed threshold of the validator.
func (cc *ChainConfig) GetMinSignedPerWindow() (err error) {
	if cc.client == nil {
		return errors.New("nil rpc client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	qParams := &slashing.QueryParamsRequest{}
	b, err := qParams.Marshal()
	if err != nil {
		return
	}
	resp, err := cc.client.ABCIQuery(ctx, "/cosmos.slashing.v1beta1.Query/Params", b)
	if err != nil {
		return
	}
	if resp.Response.Value == nil {
		err = errors.New("🛑 could not query slashing params, got empty response")
		return
	}
	params := &slashing.QueryParamsResponse{}
	err = params.Unmarshal(resp.Response.Value)
	if err != nil {
		return
	}

	cc.minSignedPerWindow = params.Params.MinSignedPerWindow.MustFloat64()
	return
}

// GetValInfo the first bool is used to determine if extra information about the validator should be printed.
func (cc *ChainConfig) GetValInfo(first bool) (err error) {
	if cc.client == nil {
		return errors.New("nil rpc client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if cc.valInfo == nil {
		cc.valInfo = &ValInfo{}
	}

	// Fetch info from /cosmos.staking.v1beta1.Query/Validator
	// it's easier to ask people to provide valoper since it's readily available on
	// explorers, so make it easy and lookup the consensus key for them.
	conspub, moniker, jailed, bonded, err := getVal(ctx, cc.client, cc.ValAddress)
	if err != nil {
		return
	}

	cc.valInfo.Conspub = conspub
	cc.valInfo.Moniker = moniker
	cc.valInfo.Jailed = jailed
	cc.valInfo.Bonded = bonded

	if first && cc.valInfo.Bonded {
		l(fmt.Sprintf("⚙️ found %s (%s) in validator set", cc.ValAddress, cc.valInfo.Moniker))
	} else if first && !cc.valInfo.Bonded {
		l(fmt.Sprintf("❌ %s (%s) is INACTIVE", cc.ValAddress, cc.valInfo.Moniker))
	}

	if strings.Contains(cc.ValAddress, "valcons") {
		// no need to change prefix for signing info query
		cc.valInfo.Valcons = cc.ValAddress
	} else {
		// need to know the prefix for when we serialize the slashing info query, this is too fragile.
		// for now, we perform specific chain overrides based on known values because the valoper is used
		// in so many places.
		var prefix string
		split := strings.Split(cc.ValAddress, "valoper")
		if len(split) != 2 {
			if pre, ok := altValopers.getAltPrefix(cc.ValAddress); ok {
				cc.valInfo.Valcons, err = bech32.ConvertAndEncode(pre, cc.valInfo.Conspub[:20])
				if err != nil {
					return
				}
			} else {
				err = errors.New("❓ could not determine bech32 prefix from valoper address: " + cc.ValAddress)
				return
			}
		} else {
			prefix = split[0] + "valcons"
			cc.valInfo.Valcons, err = bech32.ConvertAndEncode(prefix, cc.valInfo.Conspub[:20])
			if err != nil {
				return
			}
		}
		if first {
			l("⚙️", cc.ValAddress[:20], "... is using consensus key:", cc.valInfo.Valcons)
		}
	}

	var adapter ChainAdapter
	switch cc.Adapter.Name {
	case "namada":
		adapter = &NamadaAdapter{
			ChainConfig: cc,
		}
	default:
		adapter = &DefaultAdapter{
			ChainConfig: cc,
		}
	}
	unvotedProposalIds, err := adapter.QueryUnvotedOpenProposalIds(ctx)
	if err == nil {
		l("🌟 found unvoted proposals", cc.name, unvotedProposalIds)
		cc.unvotedOpenGovProposalIds = unvotedProposalIds
		if td.Prom {
			td.statsChan <- cc.mkUpdate(metricUnvotedProposals, float64(len(cc.unvotedOpenGovProposalIds)), "")
		}
	} else {
		l(err)
	}

	// get current signing information (tombstoned, missed block count)
	qSigning := slashing.QuerySigningInfoRequest{ConsAddress: cc.valInfo.Valcons}
	b, err := qSigning.Marshal()
	if err != nil {
		return
	}
	resp, err := cc.client.ABCIQuery(ctx, "/cosmos.slashing.v1beta1.Query/SigningInfo", b)
	if resp == nil || resp.Response.Value == nil {
		err = errors.New("could not query validator slashing status, got empty response")
		return
	}
	slash := &slashing.QuerySigningInfoResponse{}
	err = slash.Unmarshal(resp.Response.Value)
	if err != nil {
		return
	}
	cc.valInfo.Tombstoned = slash.ValSigningInfo.Tombstoned
	if cc.valInfo.Tombstoned {
		l(fmt.Sprintf("❗️☠️ %s (%s) is tombstoned 🪦❗️", cc.ValAddress, cc.valInfo.Moniker))
	}
	cc.valInfo.Missed = slash.ValSigningInfo.MissedBlocksCounter
	if td.Prom {
		td.statsChan <- cc.mkUpdate(metricWindowMissed, float64(cc.valInfo.Missed), "")
	}

	// finally get the signed blocks window
	if cc.valInfo.Window == 0 {
		qParams := &slashing.QueryParamsRequest{}
		b, err = qParams.Marshal()
		if err != nil {
			return
		}
		resp, err = cc.client.ABCIQuery(ctx, "/cosmos.slashing.v1beta1.Query/Params", b)
		if err != nil {
			return
		}
		if resp.Response.Value == nil {
			err = errors.New("🛑 could not query slashing params, got empty response")
			return
		}
		params := &slashing.QueryParamsResponse{}
		err = params.Unmarshal(resp.Response.Value)
		if err != nil {
			return
		}
		if first && td.Prom {
			td.statsChan <- cc.mkUpdate(metricWindowSize, float64(params.Params.SignedBlocksWindow), "")
			td.statsChan <- cc.mkUpdate(metricTotalNodes, float64(len(cc.Nodes)), "")
		}
		cc.valInfo.Window = params.Params.SignedBlocksWindow
	}
	return
}

// getVal returns the public key, moniker, and if the validator is jailed.
func getVal(ctx context.Context, client *rpchttp.HTTP, valoper string) (pub []byte, moniker string, jailed, bonded bool, err error) {
	if strings.Contains(valoper, "valcons") {
		_, bz, err := bech32.DecodeAndConvert(valoper)
		if err != nil {
			return nil, "", false, false, errors.New("could not decode and convert your address" + valoper)
		}

		hexAddress := fmt.Sprintf("%X", bz)
		return ToBytes(hexAddress), valoper, false, true, nil
	}

	q := staking.QueryValidatorRequest{
		ValidatorAddr: valoper,
	}
	b, err := q.Marshal()
	if err != nil {
		return
	}
	resp, err := client.ABCIQuery(ctx, "/cosmos.staking.v1beta1.Query/Validator", b)
	if err != nil {
		return
	}
	if resp.Response.Value == nil {
		return nil, "", false, false, errors.New("could not find validator " + valoper)
	}
	val := &staking.QueryValidatorResponse{}
	err = val.Unmarshal(resp.Response.Value)
	if err != nil {
		return
	}
	if val.Validator.ConsensusPubkey == nil {
		return nil, "", false, false, errors.New("got invalid consensus pubkey for " + valoper)
	}

	pubBytes := make([]byte, 0)
	switch val.Validator.ConsensusPubkey.TypeUrl {
	case "/cosmos.crypto.ed25519.PubKey":
		pk := ed25519.PubKey{}
		err = pk.Unmarshal(val.Validator.ConsensusPubkey.Value)
		if err != nil {
			return
		}
		pubBytes = pk.Address().Bytes()
	case "/cosmos.crypto.secp256k1.PubKey":
		pk := secp256k1.PubKey{}
		err = pk.Unmarshal(val.Validator.ConsensusPubkey.Value)
		if err != nil {
			return
		}
		pubBytes = pk.Address().Bytes()
	}
	if len(pubBytes) == 0 {
		return nil, "", false, false, errors.New("could not get pubkey for" + valoper)
	}

	return pubBytes, val.Validator.GetMoniker(), val.Validator.Jailed, val.Validator.Status == 3, nil
}

func ToBytes(address string) []byte {
	bz, _ := hex.DecodeString(strings.ToLower(address))
	return bz
}
