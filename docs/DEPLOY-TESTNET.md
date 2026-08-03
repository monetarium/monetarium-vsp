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

Config file is `~/.monetarium/monetarium.conf` (note: the home dir is
`.monetarium`, the file is `monetarium.conf` — not `monetarium-node.conf`;
a config in the wrong place is silently ignored and the node starts on
**mainnet**):

```ini
testnet=1
txindex=1
rpcuser=<user>
rpcpass=<pass>
rpclisten=127.0.0.1:19509
; chaincfg ships with empty DNS seeders, so peers must be given manually
addpeer=176.9.28.21:19508
addpeer=134.249.62.108:19508
```

Without the `addpeer` lines the node sits at height 0 forever with no
error — there are no testnet seeders to discover peers from.

Wait for full sync. Check: `getblockcount` over RPC must match
https://testnet.monetarium.online (`/api/status` returns `node_height`).

## 2. Voting wallets (2 instances)

Keys are generated and kept by the project owner, NOT the operator:
each wallet gets its own seed (`monetarium-wallet --create`), and no
funds are ever sent to these wallets.

Create each wallet non-interactively (answers: passphrase, passphrase,
no extra public encryption, no existing seed, OK, no xpub import) —
**write the printed seed down, it is shown only once**:

```bash
printf '%s\n%s\nno\nno\nOK\nno\n' "$PASS" "$PASS" |
  monetarium-wallet --testnet --appdata=~/.mvsp-wallet-1 --create
```

Each wallet is a separate process with its own data directory and port.
Config file `~/.mvsp-wallet-N/monetarium-wallet.conf`:

```ini
testnet=1
appdata=/home/<user>/.mvsp-wallet-1
enablevoting=1
manualtickets=1
nogrpc=1
username=<wallet rpc user>
password=<wallet rpc pass>
pass=<passphrase>
rpcconnect=127.0.0.1:19509
mondusername=<node rpc user>
mondpassword=<node rpc pass>
cafile=/home/<user>/.monetarium/rpc.cert
rpclisten=127.0.0.1:19510
; second instance: appdata=...-2, rpclisten=127.0.0.1:19520
```

Note the fork renamed the node-RPC credential options: `mondusername` /
`mondpassword` (not `dcrduser`/`monduser` — an unknown option aborts
startup). `username`/`password` are the wallet's own JSON-RPC creds and
double as the defaults for the node connection.

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

The database must be created first, with the fee xpub — `vspd` itself
never prompts for it:

```bash
go build -o /usr/local/bin/vspd ./cmd/vspd
go build -o /usr/local/bin/vspadmin ./cmd/vspadmin
vspadmin createdatabase <fee xpub> --network=testnet
```

`~/.vspd/vspd.conf` (minimum):

```ini
network = testnet
listen = 127.0.0.1:8800
vspfee = 1.0
dcrduser = <node rpc user>
dcrdpass = <node rpc pass>
dcrdcert = /home/<user>/.monetarium/rpc.cert
walletuser = <user>,<user>
walletpass = <pass>,<pass>
wallethost = 127.0.0.1:19510,127.0.0.1:19520
walletcert = /home/<user>/.mvsp-wallet-1/rpc.cert,/home/<user>/.mvsp-wallet-2/rpc.cert
adminpass = <admin page password>
designation = Monetarium Testnet VSP
supportemail = <email>
backupinterval = 3m
```

`walletuser`/`walletpass` are **comma-separated lists that must have one
entry per host** — a single value with two hosts is a startup error, even
when the credentials are identical.

vspd loads its HTML templates and static assets through relative paths
(`internal/webapi/{templates,public}`), so its working directory must
contain them or it panics on startup. Copy them out of the source tree
to a stable location and point the unit at it:

```bash
sudo mkdir -p /opt/vspd/internal/webapi
sudo cp -r internal/webapi/templates internal/webapi/public /opt/vspd/internal/webapi/
# unit: WorkingDirectory=/opt/vspd
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
`mvsp-wallet-2`, `vspd`), start order node -> wallets -> vspd
(`After=`/`Requires=`). Example units are in
[docs/deployment.md](./deployment.md) (upstream; substitute the
monetarium-* binary names).

Use `Restart=always` with `StartLimitIntervalSec=0`, not
`Restart=on-failure`: a wallet that exits cleanly on an RPC hiccup would
otherwise stay down, and the default start-limit gives up after five
restarts — exactly the scenario where an unattended VSP must keep
retrying. `RestartSec=15` is enough for the node to be back before the
wallets retry.

vspd tolerates a wallet being down (it logs `proceeding with only N` and
reconnects on the next block), so restart storms are not dangerous — but
`votingwalletsonline` in vspinfo only recovers on the next connection
attempt, which is what to check after any restart.

A 5-minute health timer (curl `/api/v3/vspinfo`, restart on two
consecutive failures) plus a daily backup timer covers unattended
operation.

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
