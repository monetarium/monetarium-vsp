# v3tool

v3tool is a simple client for manual testing of vspm.
It is a developer tool, not suitable for end users or production use.

## Prerequisites

1. An instance of monetarium-wallet which owns at least one mempool, immature or live ticket.
1. An instance of vspm to test.

## What v3tool does

1. Retrieve the pubkey from vspm.
1. Retrieve the list of owned mempool/immature/live tickets from monetarium-wallet.
1. For each ticket:
    1. Use monetarium-wallet to find the tx hex, voting privkey and commitment address of the ticket.
    1. Get a fee address and amount from vspm to register this ticket.
    1. Create the fee tx and send it to vspm.
    1. Get the ticket status.
    1. Change vote choices on the ticket.
    1. Get the ticket status again.
