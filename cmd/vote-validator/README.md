# vote-validator

vote-validator is a tool for VSP admins to verify that their vspm deployment
is voting correctly according to user preferences.

## What it does

1. Retrieve all voted tickets from the provided vspm database file.
1. Retrieve vote info from mondata for every voted ticket.
1. For the n most recently voted tickets, compare the vote choices recorded
   on-chain to the vote choices set by the user.
1. Write details of any discrepancies to a file for further investigation.

## How to run it

Only run vote-validator using a copy of the vspm database backup file.
Never use a real production database.

vote-validator can be run from the repository root as such:

```no-highlight
go run ./cmd/vote-validator -n 1000 -f ./vspm.db-backup
```
