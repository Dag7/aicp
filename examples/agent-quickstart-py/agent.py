"""
AICP Agent Quickstart — Python

This example demonstrates the 5-call agent experience:
  1. Create identity
  2. Discover services
  3. Onboard to a service
  4. Make a payment
  5. Verify an identity

Run: python agent.py
"""

import asyncio
import os

from aicp import AICPAgent, generate_key_pair, principal_did

SERVER_URL = os.environ.get("AICP_SERVER", "http://localhost:8080")


async def main() -> None:
    # 1. Generate an identity and connect.
    print("Generating agent identity...")
    kp = generate_key_pair()
    did = principal_did(kp.public_key)
    print(f"Agent DID: {did}")

    async with await AICPAgent.create(SERVER_URL, kp.private_key) as agent:
        # 2. Check server health.
        healthy = await agent.health()
        print(f"Server healthy: {healthy}")

        if not healthy:
            print("Server is not reachable. Start the AICP server first.")
            return

        # 3. Discover the server's service info.
        try:
            service = await agent.discover(SERVER_URL.removeprefix("http://").removeprefix("https://"))
            print(f"Service: {service.service.name} ({service.service.did})")
            print(f"Categories: {', '.join(service.service.category)}")

            # 4. Verify our own DID.
            verify_result = await agent.verify(did)
            print(f"\nVerification result:")
            print(f"  Valid: {verify_result.valid}")
            print(f"  Trust Score: {verify_result.trust_score}")

            if verify_result.errors:
                print(f"  Errors: {', '.join(verify_result.errors)}")
        except Exception as e:
            print(f"API call failed: {e}")

    print("\nDone! Agent is ready to interact with AICP services.")


if __name__ == "__main__":
    asyncio.run(main())
