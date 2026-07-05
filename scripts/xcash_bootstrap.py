#!/usr/bin/env python3
"""Bootstrap xcash for local BetMonster testing.

Runs inside the xcash Django container. It creates/activates the local anvil
chain, deploys the USDT mock and VaultSlot contracts, funds the system hot
wallet, creates a test project, and prints the credentials needed by BetMonster.
"""
from __future__ import annotations

import os
import sys
import time
from pathlib import Path

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
from web3.exceptions import Web3Exception  # noqa: E402

from chains.constants import ChainCode  # noqa: E402
from chains.models import AddressUsage  # noqa: E402
from chains.models import Chain  # noqa: E402
from chains.models import ChainType  # noqa: E402
from core.default_data import _deploy_local_evm_erc20_contract  # noqa: E402
from core.default_data import ensure_chain_native_mapping  # noqa: E402
from core.default_data import ensure_crypto_on_chain_mapping  # noqa: E402
from core.models import SystemWallet  # noqa: E402
from evm.local_vault_slot import ensure_local_vault_slot_contracts  # noqa: E402
from projects.models import Project  # noqa: E402

ANVIL_RPC = os.environ.get("XCASH_ANVIL_RPC", "http://xcash_anvil:8545")
ANVIL_FUND_ETH = 10
BETMONSTER_WEBHOOK = os.environ.get(
    "BETMONSTER_WEBHOOK_URL",
    "http://host.docker.internal:8080/webhooks/xcash/deposit",
)
PROJECT_NAME = "BetMonster Local"


def wait_for_anvil(rpc: str, timeout: int = 60) -> Web3:
    deadline = time.monotonic() + timeout
    last_error = None
    while time.monotonic() < deadline:
        try:
            w3 = Web3(Web3.HTTPProvider(rpc, request_kwargs={"timeout": 5}))
            if w3.is_connected():
                w3.eth.block_number
                return w3
        except Exception as exc:  # noqa: BLE001
            last_error = exc
        time.sleep(1)
    raise RuntimeError(f"Could not connect to anvil at {rpc}: {last_error}")


def ensure_anvil_chain(w3: Web3) -> Chain:
    chain, _ = Chain.objects.update_or_create(
        code=ChainCode.Anvil,
        defaults={
            "rpc": ANVIL_RPC,
            "active": True,
        },
    )
    chain.save()
    return chain


def deploy_local_contracts(w3: Web3) -> tuple[str, str]:
    """Deploy USDT and USDC mocks and VaultSlot contracts on anvil."""
    ensure_chain_native_mapping(chain_name=ChainCode.Anvil, crypto_symbol="ETH")
    usdt_address = _deploy_local_evm_erc20_contract(w3=w3)
    ensure_crypto_on_chain_mapping(
        chain_name=ChainCode.Anvil,
        crypto_symbol="USDT",
        address=usdt_address,
        decimals=6,
    )
    usdc_address = _deploy_local_evm_erc20_contract(w3=w3)
    ensure_crypto_on_chain_mapping(
        chain_name=ChainCode.Anvil,
        crypto_symbol="USDC",
        address=usdc_address,
        decimals=6,
    )
    ensure_local_vault_slot_contracts(w3=w3)
    return usdt_address, usdc_address


def ensure_system_wallet_funded(w3: Web3) -> str:
    system_wallet = SystemWallet.get_current()
    address_obj = system_wallet.wallet.get_address(
        chain_type=ChainType.EVM,
        usage=AddressUsage.HotWallet,
    )
    address = address_obj.address
    funder = w3.eth.accounts[0]
    tx_hash = w3.eth.send_transaction(
        {
            "from": funder,
            "to": address,
            "value": w3.to_wei(ANVIL_FUND_ETH, "ether"),
        }
    )
    w3.eth.wait_for_transaction_receipt(tx_hash, timeout=30, poll_latency=0.5)
    return address


def ensure_betmonster_project() -> Project:
    collection_address = Web3.to_checksum_address(
        os.environ.get(
            "BETMONSTER_COLLECTION_ADDRESS",
            "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
        )
    )
    project, _ = Project.objects.update_or_create(
        name=PROJECT_NAME,
        defaults={
            "webhook": BETMONSTER_WEBHOOK,
            "webhook_open": True,
            "ip_white_list": "*",
            "is_test": True,
            "active": True,
        },
    )
    if not project.evm_vault:
        project.evm_vault = collection_address
        project.save(update_fields=["evm_vault"])
    return project


def main() -> int:
    try:
        w3 = wait_for_anvil(ANVIL_RPC)
        print(f"Connected to anvil at {ANVIL_RPC} (block {w3.eth.block_number})")

        chain = ensure_anvil_chain(w3)
        print(f"Anvil chain active: {chain.code} @ {chain.rpc}")

        usdt_address, usdc_address = deploy_local_contracts(w3)
        print(f"Local USDT mock deployed at {usdt_address}")
        print(f"Local USDC mock deployed at {usdc_address}")

        system_address = ensure_system_wallet_funded(w3)
        print(f"System wallet funded: {system_address}")

        project = ensure_betmonster_project()
        print(f"Project ready: {project.name} ({project.appid})")

        print()
        print("# Active local pairs (anvil only):")
        print("#   USDT:anvil, USDC:anvil, ETH:anvil")
        print()
        print("# Add these values to your BetMonster .env:")
        print(f"XCASH_BASE_URL=http://localhost:6688")
        print(f"XCASH_APPID={project.appid}")
        print(f"XCASH_HMAC_KEY={project.hmac_key}")
        print(f"XCASH_WEBHOOK_SECRET={project.hmac_key}")
        return 0
    except Web3Exception as exc:
        print(f"Web3 error: {exc}")
        return 1
    except Exception as exc:  # noqa: BLE001
        print(f"Bootstrap error: {exc}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
