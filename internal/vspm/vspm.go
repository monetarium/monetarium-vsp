// Copyright (c) 2020-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package vspm

import (
	"context"
	"errors"
	"time"

	"github.com/decred/slog"
	"github.com/monetarium/monetarium-node/wire"
	"github.com/monetarium/monetarium-vsp/database"
	"github.com/monetarium/monetarium-vsp/internal/config"
	"github.com/monetarium/monetarium-vsp/rpc"
)

const (
	// requiredConfs is the number of confirmations required to consider a
	// ticket purchase or a fee transaction to be final.
	requiredConfs = 6

	// consistencyInterval is the time period between wallet consistency checks.
	consistencyInterval = 30 * time.Minute

	// mondInterval is the time period between mond connection checks.
	mondInterval = time.Second * 15
)

type Vspm struct {
	network *config.Network
	log     slog.Logger
	db      *database.VspDatabase
	mond    rpc.MondConnect
	wallets rpc.WalletConnect

	blockNotifChan chan *wire.BlockHeader

	// lastScannedBlock is the height of the most recent block which has been
	// scanned for spent tickets.
	lastScannedBlock int64
}

func New(network *config.Network, log slog.Logger, db *database.VspDatabase,
	mond rpc.MondConnect, wallets rpc.WalletConnect, blockNotifChan chan *wire.BlockHeader) *Vspm {

	v := &Vspm{
		network: network,
		log:     log,
		db:      db,
		mond:    mond,
		wallets: wallets,

		blockNotifChan: blockNotifChan,
	}

	return v
}

func (v *Vspm) Run(ctx context.Context) {
	// Run database integrity checks to ensure all data in database is present
	// and up-to-date.
	err := v.checkDatabaseIntegrity(ctx)
	if err != nil {
		// Don't log error if shutdown was requested, just return.
		if errors.Is(err, context.Canceled) {
			return
		}

		// vspm should still start if this fails, so just log an error.
		v.log.Errorf("Database integrity check failed: %v", err)
	}

	// Stop if shutdown requested.
	if ctx.Err() != nil {
		return
	}

	// Run the update function now to catch up with any blocks mined while vspm
	// was shut down.
	v.update(ctx)

	// Stop if shutdown requested.
	if ctx.Err() != nil {
		return
	}

	// Run voting wallet consistency check now to ensure all wallets are up to
	// date.
	v.checkWalletConsistency(ctx)

	// Stop if shutdown requested.
	if ctx.Err() != nil {
		return
	}

	// Start all background tasks and notification handlers.
	consistencyTicker := time.NewTicker(consistencyInterval)
	defer consistencyTicker.Stop()
	mondTicker := time.NewTicker(mondInterval)
	defer mondTicker.Stop()

	for {
		select {
		// Run voting wallet consistency check periodically.
		case <-consistencyTicker.C:
			v.checkWalletConsistency(ctx)

		// Ensure mond client is connected so notifications are received.
		case <-mondTicker.C:
			_, _, err := v.mond.Client()
			if err != nil {
				v.log.Error(err)
			}

		// Run the update function every time a block connected notification is
		// received from mond.
		case header := <-v.blockNotifChan:
			v.log.Debugf("Block notification %d (%s)", header.Height, header.BlockHash().String())
			v.update(ctx)

		// Handle shutdown request.
		case <-ctx.Done():
			return
		}
	}
}
