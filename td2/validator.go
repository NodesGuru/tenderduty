package tenderduty

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/types/bech32"
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

// GetMinSignedPerWindow The check the minimum signed threshold of the validator.
func (cc *ChainConfig) GetMinSignedPerWindow() (err error) {
	if cc.client == nil {
		return errors.New("nil rpc client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var provider ChainProvider
	switch cc.Provider.Name {
	case "namada":
		provider = &NamadaProvider{
			ChainConfig: cc,
		}
	default:
		provider = &DefaultProvider{
			ChainConfig: cc,
		}
	}

	slashingParams, err := provider.QuerySlashingParams(ctx)
	if err != nil {
		return
	}

	cc.minSignedPerWindow = slashingParams.MinSignedPerWindow.MustFloat64()
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

	var provider ChainProvider
	switch cc.Provider.Name {
	case "namada":
		provider = &NamadaProvider{
			ChainConfig: cc,
		}
	default:
		provider = &DefaultProvider{
			ChainConfig: cc,
		}
	}

	// Fetch info from /cosmos.staking.v1beta1.Query/Validator
	// it's easier to ask people to provide valoper since it's readily available on
	// explorers, so make it easy and lookup the consensus key for them.
	conspub, moniker, jailed, bonded, err := provider.QueryValidatorInfo(ctx)
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

	// Query for unvoted proposals regardless of alert setting
	unvotedProposalIds, err := provider.QueryUnvotedOpenProposalIds(ctx)
	if err == nil {
		cc.unvotedOpenGovProposalIds = unvotedProposalIds
		if td.Prom {
			td.statsChan <- cc.mkUpdate(metricUnvotedProposals, float64(len(cc.unvotedOpenGovProposalIds)), "")
		}
	} else {
		l(err)
	}

	// Log if governance alerts are disabled (only on first run)
	if first && !cc.Alerts.GovernanceAlerts {
		l(fmt.Sprintf("ℹ️ Governance alerts disabled for %s (%s)", cc.ValAddress, cc.valInfo.Moniker))
	}

	signingInfo, err := provider.QuerySigningInfo(ctx)
	if err != nil {
		return
	}
	cc.valInfo.Tombstoned = signingInfo.Tombstoned
	if cc.valInfo.Tombstoned {
		l(fmt.Sprintf("❗️☠️ %s (%s) is tombstoned 🪦❗️", cc.ValAddress, cc.valInfo.Moniker))
	}
	cc.valInfo.Missed = signingInfo.MissedBlocksCounter
	if td.Prom {
		td.statsChan <- cc.mkUpdate(metricWindowMissed, float64(cc.valInfo.Missed), "")
	}

	// finally get the signed blocks window
	if cc.valInfo.Window == 0 {
		slashingParams, error := provider.QuerySlashingParams(ctx)
		if error != nil {
			return
		}
		if first && td.Prom {
			td.statsChan <- cc.mkUpdate(metricWindowSize, float64(slashingParams.SignedBlocksWindow), "")
			td.statsChan <- cc.mkUpdate(metricTotalNodes, float64(len(cc.Nodes)), "")
		}
		cc.valInfo.Window = slashingParams.SignedBlocksWindow
	}
	return
}

func ToBytes(address string) []byte {
	bz, _ := hex.DecodeString(strings.ToLower(address))
	return bz
}
