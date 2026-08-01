// Copyright (c) 2020-2024 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"fmt"

	"github.com/monetarium/monetarium-node/chaincfg"
)

type Network struct {
	*chaincfg.Params
	DcrdRPCServerPort   string
	WalletRPCServerPort string
	BlockExplorerURL    string
	// MinWallets is the minimum number of voting wallets required for a vspd
	// deployment on this network. vspd will log an error and refuse to start if
	// fewer wallets are configured.
	MinWallets int
	// DCP0005Height is the activation height of DCP-0005 block header
	// commitments agenda on this network.
	DCP0005Height int64
	// DCP0010Height is the activation height of DCP-0010 change PoW/PoS subsidy
	// split agenda on this network.
	DCP0010Height int64
	// DCP0012Height is the activation height of DCP-0012 change PoW/PoS subsidy
	// split R2 agenda on this network.
	DCP0012Height int64
}

// Monetarium networks launched on a modern (dcrd 2.x era) rule set: the
// DCP-0005/0010/0012 rules are active from the first block on every network,
// so all three heights are 1 (mirroring how upstream configures simnet). The
// legacy stake-version 4..10 deployments carried in chainparams are ancestry,
// not history that played out on this chain.
//
// RPC ports follow the fork's own scheme (see monetarium-node sampleconfig
// and monetarium-wallet internal/netparams): node RPC 9509/19509, wallet
// JSON-RPC 9510/19510 for mainnet/testnet respectively.

var MainNet = Network{
	Params:              chaincfg.MainNetParams(),
	DcrdRPCServerPort:   "9509",
	WalletRPCServerPort: "9510",
	BlockExplorerURL:    "https://monetarium.online",
	MinWallets:          3,
	DCP0005Height:       1,
	DCP0010Height:       1,
	DCP0012Height:       1,
}

var TestNet3 = Network{
	Params:              chaincfg.TestNet3Params(),
	DcrdRPCServerPort:   "19509",
	WalletRPCServerPort: "19510",
	BlockExplorerURL:    "https://testnet.monetarium.online",
	MinWallets:          1,
	DCP0005Height:       1,
	DCP0010Height:       1,
	DCP0012Height:       1,
}

var SimNet = Network{
	Params:              chaincfg.SimNetParams(),
	DcrdRPCServerPort:   "19556",
	WalletRPCServerPort: "19557",
	BlockExplorerURL:    "...",
	MinWallets:          1,
	// All rules active from the start on simnet, same as upstream.
	DCP0005Height: 1,
	DCP0010Height: 1,
	DCP0012Height: 1,
}

func NetworkFromName(name string) (*Network, error) {
	switch name {
	case "mainnet":
		return &MainNet, nil
	case "testnet":
		return &TestNet3, nil
	case "simnet":
		return &SimNet, nil
	default:
		return nil, fmt.Errorf("%q is not a supported network", name)
	}
}

// DCP5Active returns true if the DCP-0005 block header commitments agenda is
// active on this network at the provided height, otherwise false.
func (n *Network) DCP5Active(height int64) bool {
	return height >= n.DCP0005Height
}

// DCP10Active returns true if the DCP-0010 change PoW/PoS subsidy split agenda
// is active on this network at the provided height, otherwise false.
func (n *Network) DCP10Active(height int64) bool {
	return height >= n.DCP0010Height
}

// DCP12Active returns true if the DCP-0012 change PoW/PoS subsidy split R2
// agenda is active on this network at the provided height, otherwise false.
func (n *Network) DCP12Active(height int64) bool {
	return height >= n.DCP0012Height
}

// CurrentVoteVersion returns the most recent version in the current networks
// consensus agenda deployments.
func (n *Network) CurrentVoteVersion() uint32 {
	var latestVersion uint32
	for version := range n.Deployments {
		if latestVersion < version {
			latestVersion = version
		}
	}
	return latestVersion
}
