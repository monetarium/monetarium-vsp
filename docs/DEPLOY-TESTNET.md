# Deploying monetarium-vsp on testnet

Runbook for the server operator. End state: a working VSP at
`https://vsp.testnet.monetarium.online` (or your own domain), ready to
accept tickets from the Skarb wallet.

Everything fits on one server (Linux, 2+ GB RAM, ~20 GB disk):

```
monetarium-node (full node, txindex)   <- RPC port 19509 (localhost)
monetarium-wallet x2 (voting wallets)  <- RPC ports 19510, 19520 (localhost)
vspd                                   <- port 8800 (localhost)
reverse proxy with TLS (nginx/caddy)   <- 443 public
```

All RPC ports listen on localhost only. Only 443 is exposed.

## 1. Node

```bash
monetarium-node --testnet --txindex \
  --rpcuser=<user> --rpcpass=<pass> \
  --rpclisten=127.0.0.1:19509
```

Wait for full sync. Check: `getblockcount` over RPC must match
https://testnet.monetarium.online.

## 2. Voting wallets (2 instances)

Keys are generated and kept by the project owner, NOT the operator:
each wallet gets its own seed (`monetarium-wallet --create`), and no
funds are ever sent to these wallets.

Each wallet is a separate process with its own data directory and port:

```bash
monetarium-wallet --testnet --enablevoting --manualtickets --nogrpc \
  --appdata=~/.mvsp-wallet-1 \
  --username=<user> --password=<pass> \
  --pass=<passphrase> \
  --rpcconnect=127.0.0.1:19509 \
  --cafile=<path to the node's rpc.cert> \
  --rpclisten=127.0.0.1:19510
# second instance: --appdata=~/.mvsp-wallet-2 --rpclisten=127.0.0.1:19520
```

Mandatory flags (verified against a live run):
- `--manualtickets` — vspd refuses to work with a wallet that lacks it
  (the wallet must vote ONLY tickets added by the VSP);
- `--pass=<passphrase>` — the first start needs the passphrase for
  account discovery, and without the flag the daemon silently blocks on
  a hidden stdin prompt; the same flag keeps the wallet unlocked for
  voting (store the passphrase in a private config file, mode 0600).

## 3. Fee key (xpub)

The owner generates a **dedicated** account in their own personal wallet
(e.g. an account named `vspfees`) and hands the operator only its
**extended public key (xpub)**. The private keys behind fee addresses
never touch the server.

## 4. vspd

```bash
go build ./cmd/vspd && sudo cp vspd /usr/local/bin/
vspd --network=testnet     # first run creates the home dir and asks for the xpub
```

`~/.vspd/vspd.conf` (minimum):

```ini
network = testnet
listen = 127.0.0.1:8800
vspfee = 1.0
dcrduser = <user>
dcrdpass = <pass>
; dcrdcert = path to the node's rpc.cert
walletuser = <user>
walletpass = <pass>
wallethost = 127.0.0.1:19510,127.0.0.1:19520
; walletcert = comma-separated paths to the wallets' rpc.cert files
adminpass = <admin page password>
designation = Monetarium Testnet VSP
supportemail = <email>
backupinterval = 3m
```

## 5. TLS and domain

Caddy (the shortest option):

```
vsp.testnet.monetarium.online {
    reverse_proxy 127.0.0.1:8800
}
```

## 6. systemd

One unit per process (`monetarium-node`, `mvsp-wallet-1`,
`mvsp-wallet-2`, `vspd`), `Restart=on-failure`, start order:
node -> wallets -> vspd. Example units are in
[docs/deployment.md](./deployment.md) (upstream; substitute the
monetarium-* binary names).

## 7. Verification

```bash
curl https://vsp.testnet.monetarium.online/api/v3/vspinfo
```

Expected: JSON with `"network": "testnet"`, `"vspfee": 1`, the public
key and pool statistics. Admin page: `https://<domain>/admin` (password
from the config). Database backups: downloadable from the admin page;
also back up the whole `~/.vspd` directory.

## 8. Hand back to the owner after launch

- The VSP URL and the /api/v3/vspinfo output (its pubkey goes into the
  Skarb wallet).
- Confirmation that both voting wallets are unlocked and syncing (vspd
  logs this at startup).

## A note on SSFee

Voting wallets on Monetarium receive stake-fee payouts (SSFee, including
SKA tokens) for every vote they cast. This differs from Decred: balances
on the voting wallets will grow over time. The withdrawal policy for
those funds is the owner's call; the operator only needs to know this is
expected behavior, not a leak.
