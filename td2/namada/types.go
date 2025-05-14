// this file is taken from https://github.com/ekhvalov/tenderduty/blob/main/pkg/namada/types.go
package namada

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/cosmos/btcutil/bech32"
	gov "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/near/borsh-go"
)

type Uint [4]uint64

func (u Uint) BigInt() *big.Int {
	result := new(big.Int).SetUint64(u[3])
	for i := 2; i >= 0; i-- {
		result = new(big.Int).Lsh(result, 64)
		result = new(big.Int).Add(result, new(big.Int).SetUint64(u[i]))
	}
	return result
}

func (u Uint) String() string {
	return u.BigInt().String()
}

const decPrecision = 12

var e12 = new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decPrecision), nil))

type Dec struct {
	Raw Uint
}

func (d Dec) String() string {
	dec := new(big.Float).SetInt(d.Raw.BigInt())
	dec = new(big.Float).Quo(dec, e12)
	return dec.SetPrec(decPrecision).Text('f', -1)
}

func encodeBytes(hrp string, bs []byte) string {
	result, err := bech32.EncodeFromBase256(hrp, bs)
	if err != nil {
		return err.Error()
	}
	return result
}

type AddressHash [20]byte

func getHumanAddress(discriminant byte, address AddressHash) string {
	res := make([]byte, 21)
	res[0] = discriminant
	copy(res[1:], address[:])
	return encodeBytes("tnam", res)
}

type EstablishedAddress struct {
	Hash AddressHash
}

func (ea EstablishedAddress) String() string {
	return getHumanAddress(DiscriminantEstablished, ea.Hash)
}

type ImplicitAddress struct {
	AddressHash
}

func (ia ImplicitAddress) String() string {
	return getHumanAddress(DiscriminantImplicit, ia.AddressHash)
}

type IbcTokenHash AddressHash

type EthAddress AddressHash

type InternalAddress struct {
	Enum          borsh.Enum `borsh_enum:"true"`
	PoS           struct{}
	PosSlashPool  struct{}
	Parameters    struct{}
	Ibc           struct{}
	IbcToken      IbcTokenHash
	Governance    struct{}
	EthBridge     struct{}
	EthBridgePool struct{}
	Erc20         EthAddress
	Nut           EthAddress
	Multitoken    struct{}
	Pgf           struct{}
	Masp          struct{}
}

var DefaultAddress AddressHash = [20]byte{}

const (
	DiscriminantImplicit byte = iota
	DiscriminantEstablished
	DiscriminantPos
	DiscriminantSlashPool
	DiscriminantParameters
	DiscriminantGovernance
	DiscriminantIbc
	DiscriminantEthBridge
	DiscriminantBridgePool
	DiscriminantMultitoken
	DiscriminantPgf
	DiscriminantErc20
	DiscriminantNut
	DiscriminantIbcToken
	DiscriminantMasp
)

func (ia InternalAddress) String() string {
	switch ia.Enum {
	case 0:
		return getHumanAddress(DiscriminantPos, DefaultAddress)
	case 1:
		return getHumanAddress(DiscriminantSlashPool, DefaultAddress)
	case 2:
		return getHumanAddress(DiscriminantParameters, DefaultAddress)
	case 3:
		return getHumanAddress(DiscriminantIbc, DefaultAddress)
	case 4:
		return getHumanAddress(DiscriminantIbcToken, AddressHash(ia.IbcToken))
	case 5:
		return getHumanAddress(DiscriminantGovernance, DefaultAddress)
	case 6:
		return getHumanAddress(DiscriminantEthBridge, DefaultAddress)
	case 7:
		return getHumanAddress(DiscriminantBridgePool, DefaultAddress)
	case 8:
		return getHumanAddress(DiscriminantErc20, AddressHash(ia.Erc20))
	case 9:
		return getHumanAddress(DiscriminantNut, AddressHash(ia.Nut))
	case 10:
		return getHumanAddress(DiscriminantMultitoken, DefaultAddress)
	case 11:
		return getHumanAddress(DiscriminantPgf, DefaultAddress)
	case 12:
		return getHumanAddress(DiscriminantMasp, DefaultAddress)
	}
	return ""
}

type Address struct {
	Enum        borsh.Enum `borsh_enum:"true"`
	Established EstablishedAddress
	Implicit    ImplicitAddress
	Internal    InternalAddress
}

func (a Address) String() string {
	switch a.Enum {
	case 0:
		return a.Established.String()
	case 1:
		return a.Implicit.String()
	case 2:
		return a.Internal.String()
	}
	return ""
}

type LivenessInfo struct {
	LivenessWindowLen uint64              `json:"liveness_window_len"`
	LivenessThreshold Dec                 `json:"liveness_threshold"`
	Validators        []ValidatorLiveness `json:"validators"`
}

type ValidatorLiveness struct {
	NativeAddress Address `json:"native_address"`
	CometAddress  string  `json:"comet_address"`
	MissedVotes   uint64  `json:"missed_votes"`
}

type Epoch uint64

type ValidatorState borsh.Enum

func (v ValidatorState) String() string {
	switch v {
	case ValidatorStateConsensus:
		return "Consensus"
	case ValidatorStateBelowCapacity:
		return "BelowCapacity"
	case ValidatorStateBelowThreshold:
		return "BelowThreshold"
	case ValidatorStateInactive:
		return "Inactive"
	case ValidatorStateJailed:
		return "Jailed"
	default:
		return "Unknown"
	}
}

const (
	ValidatorStateConsensus ValidatorState = iota
	ValidatorStateBelowCapacity
	ValidatorStateBelowThreshold
	ValidatorStateInactive
	ValidatorStateJailed
)

type ValidatorStateInfo struct {
	State *ValidatorState `borsh_enum:"true"`
	Epoch Epoch
}

type ValidatorMetaData struct {
	Metadata *struct {
		Email         string  // Validator's email
		Description   *string // Validator description, optional
		Website       *string // Validator website, optional
		DiscordHandle *string // Validator's discord handle, optional
		Avatar        *string // URL that points to a picture identifying the validator, optional
		Name          *string // Validator's name, optional
	}
}

type ValidatorCommissionPair struct {
	CommissionRate              *Dec  // Validator commission rate, optional
	MaxCommissionChangePerEpoch *Dec  // Validator max commission rate change per epoch, optional
	Epoch                       Epoch // Query epoch
}

// NamadaProposalResponse represents the structure of the API response
type NamadaProposalResponse struct {
	Results    []NamadaProposal `json:"results"`
	Pagination struct {
		Page       int `json:"page"`
		PerPage    int `json:"perPage"`
		TotalPages int `json:"totalPages"`
		TotalItems int `json:"totalItems"`
	} `json:"pagination"`
}

type NamadaVotingPowerResponse struct {
	TotalVotingPower string `json:"totalVotingPower"`
}

type Validator struct {
	ValidatorID   string `json:"validatorId"`
	Rank          int    `json:"rank"`
	Address       string `json:"address"`
	VotingPower   string `json:"votingPower"`
	MaxCommission string `json:"maxCommission"`
	Commission    string `json:"commission"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	Website       string `json:"website"`
	Description   string `json:"description"`
	DiscordHandle string `json:"discordHandle"`
	Avatar        string `json:"avatar"`
	State         string `json:"state"`
}

type NamadaValidatorRewardsResponse struct {
	Validator      Validator `json:"validator"`
	MinDenomAmount string    `json:"minDenomAmount"`
}

// NamadaProposal represents a proposal in the Namada ecosystem
type NamadaProposal struct {
	ID              string `json:"id"`
	Content         string `json:"content"`
	Type            string `json:"type"`
	TallyType       string `json:"tallyType"`
	Data            string `json:"data"`
	Author          string `json:"author"`
	StartEpoch      string `json:"startEpoch"`
	EndEpoch        string `json:"endEpoch"`
	ActivationEpoch string `json:"activationEpoch"`
	StartTime       string `json:"startTime"`
	EndTime         string `json:"endTime"`
	CurrentTime     string `json:"currentTime"`
	ActivationTime  string `json:"activationTime"`
	Status          string `json:"status"`
	YayVotes        string `json:"yayVotes"`
	NayVotes        string `json:"nayVotes"`
	AbstainVotes    string `json:"abstainVotes"`
}

func (np *NamadaProposal) ToGovProposal() (*gov.Proposal, error) {
	// Parse the proposal ID
	proposalId, err := strconv.ParseUint(np.ID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proposal ID: %w", err)
	}

	// Parse voting times
	var votingStartTime, votingEndTime time.Time
	if startTs, err := strconv.ParseInt(np.StartTime, 10, 64); err == nil {
		votingStartTime = time.Unix(startTs, 0)
	} else {
		return nil, fmt.Errorf("failed to parse voting start time: %w", err)
	}

	if endTs, err := strconv.ParseInt(np.EndTime, 10, 64); err == nil {
		votingEndTime = time.Unix(endTs, 0)
	} else {
		return nil, fmt.Errorf("failed to parse voting end time: %w", err)
	}

	// Create and return the gov.Proposal
	return &gov.Proposal{
		ProposalId:      proposalId,
		VotingStartTime: votingStartTime,
		VotingEndTime:   votingEndTime,
	}, nil
}
