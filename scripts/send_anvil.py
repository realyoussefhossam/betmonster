#!/usr/bin/env python3
"""Send a test currency on the local anvil chain.

Runs inside the xcash Django container. For ERC20 tokens it looks up the
contract address and decimals from the xcash database, then mints the requested
amount from anvil account 0. For native tokens (e.g. ETH) it transfers value
directly from anvil account 0.

Usage (inside xcash_django container):
    python /tmp/send_anvil.py [currency] <destination_address> [amount]

Arguments:
    currency          Token symbol, e.g. ETH, USDT or USDC (default: USDT)
    destination_address  EVM address to receive the funds
    amount            Amount to send (default: 10)

Examples:
    python /tmp/send_anvil.py ETH 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 1
    python /tmp/send_anvil.py USDT 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 20
    python /tmp/send_anvil.py 0x7e77B4AD9AA1e006da07fc1A906e8f8195606e16 5
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


def get_token_config(currency: str) -> tuple[str | None, int]:
    """Return (contract_address, decimals) for the given currency on anvil.

    A None contract address means the currency is native (e.g. ETH).
    """
    chain = Chain.objects.get(code=ChainCode.Anvil)
    mapping = CryptoOnChain.objects.get(chain=chain, crypto__symbol=currency.upper())
    address = mapping.address.strip() if mapping.address else None
    if address:
        address = Web3.to_checksum_address(address)
    return address, mapping.decimals


def parse_args(argv: list[str]) -> tuple[str, str, Decimal]:
    """Parse arguments: [currency] <address> [amount]."""
    if len(argv) < 2:
        raise ValueError("missing destination address")

    # If the first arg is a valid EVM address, it is the destination and currency defaults to USDT.
    if argv[0].startswith("0x") and len(argv[0]) == 42:
        currency = "USDT"
        destination = argv[0]
        amount_idx = 1
    else:
        if len(argv) < 3:
            raise ValueError("missing destination address")
        currency = argv[0]
        destination = argv[1]
        amount_idx = 2

    amount = Decimal(argv[amount_idx]) if len(argv) > amount_idx else Decimal("10")
    return currency, destination, amount


def send_native(w3: Web3, destination: str, amount: Decimal, decimals: int) -> tuple[str, int, int]:
    """Send a native currency. Returns (tx_hash_hex, balance_before, balance_after)."""
    sender = w3.eth.accounts[0]
    value = int(amount * (10 ** decimals))

    balance_before = w3.eth.get_balance(destination)
    tx_hash = w3.eth.send_transaction({"to": destination, "value": value, "from": sender})
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30, poll_latency=0.5)
    balance_after = w3.eth.get_balance(destination)

    if receipt.status != 1:
        raise RuntimeError(f"Transaction failed: {tx_hash.hex()}")
    return tx_hash.hex(), balance_before, balance_after


def send_erc20(w3: Web3, contract_address: str, destination: str, amount: Decimal, decimals: int) -> tuple[str, int, int]:
    """Mint an ERC20 token. Returns (tx_hash_hex, balance_before, balance_after)."""
    if w3.eth.get_code(contract_address) == b"":
        raise RuntimeError(
            f"No contract deployed at {contract_address} for token on anvil. "
            "The mock contract may need to be redeployed."
        )

    contract = w3.eth.contract(address=contract_address, abi=ERC20_ABI)
    minter = w3.eth.accounts[0]
    value = int(amount * (10 ** decimals))

    balance_before = contract.functions.balanceOf(destination).call()
    tx_hash = contract.functions.mint(destination, value).transact({"from": minter})
    receipt = w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30, poll_latency=0.5)
    balance_after = contract.functions.balanceOf(destination).call()

    if receipt.status != 1:
        raise RuntimeError(f"Transaction failed: {tx_hash.hex()}")
    return tx_hash.hex(), balance_before, balance_after


def main() -> int:
    try:
        currency, destination, amount = parse_args(sys.argv[1:])
    except ValueError as e:
        print(f"Usage: {sys.argv[0]} [currency] <destination_address> [amount]", file=sys.stderr)
        print(f"Error: {e}", file=sys.stderr)
        return 1

    destination = Web3.to_checksum_address(destination)

    w3 = Web3(Web3.HTTPProvider(ANVIL_RPC))
    if not w3.is_connected():
        print(f"Could not connect to anvil at {ANVIL_RPC}", file=sys.stderr)
        return 1

    try:
        contract_address, decimals = get_token_config(currency)
    except Exception as e:
        print(f"Could not load token config for {currency}: {e}", file=sys.stderr)
        return 1

    if contract_address is None:
        print(f"Sending {amount} {currency} (native) to {destination}")
    else:
        print(f"{currency} mock contract: {contract_address}")
        print(f"Sending {amount} {currency} to {destination}")

    try:
        if contract_address is None:
            tx_hash, balance_before, balance_after = send_native(w3, destination, amount, decimals)
        else:
            tx_hash, balance_before, balance_after = send_erc20(w3, contract_address, destination, amount, decimals)
    except Exception as e:
        print(f"Failed to send {currency}: {e}", file=sys.stderr)
        return 1

    print(f"Transaction mined: {tx_hash}")
    print(f"Destination balance before: {balance_before / 10**decimals}")
    print(f"Destination balance after:  {balance_after / 10**decimals}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
