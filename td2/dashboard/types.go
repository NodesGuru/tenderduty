package dash

import (
	github_com_cosmos_cosmos_sdk_types "github.com/cosmos/cosmos-sdk/types"
	utils "github.com/firstset/tenderduty/v2/td2/utils"
)

type ChainStatus struct {
	MsgType                 string                                       `json:"msgType"`
	Name                    string                                       `json:"name"`
	ChainId                 string                                       `json:"chain_id"`
	Moniker                 string                                       `json:"moniker"`
	Bonded                  bool                                         `json:"bonded"`
	Jailed                  bool                                         `json:"jailed"`
	Tombstoned              bool                                         `json:"tombstoned"`
	Missed                  int64                                        `json:"missed"`
	Window                  int64                                        `json:"window"`
	MinSignedPerWindow      float64                                      `json:"min_signed_per_window"`
	Nodes                   int                                          `json:"nodes"`
	HealthyNodes            int                                          `json:"healthy_nodes"`
	ActiveAlerts            int                                          `json:"active_alerts"`
	Height                  int64                                        `json:"height"`
	LastError               string                                       `json:"last_error"`
	UnvotedOpenGovProposals int                                          `json:"unvoted_open_gov_proposals"`
	TotalBondedTokens       float64                                      `json:"total_bonded_tokens"`
	VotingPowerPercent      float64                                      `json:"voting_power_percent"`
	DelegatedTokens         float64                                      `json:"delegated_tokens"`
	CommissionRate          float64                                      `json:"commission_rate"`
	SelfDelegationRewards   *github_com_cosmos_cosmos_sdk_types.DecCoins `json:"self_delegation_rewards"`
	Commission              *github_com_cosmos_cosmos_sdk_types.DecCoins `json:"commission"`
	CryptoPrice             *utils.CryptoPrice                           `json:"crypto_price"`

	Blocks []int `json:"blocks"`
}

type LogMessage struct {
	MsgType string `json:"msgType"`
	Ts      int64  `json:"ts"`
	Msg     string `json:"msg"`
}
