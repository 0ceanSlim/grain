// NIP-86 submit helper. The only Nostr-protocol code in the grain
// admin frontend; designed to be lifted into the grain client library
// verbatim, so it has no DOM dependencies and reads nothing off the
// page beyond window.grainSigner (the existing signer bridge wired
// by mill-bridge.js).
//
// Wire shape, per NIP-86 + NIP-98:
//   - POST to the relay base URL ("/") with body {method, params}.
//   - Authorization: "Nostr " + base64(JSON(signed kind-27235)).
//   - Signed event tags: u=<absolute URL>, method=POST, payload=<sha256(body)>.
//   - Content-Type: application/nostr+json+rpc.
//
// Stringifying the body exactly once is load-bearing: the bytes
// hashed for the `payload` tag must be byte-identical to the bytes
// sent on the wire, or the relay's NIP-98 check will reject.

(function () {
  "use strict";

  async function sha256Hex(str) {
    const bytes = new TextEncoder().encode(str);
    const digest = await crypto.subtle.digest("SHA-256", bytes);
    return Array.from(new Uint8Array(digest))
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
  }

  async function submit(method, params) {
    if (!window.grainSigner || typeof window.grainSigner.signEvent !== "function") {
      throw new Error("grainSigner unavailable — sign in to continue");
    }
    const body = JSON.stringify({ method: method, params: params || [] });
    const url = window.location.origin + "/";
    const payload = await sha256Hex(body);

    const tmpl = {
      kind: 27235,
      created_at: Math.floor(Date.now() / 1000),
      content: "",
      tags: [
        ["u", url],
        ["method", "POST"],
        ["payload", payload],
      ],
    };
    const signed = await window.grainSigner.signEvent(tmpl);
    const auth = "Nostr " + btoa(JSON.stringify(signed));

    const resp = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/nostr+json+rpc",
        Authorization: auth,
      },
      body: body,
    });

    let json = null;
    try {
      json = await resp.json();
    } catch (_) {
      // Non-JSON response body — fall through to status-based error.
    }
    if (!resp.ok) {
      const msg = (json && (json.error || json.message)) || resp.statusText;
      throw new Error("NIP-86 " + method + " failed: " + msg);
    }
    if (json && json.error) {
      throw new Error("NIP-86 " + method + " error: " + json.error);
    }
    return json && Object.prototype.hasOwnProperty.call(json, "result")
      ? json.result
      : json;
  }

  window.grainNIP86 = { submit: submit };
})();
