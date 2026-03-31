/**
 * AICP Agent Quickstart — TypeScript
 *
 * This example demonstrates the 5-call agent experience:
 *   1. Create identity
 *   2. Discover services
 *   3. Onboard to a service
 *   4. Make a payment
 *   5. Verify an identity
 *
 * Run: npx tsx agent.ts
 */

import {
  AICPAgent,
  generateKeyPair,
  principalDID,
} from "@aicp/sdk";

const SERVER_URL = process.env.AICP_SERVER ?? "http://localhost:8080";

async function main() {
  // 1. Generate an identity and connect.
  console.log("Generating agent identity...");
  const keyPair = generateKeyPair();
  const did = principalDID(keyPair.publicKey);
  console.log(`Agent DID: ${did}`);

  const agent = await AICPAgent.create({
    serverURL: SERVER_URL,
    privateKey: keyPair.privateKey,
  });

  // 2. Check server health.
  const healthy = await agent.health();
  console.log(`Server healthy: ${healthy}`);

  if (!healthy) {
    console.log("Server is not reachable. Start the AICP server first.");
    return;
  }

  // 3. Discover the server's service info.
  try {
    const service = await agent.discover(SERVER_URL.replace(/^https?:\/\//, ""));
    console.log(`Service: ${service.service.name} (${service.service.did})`);
    console.log(`Categories: ${service.service.category.join(", ")}`);

    // 4. Verify our own DID.
    const verifyResult = await agent.verify(did);
    console.log(`\nVerification result:`);
    console.log(`  Valid: ${verifyResult.valid}`);
    console.log(`  Trust Score: ${verifyResult.trust_score}`);

    if (verifyResult.errors?.length) {
      console.log(`  Errors: ${verifyResult.errors.join(", ")}`);
    }
  } catch (err) {
    console.error(`API call failed: ${err}`);
  }

  console.log("\nDone! Agent is ready to interact with AICP services.");
}

main().catch(console.error);
