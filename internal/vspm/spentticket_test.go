// Copyright (c) 2026 The Monetarium developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vspm

import (
	"bytes"
	"math"
	"testing"

	"github.com/monetarium/monetarium-node/chaincfg"
	"github.com/monetarium/monetarium-node/txscript"
	"github.com/monetarium/monetarium-node/txscript/stdaddr"
	"github.com/monetarium/monetarium-node/wire"
)

// voteTx returns a vote whose SSFee consolidation output carries the given
// two-byte marker: "SC" for a P2PKH address, "SH" for a P2SH one.
func voteTx(t *testing.T, marker string) *wire.MsgTx {
	t.Helper()

	hash160 := bytes.Repeat([]byte{0x11}, 20)
	addr, err := stdaddr.NewAddressPubKeyHashEcdsaSecp256k1V0(hash160,
		chaincfg.MainNetParams())
	if err != nil {
		t.Fatal(err)
	}
	_, rewardScript := addr.PayVoteCommitmentScript()

	// OP_RETURN OP_DATA_36 <block hash> <block height>
	blockRef := append([]byte{txscript.OP_RETURN, txscript.OP_DATA_36},
		make([]byte, 36)...)
	// OP_RETURN OP_DATA_2 <vote bits>
	voteBits := []byte{txscript.OP_RETURN, txscript.OP_DATA_2, 0x01, 0x00}
	// OP_RETURN OP_DATA_22 <marker> <hash160>
	consolidation := append([]byte{txscript.OP_RETURN, txscript.OP_DATA_22},
		marker...)
	consolidation = append(consolidation, hash160...)

	tx := wire.NewMsgTx()
	tx.AddTxIn(&wire.TxIn{ // stakebase
		PreviousOutPoint: wire.OutPoint{Index: math.MaxUint32},
		BlockHeight:      wire.NullBlockHeight,
		BlockIndex:       wire.NullBlockIndex,
		SignatureScript:  []byte{0x00, 0x00},
	})
	tx.AddTxIn(&wire.TxIn{ // ticket
		PreviousOutPoint: wire.OutPoint{Hash: [32]byte{0x01}, Tree: wire.TxTreeStake},
	})
	tx.AddTxOut(&wire.TxOut{PkScript: blockRef})
	tx.AddTxOut(&wire.TxOut{PkScript: voteBits})
	tx.AddTxOut(&wire.TxOut{Value: 1e8, PkScript: rewardScript})
	tx.AddTxOut(&wire.TxOut{PkScript: consolidation})
	return tx
}

func TestSpentTicketVoted(t *testing.T) {
	for _, marker := range []string{"SC", "SH"} {
		s := spentTicket{spendingTx: voteTx(t, marker)}
		if !s.voted() {
			t.Errorf("vote with %q consolidation output: voted() = false, want true",
				marker)
		}
	}
}
