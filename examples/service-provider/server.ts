/**
 * AICP Service Provider Example
 *
 * Shows how to integrate an existing web service with AICP:
 *   - Serve .well-known/aicp.json
 *   - Authenticate requests via DID-Auth
 *   - Check trust scores before granting access
 *
 * Run: npx tsx server.ts
 */

import { createServer } from "node:http";
import {
  AICPServiceAdapter,
  AICPClient,
  type AICPRequestContext,
} from "@aicp/sdk";

const PORT = parseInt(process.env.PORT ?? "3000");
const AICP_SERVER = process.env.AICP_SERVER ?? "http://localhost:8080";

// 1. Configure the service adapter.
const adapter = new AICPServiceAdapter({
  serviceDID: "did:aicp:myservice123",
  serviceName: "Example AI Service",
  categories: ["ai", "nlp", "text-generation"],
  requiredCredentials: ["AICPAgentDelegation"],
  acceptedPayments: ["stripe"],
  minimumTrustScore: 100,
});

// 2. Generate the ANS record.
const ansRecord = adapter.buildANSRecord({
  description: "An example AI-powered text generation service",
  homepage: `http://localhost:${PORT}`,
  apiBase: `http://localhost:${PORT}/api`,
  pricingModel: "usage",
  pricingTiers: [
    { name: "free", rate: "0", description: "100 requests/day" },
    { name: "pro", rate: "0.01", description: "Per request" },
  ],
});

// 3. Create an AICP client for verification.
const aicpClient = new AICPClient({ serverURL: AICP_SERVER });

// 4. Build the HTTP server.
const server = createServer(async (req, res) => {
  const url = new URL(req.url ?? "/", `http://localhost:${PORT}`);

  // Serve .well-known/aicp.json
  if (url.pathname === "/.well-known/aicp.json") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify(ansRecord, null, 2));
    return;
  }

  // API endpoint: requires AICP authentication.
  if (url.pathname === "/api/generate") {
    const auth = adapter.extractAuth(req.headers.authorization);

    if (!auth.authenticated) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "AICP authentication required" }));
      return;
    }

    // Optionally verify the DID's trust score.
    try {
      const verification = await aicpClient.verify(auth.did!);
      if (!verification.valid) {
        res.writeHead(403, { "Content-Type": "application/json" });
        res.end(JSON.stringify({ error: "DID verification failed" }));
        return;
      }

      console.log(
        `Authenticated request from ${auth.did} (trust: ${verification.trust_score})`,
      );
    } catch {
      // If AICP server is unreachable, allow the request
      // (graceful degradation).
      console.log(`AICP server unreachable, allowing ${auth.did}`);
    }

    // Process the request.
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({
        result: "Hello from AICP-integrated service!",
        agent_did: auth.did,
      }),
    );
    return;
  }

  // Health check.
  if (url.pathname === "/health") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ status: "ok" }));
    return;
  }

  res.writeHead(404);
  res.end("Not found");
});

server.listen(PORT, () => {
  console.log(`Service provider running on http://localhost:${PORT}`);
  console.log(`ANS record: http://localhost:${PORT}/.well-known/aicp.json`);
  console.log(`API: POST http://localhost:${PORT}/api/generate`);
});
