#!/usr/bin/env python3
"""Send test USDT on the local anvil chain.

Runs inside the xcash Django container. It looks up the USDT mock contract
address from the xcash database, then sends the requested amount from anvil
account 0 to the destination address.

Usage (inside xcash_django container):
    python /tmp/send_anvil_usdt.py <destination_address> [amount]

Example:
    python /tmp/send_anvil_usdt.py 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 10
"""
from __future__ import annotations

import os
import sys
from pathlib import Path
from decimal import Decimal

# xcash interior package layout: apps live under /app/xcash, but manage.py adds
# that directory to sys.path. Replicate that here so absolute imports work.
APP_ROOT = Path("/app")
if str(APP_ROOT) not in sys.path:
    sys.path.append(str(APP_ROOT))
XCASH_PKG = APP_ROOT / "xcash"
if str(XCASH_PKG) not in sys.path:
    sys.path.append(str(XCASH_PKG))

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "config.settings.production")

import django  # noqa: E402

django.setup()

from web3 import Web3  # noqa: E402

from chains.constants import ChainCode  # noqa: E402
from chains.models import Chain  # noqa: E402
from currencies.models import CryptoOnChain  # noqa: E402

ANVIL_RPC = os.environ.get("XCASH_ANVIL_RPC", "http://xcash_anvil:8545")
USDT_DECIMALS = 6

ERC20_ABI = [
    {
        "constant": False,
        "inputs": [
            {"name": "_to", "type": "address"},
            {"name": "_value", "type": "uint256"},
        ],
        "name": "transfer",
        "outputs": [{"name": "", "type": "bool"}],
        "payable": False,
        "stateMutability": "nonpayable",
        "type": "function",
    },
    {
        "constant": False,
        "inputs": [
            {"name": "to", "type": "address"},
            {"name": "value", "type": "uint256"},
        ],
        "name": "mint",
        "outputs": [{"name": "", "type": "bool"}],
        "payable": False,
        "stateMutability": "nonpayable",
        "type": "function",
    },
    {
        "constant": True,
        "inputs": [{"name": "_owner", "type": "address"}],
        "name": "balanceOf",
        "outputs": [{"name": "balance", "type": "uint256"}],
        "payable": False,
        "stateMutability": "view",
        "type": "function",
    },
]


def get_usdt_address() -> str:
    chain = Chain.objects.get(code=ChainCode.Anvil)
    mapping = CryptoOnChain.objects.get(chain=chain, crypto__symbol="USDT")
    if not mapping.address:
        raise RuntimeError("No USDT contract address configured for anvil chain")
    return Web3.to_checksum_address(mapping.address)


def main() -> int:
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <destination_address> [amount]", file=sys.stderr)
        return 1

    destination = Web3.to_checksum_address(sys.argv[1])
    amount = Decimal(sys.argv[2]) if len(sys.argv) > 2 else Decimal("10")

    w3 = Web3(Web3.HTTPProvider(ANVIL_RPC))
    if not w3.is_connected():
        print(f"Could not connect to anvil at {ANVIL_RPC}", file=sys.stderr)
        return 1

    usdt_address = get_usdt_address()
    print(f"USDT mock contract: {usdt_address}")
    print(f"Sending {amount} USDT to {destination}")

    contract = w3.eth.contract(address=usdt_address, abi=ERC20_ABI)
    minter = w3.eth.accounts[0]
    value = int(amount * (10 ** USDT_DECIMALS))

    balance_before = contract.functions.balanceOf(destination).call()
    tx_hash = contract.functions.mint(destination, value).transact({"from": minter})
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30, poll_latency=0.5)
    balance_after = contract.functions.balanceOf(destination).call()

    if receipt.status != 1:
        print(f"Transaction failed: {tx_hash.hex()}", file=sys.stderr)
        return 1

    print(f"Transaction mined: {tx_hash.hex()}")
    print(f"Destination balance before: {balance_before / 10**USDT_DECIMALS}")
    print(f"Destination balance after:  {balance_after / 10**USDT_DECIMALS}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
