// Copyright (c) 2021-2026 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package rpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"github.com/decred/slog"
	"github.com/jrick/bitset"
	"github.com/jrick/wsrpc/v2"
	"github.com/monetarium/monetarium-node/blockchain/standalone"
	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/chaincfg/chainhash"
	"github.com/monetarium/monetarium-node/gcs"
	"github.com/monetarium/monetarium-node/gcs/blockcf2"
	mondtypes "github.com/monetarium/monetarium-node/rpc/jsonrpc/types"
	"github.com/monetarium/monetarium-node/wire"
)

const (
	// These numerical error codes are defined in mond/dcrjson. Copied here so
	// we dont need to import the whole package.
	ErrRPCDuplicateTx = -40
	ErrNoTxInfo       = -5
)

// ErrOrphan error string is defined in mond/internal/mempool. Copied here
// because it is not exported.
var ErrOrphan = regexp.MustCompile(`orphan transaction \w+ references output \w+:\d+ of unknown or fully-spent transaction`)

// MondRPC provides methods for calling mond JSON-RPCs without exposing the details
// of JSON encoding.
type MondRPC struct {
	Caller
}

type MondConnect struct {
	client *client
	params *chaincfg.Params
	log    slog.Logger
}

func SetupMond(user, pass, addr string, cert []byte, params *chaincfg.Params, log slog.Logger,
	blockConnectedChan chan *wire.BlockHeader) MondConnect {
	client := setup(user, pass, addr, cert, log)

	client.notifier = &blockConnectedHandler{
		blockConnected: blockConnectedChan,
		log:            log,
	}

	return MondConnect{
		client: client,
		params: params,
		log:    log,
	}
}

func (d *MondConnect) Close() {
	d.client.Close()
	d.log.Debug("mond client closed")
}

// Client creates a new MondRPC client instance. Returns an error if dialing
// mond fails or if mond is misconfigured.
func (d *MondConnect) Client() (*MondRPC, string, error) {
	c, newConnection, err := d.client.dial(context.TODO())
	if err != nil {
		return nil, d.client.addr, fmt.Errorf("mond dial error: %w", err)
	}

	mondRPC := &MondRPC{c}

	// If this is a reused connection, we don't need to validate the mond config
	// again.
	if !newConnection {
		return mondRPC, d.client.addr, nil
	}

	// Verify mond is at the required version.
	err = mondRPC.checkVersion()
	if err != nil {
		d.client.Close()
		return nil, d.client.addr, fmt.Errorf("mond version check failed: %w", err)
	}

	// Verify mond is on the correct network.
	netID, err := mondRPC.getCurrentNet()
	if err != nil {
		d.client.Close()
		return nil, d.client.addr, fmt.Errorf("mond getcurrentnet check failed: %w", err)
	}
	if netID != d.params.Net {
		d.client.Close()
		return nil, d.client.addr, fmt.Errorf("mond running on %s, expected %s", netID, d.params.Net)
	}

	// Verify mond has tx index enabled (required for getrawtransaction).
	info, err := mondRPC.getInfo()
	if err != nil {
		d.client.Close()
		return nil, d.client.addr, fmt.Errorf("mond getinfo check failed: %w", err)
	}
	if !info.TxIndex {
		d.client.Close()
		return nil, d.client.addr, errors.New("mond does not have transaction index enabled (--txindex)")
	}

	// Request blockconnected notifications.
	if d.client.notifier != nil {
		err = mondRPC.NotifyBlocks()
		if err != nil {
			return nil, d.client.addr, fmt.Errorf("notifyblocks failed: %w", err)
		}
	}

	d.log.Debugf("Connected to mond")

	return &MondRPC{c}, d.client.addr, nil
}

// checkVersion uses version RPC to retrieve the binary and API version of mond.
// An error is returned if there is not semver compatibility with the minimum
// expected versions.
func (c *MondRPC) checkVersion() error {
	var verMap map[string]mondtypes.VersionResult
	err := c.Call(context.TODO(), "version", &verMap)
	if err != nil {
		return err
	}

	return errors.Join(
		checkVersion(verMap, "monetarium"),
		checkVersion(verMap, "monetariumjsonrpcapi"),
	)
}

// getCurrentNet uses getcurrentnet RPC to return the Decred network the wallet
// is connected to.
func (c *MondRPC) getCurrentNet() (wire.CurrencyNet, error) {
	var netID wire.CurrencyNet
	err := c.Call(context.TODO(), "getcurrentnet", &netID)
	if err != nil {
		return 0, err
	}
	return netID, nil
}

// getInfo uses getinfo RPC to return various daemon, network, and chain info.
func (c *MondRPC) getInfo() (*mondtypes.InfoChainResult, error) {
	var info mondtypes.InfoChainResult
	err := c.Call(context.TODO(), "getinfo", &info)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// GetRawTransaction uses getrawtransaction RPC to retrieve details about the
// transaction with the provided hash.
func (c *MondRPC) GetRawTransaction(txHash string) (*mondtypes.TxRawResult, error) {
	verbose := 1
	var resp mondtypes.TxRawResult
	err := c.Call(context.TODO(), "getrawtransaction", &resp, txHash, verbose)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DecodeRawTransaction uses decoderawtransaction RPC to decode raw transaction bytes.
func (c *MondRPC) DecodeRawTransaction(txHex string) (*mondtypes.TxRawDecodeResult, error) {
	var resp mondtypes.TxRawDecodeResult
	err := c.Call(context.TODO(), "decoderawtransaction", &resp, txHex)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// SendRawTransaction uses sendrawtransaction RPC to broadcast a transaction to
// the network. It ignores errors caused by duplicate transactions.
func (c *MondRPC) SendRawTransaction(txHex string) error {
	const allowHighFees = false
	err := c.Call(context.TODO(), "sendrawtransaction", nil, txHex, allowHighFees)
	if err != nil {

		// Ignore errors caused by the transaction already existing in the
		// mempool or in a mined block.

		// Error code -40 (ErrRPCDuplicateTx) is completely ignorable because it
		// indicates that mond definitely already has this transaction.
		var e *wsrpc.Error
		if errors.As(err, &e) && e.Code == ErrRPCDuplicateTx {
			return nil
		}

		// Errors about orphan/spent outputs indicate that mond *might* already
		// have this transaction. Use getrawtransaction to confirm.
		if ErrOrphan.MatchString(err.Error()) {
			_, getErr := c.GetRawTransaction(txHex)
			if getErr == nil {
				return nil
			}
		}

		return err
	}
	return nil
}

// NotifyBlocks uses notifyblocks RPC to request new block notifications from mond.
func (c *MondRPC) NotifyBlocks() error {
	return c.Call(context.TODO(), "notifyblocks", nil)
}

// GetBestBlockHeader uses getbestblockhash RPC, followed by getblockheader RPC,
// to retrieve the header of the best block known to the mond instance.
func (c *MondRPC) GetBestBlockHeader() (*wire.BlockHeader, error) {
	var bestBlockHash string
	err := c.Call(context.TODO(), "getbestblockhash", &bestBlockHash)
	if err != nil {
		return nil, err
	}

	blockHeader, err := c.GetBlockHeader(bestBlockHash)
	if err != nil {
		return nil, err
	}
	return blockHeader, nil
}

// GetBlockHeader uses getblockheader RPC with verbose=false to retrieve
// the header of the requested block.
func (c *MondRPC) GetBlockHeader(blockHash string) (*wire.BlockHeader, error) {
	const verbose = false
	var resp string
	err := c.Call(context.TODO(), "getblockheader", &resp, blockHash, verbose)
	if err != nil {
		return nil, err
	}

	// Decode the serialized block header hex to raw bytes.
	headerBytes, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}

	// Deserialize the block header and return it.
	var blockHeader wire.BlockHeader
	err = blockHeader.Deserialize(bytes.NewReader(headerBytes))
	if err != nil {
		return nil, err
	}

	return &blockHeader, nil
}

// ExistsLiveTicket uses existslivetickets RPC to check if the provided ticket
// hash is a live ticket known to the mond instance.
func (c *MondRPC) ExistsLiveTicket(ticketHash string) (bool, error) {
	var exists string
	err := c.Call(context.TODO(), "existslivetickets", &exists, []string{ticketHash})
	if err != nil {
		return false, err
	}

	existsBytes := make([]byte, hex.DecodedLen(len(exists)))
	_, err = hex.Decode(existsBytes, []byte(exists))
	if err != nil {
		return false, err
	}

	return bitset.Bytes(existsBytes).Get(0), nil
}

func (c *MondRPC) GetBlock(hash string) (*wire.MsgBlock, error) {
	var resp string
	const verbose = false
	const verboseTx = false
	err := c.Call(context.TODO(), "getblock", &resp, hash, verbose, verboseTx)
	if err != nil {
		return nil, err
	}

	// Decode the serialized block hex to raw bytes.
	blockBytes, err := hex.DecodeString(resp)
	if err != nil {
		return nil, err
	}

	// Deserialize the block and return it.
	var msgBlock wire.MsgBlock
	err = msgBlock.Deserialize(bytes.NewReader(blockBytes))
	if err != nil {
		return nil, err
	}

	return &msgBlock, nil
}

func (c *MondRPC) GetBlockCount() (int64, error) {
	var count int64
	err := c.Call(context.TODO(), "getblockcount", &count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (c *MondRPC) GetBlockHash(height int64) (string, error) {
	var resp string
	err := c.Call(context.TODO(), "getblockhash", &resp, height)
	if err != nil {
		return "", err
	}
	return resp, nil
}

// GetCFilterV2 retrieves the GCS filter for the provided block header,
// optionally verifies the inclusion proof, then returns the filter along with
// its key.
func (c *MondRPC) GetCFilterV2(header *wire.BlockHeader, verifyProof bool) ([gcs.KeySize]byte, *gcs.FilterV2, error) {
	var key [gcs.KeySize]byte
	var resp mondtypes.GetCFilterV2Result
	err := c.Call(context.TODO(), "getcfilterv2", &resp, header.BlockHash().String())
	if err != nil {
		return key, nil, fmt.Errorf("getcfilterv2 error: %w", err)
	}

	filterB, err := hex.DecodeString(resp.Data)
	if err != nil {
		return key, nil, fmt.Errorf("error decoding block filter: %w", err)
	}

	filter, err := gcs.FromBytesV2(blockcf2.B, blockcf2.M, filterB)
	if err != nil {
		return key, nil, fmt.Errorf("error decoding block filter: %w", err)
	}

	if verifyProof {
		filterHash := filter.Hash()

		proofHashes := make([]chainhash.Hash, len(resp.ProofHashes))
		for i, proofHash := range resp.ProofHashes {
			h, err := chainhash.NewHashFromStr(proofHash)
			if err != nil {
				return key, nil, err
			}
			proofHashes[i] = *h
		}

		if !standalone.VerifyInclusionProof(&header.StakeRoot, &filterHash, resp.ProofIndex, proofHashes) {
			return key, nil, errors.New("failed to verify inclusion proof")
		}
	}

	return blockcf2.Key(&header.MerkleRoot), filter, nil
}
